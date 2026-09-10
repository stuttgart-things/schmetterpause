# Schmetterpause on Azure Container Apps.
#
# The same image as on Compose and Kubernetes (invariant 1), configured the same
# way — environment variables and nothing else (invariant 2). This file is the
# Azure rendering of what kcl/ renders for a cluster, and it follows the same
# conventions: one replica, migrations in an init container, the session key
# kept away from that init container, SP_COOKIE_SECURE left at its default.
#
#   - Postgres: Azure Database for PostgreSQL Flexible Server, the managed
#     option docs/adr/0001 names.
#   - The app: external HTTPS, one replica.
#
# No Redis (invariant 3, docs/adr/0002).

locals {
  app_name = "${var.name_prefix}-app"

  # The generated address is <app name>.<environment default domain>, known
  # from the environment alone, so the app can be told its own URL without
  # depending on itself.
  generated_url = "https://${local.app_name}.${azurerm_container_app_environment.this.default_domain}"

  # Flexible Server enforces TLS, hence sslmode=require — the same value
  # kcl/schema.k defaults dbSSLMode to.
  database_url = "postgresql://${var.postgres_user}:${var.postgres_password}@${azurerm_postgresql_flexible_server.this.fqdn}:5432/${var.postgres_db}?sslmode=require"

  # kcl/configmap.k, key for key. extra_env_vars comes last so it overrides, as
  # extraEnvVars does there.
  #
  # SP_COOKIE_SECURE is deliberately absent. It defaults to true in the code so
  # a forgotten setting fails closed; setting it here could only weaken it.
  app_env = merge(
    {
      SP_HTTP_ADDR = ":8080"
      SP_LOG_LEVEL = var.log_level
      # False, always. Migrations run in the init container, so the step that
      # changes the schema is not something the web server does on its way up.
      SP_AUTO_MIGRATE = "false"
      # Set rather than left to the request: TLS terminates in front of the
      # app, and the QR sheet must point at the address a phone can reach.
      SP_PUBLIC_BASE_URL = var.public_base_url != "" ? trimsuffix(var.public_base_url, "/") : local.generated_url
    },
    { for k, v in { SP_BOOTSTRAP_ADMIN = var.bootstrap_admin } : k => v if v != "" },
    var.extra_env_vars,
  )
}

# A subscription policy may stamp governance tags onto resources. Every resource
# ignores tags, so a plan does not fight that policy on every run.

resource "azurerm_resource_group" "this" {
  name     = "${var.name_prefix}-rg"
  location = var.location

  lifecycle {
    ignore_changes = [tags]
  }
}

resource "azurerm_log_analytics_workspace" "this" {
  name                = "${var.name_prefix}-logs"
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  sku                 = "PerGB2018"
  retention_in_days   = 30

  lifecycle {
    ignore_changes = [tags]
  }
}

resource "azurerm_container_app_environment" "this" {
  name                       = "${var.name_prefix}-env"
  location                   = azurerm_resource_group.this.location
  resource_group_name        = azurerm_resource_group.this.name
  log_analytics_workspace_id = azurerm_log_analytics_workspace.this.id

  lifecycle {
    ignore_changes = [tags]
  }
}

# ── Postgres ──────────────────────────────────────────────────────────────────

# The server name becomes <name>.postgres.database.azure.com and must be
# globally unique. A short hash of the resource group id keeps it unique per
# subscription without the random provider.
#
# Changing name_prefix replaces this server, and the data goes with it.
resource "azurerm_postgresql_flexible_server" "this" {
  name                          = "${var.name_prefix}-pg-${substr(sha1(azurerm_resource_group.this.id), 0, 8)}"
  resource_group_name           = azurerm_resource_group.this.name
  location                      = azurerm_resource_group.this.location
  version                       = var.postgres_version
  administrator_login           = var.postgres_user
  administrator_password        = var.postgres_password
  sku_name                      = var.postgres_sku
  storage_mb                    = 32768
  auto_grow_enabled             = true
  public_network_access_enabled = true
  backup_retention_days         = 7
  geo_redundant_backup_enabled  = false

  lifecycle {
    # Some regions reject an explicit zone; Azure places it, and a plan must
    # not churn over where.
    ignore_changes = [zone, tags]
  }
}

resource "azurerm_postgresql_flexible_server_database" "this" {
  name      = var.postgres_db
  server_id = azurerm_postgresql_flexible_server.this.id
  collation = "en_US.utf8"
  charset   = "utf8"
}

