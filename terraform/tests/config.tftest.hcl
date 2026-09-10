# Runs without Azure. The provider is mocked, so what this checks is the
# configuration's own logic — the variable rules, and the environment it hands
# the app compared with what kcl/configmap.k renders — not what Azure accepts.

# The provider still validates resource ids it is handed, so every id another
# resource consumes needs a well-formed one rather than the mock's random string.
mock_provider "azurerm" {
  override_resource {
    target = azurerm_resource_group.this
    values = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/schmetterpause-rg"
    }
  }

  override_resource {
    target = azurerm_log_analytics_workspace.this
    values = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/schmetterpause-rg/providers/Microsoft.OperationalInsights/workspaces/schmetterpause-logs"
    }
  }

  override_resource {
    target = azurerm_container_app_environment.this
    values = {
      id             = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/schmetterpause-rg/providers/Microsoft.App/managedEnvironments/schmetterpause-env"
      default_domain = "calm-sea-1234.westeurope.azurecontainerapps.io"
    }
  }

  override_resource {
    target = azurerm_postgresql_flexible_server.this
    values = {
      id   = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/schmetterpause-rg/providers/Microsoft.DBforPostgreSQL/flexibleServers/schmetterpause-pg-12345678"
      fqdn = "schmetterpause-pg-12345678.postgres.database.azure.com"
    }
  }
}

variables {
  subscription_id   = "00000000-0000-0000-0000-000000000000"
  session_key       = "0123456789abcdef0123456789abcdef"
  postgres_password = "Abcdefgh12345678"
}

run "defaults_mirror_kcl" {
  command = apply

  assert {
    condition     = azurerm_container_app.this.template[0].max_replicas == 1
    error_message = "There must never be more than one replica: goose takes no session lock."
  }

  assert {
    condition     = { for e in azurerm_container_app.this.template[0].container[0].env : e.name => e.value }["SP_AUTO_MIGRATE"] == "false"
    error_message = "The server must not migrate; the init container does."
  }

  assert {
    condition     = [for e in azurerm_container_app.this.template[0].init_container[0].env : e.name] == ["SP_DATABASE_URL"]
    error_message = "The init container gets the database URL and nothing else."
  }

  assert {
    condition     = output.public_base_url == "https://schmetterpause-app.calm-sea-1234.westeurope.azurecontainerapps.io"
    error_message = "SP_PUBLIC_BASE_URL must default to the app's generated address."
  }

  assert {
    condition = length(setintersection(
      [for e in azurerm_container_app.this.template[0].container[0].env : e.name],
      ["SP_COOKIE_SECURE", "SP_KIOSK_TOKEN", "SP_BOOTSTRAP_ADMIN"],
    )) == 0
    error_message = "Unset options must stay absent, and SP_COOKIE_SECURE always."
  }

  assert {
    condition = { for e in azurerm_container_app.this.template[0].container[0].env : e.name => e.secret_name if e.secret_name != null } == {
      SP_DATABASE_URL = "database-url"
      SP_SESSION_KEY  = "session-key"
    }
    error_message = "The database URL and the session key must come from secrets."
  }
}

run "options_are_wired" {
  command = apply

  variables {
    kiosk_token     = "0123456789abcdef"
    bootstrap_admin = "Kim"
    public_base_url = "https://pause.example.com/"
    extra_env_vars  = { SP_LOG_LEVEL = "debug" }
  }

  assert {
    condition     = { for e in azurerm_container_app.this.template[0].container[0].env : e.name => e.secret_name if e.secret_name != null }["SP_KIOSK_TOKEN"] == "kiosk-token"
    error_message = "A kiosk token must reach the app as a secret."
  }

  assert {
    condition     = { for e in azurerm_container_app.this.template[0].container[0].env : e.name => e.value }["SP_BOOTSTRAP_ADMIN"] == "Kim"
    error_message = "bootstrap_admin must reach SP_BOOTSTRAP_ADMIN."
  }

  assert {
    condition     = output.public_base_url == "https://pause.example.com"
    error_message = "public_base_url must win over the generated address, without a trailing slash."
  }

  assert {
    condition     = { for e in azurerm_container_app.this.template[0].container[0].env : e.name => e.value }["SP_LOG_LEVEL"] == "debug"
    error_message = "extra_env_vars must override, as extraEnvVars does in kcl."
  }
}

run "hex_password_is_refused" {
  command = plan

  variables {
    postgres_password = "0123456789abcdef0123456789abcdef"
  }

  expect_failures = [var.postgres_password]
}

run "url_breaking_password_is_refused" {
  command = plan

  variables {
    postgres_password = "Abcdefgh1234@5678"
  }

  expect_failures = [var.postgres_password]
}

run "short_session_key_is_refused" {
  command = plan

  variables {
    session_key = "too-short"
  }

  expect_failures = [var.session_key]
}

run "secret_through_extra_env_is_refused" {
  command = plan

  variables {
    extra_env_vars = { SP_SESSION_KEY = "0123456789abcdef0123456789abcdef" }
  }

  expect_failures = [var.extra_env_vars]
}

run "second_replica_is_refused" {
  command = plan

  variables {
    min_replicas = 2
  }

  expect_failures = [var.min_replicas]
}

run "invalid_resource_pair_is_refused" {
  command = plan

  variables {
    cpu    = 0.25
    memory = "1Gi"
  }

  expect_failures = [var.memory]
}

run "latest_image_is_refused" {
  command = plan

  variables {
    image = "ghcr.io/stuttgart-things/schmetterpause:latest"
  }

  expect_failures = [var.image]
}