# "Allow access from Azure services" — the special 0.0.0.0 rule. Container Apps
# egress comes from Azure public addresses, so this is what lets the app in. It
# lets every other Azure tenant reach the port as well, with only the password
# in the way: acceptable for a test, not for keeping. See README.md.
resource "azurerm_postgresql_flexible_server_firewall_rule" "azure" {
  name             = "allow-azure-services"
  server_id        = azurerm_postgresql_flexible_server.this.id
  start_ip_address = "0.0.0.0"
  end_ip_address   = "0.0.0.0"
}

# ── Application ───────────────────────────────────────────────────────────────

resource "azurerm_container_app" "this" {
  name                         = local.app_name
  container_app_environment_id = azurerm_container_app_environment.this.id
  resource_group_name          = azurerm_resource_group.this.name
  revision_mode                = "Single"

  # The two Secrets of kcl/externalsecret.k as Container App secrets. The split
  # survives where it matters: the init container is given the database URL
  # and nothing else. The DSN carries the password, so all of it is secret.
  secret {
    name  = "database-url"
    value = local.database_url
  }

  secret {
    name  = "session-key"
    value = var.session_key
  }

  # Only whether a token is set is revealed, never the token.
  dynamic "secret" {
    for_each = nonsensitive(var.kiosk_token != "") ? ["kiosk-token"] : []
    content {
      name  = secret.value
      value = var.kiosk_token
    }
  }

  ingress {
    external_enabled = true
    target_port      = 8080
    transport        = "auto"

    # allow_insecure_connections stays false, so plain HTTP is redirected to
    # HTTPS before it reaches the app — the job of the redirect HTTPRoute on the
    # cluster. Over HTTP the Secure session cookie would never come back, and
    # every click would arrive as a stranger.

    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }

  template {
    # Exactly one, as kcl/schema.k insists: goose takes no session lock, so two
    # replicas starting together would migrate concurrently.
    #
    # One thing kcl does that this cannot: its Deployment uses Recreate, so the
    # old pod is gone before the new one migrates. A single-revision app has no
    # such strategy — the new revision starts, migrates and takes traffic while
    # the old one still serves. That holds only because migrations are forward
    # and additive (invariant 8): the old code still finds everything it knew.
    # See "Where it cannot be the same" in README.md.
    max_replicas = 1
    min_replicas = var.min_replicas

    # "migrate up" before the server starts, as the initContainer in
    # kcl/deploy.k. It gets SP_DATABASE_URL only: migrate never calls
    # ValidateForServe, so it has no use for the session key.
    #
    # 0.25 vCPU and 0.5Gi each, so the app alone and both together are valid
    # Consumption allocations either way Azure counts init containers.
    init_container {
      name   = "migrate"
      image  = var.image
      args   = ["migrate", "up"]
      cpu    = 0.25
      memory = "0.5Gi"

      env {
        name        = "SP_DATABASE_URL"
        secret_name = "database-url"
      }
    }

    container {
      name   = "schmetterpause"
      image  = var.image
      args   = ["serve"]
      cpu    = 0.25
      memory = "0.5Gi"

      dynamic "env" {
        for_each = local.app_env
        content {
          name  = env.key
          value = env.value
        }
      }

      env {
        name        = "SP_DATABASE_URL"
        secret_name = "database-url"
      }

      env {
        name        = "SP_SESSION_KEY"
        secret_name = "session-key"
      }

      dynamic "env" {
        for_each = nonsensitive(var.kiosk_token != "") ? ["SP_KIOSK_TOKEN"] : []
        content {
          name        = env.value
          secret_name = "kiosk-token"
        }
      }

      # /healthz ignores the database and /readyz checks it, so a database
      # outage takes the replica out of rotation instead of restarting it in a
      # loop that cannot help. The timeout matches kcl/deploy.k's 3 seconds and
      # stays above SP_READINESS_TIMEOUT's 2.
      liveness_probe {
        transport        = "HTTP"
        port             = 8080
        path             = "/healthz"
        initial_delay    = 5
        interval_seconds = 10
        timeout          = 3
      }

      readiness_probe {
        transport        = "HTTP"
        port             = 8080
        path             = "/readyz"
        initial_delay    = 5
        interval_seconds = 10
        timeout          = 3
      }
    }
  }

  lifecycle {
    ignore_changes = [tags]
  }

  # The init container migrates on first start, so the database and the rule
  # that lets the app reach it have to exist before the first revision does.
  depends_on = [
    azurerm_postgresql_flexible_server_database.this,
    azurerm_postgresql_flexible_server_firewall_rule.azure,
  ]
}
