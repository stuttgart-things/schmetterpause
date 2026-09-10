# Declarations only. Every value lives in schmetterpause.auto.tfvars — or, for
# the subscription and the secrets, in the gitignored terraform.tfvars — and
# not in the code. The one default left is kiosk_token's empty string: a secret
# cannot sit in a committed file, and leaving it out has to mean "no kiosk".
#
# Every application setting names the kcl/schema.k field it mirrors. The two
# describe the same contract, and a change to one lands in the other in the
# same pull request — see README.md.

# ── Azure ─────────────────────────────────────────────────────────────────────

variable "subscription_id" {
  description = "Azure subscription to deploy into. In terraform.tfvars."
  type        = string
}

variable "location" {
  description = "Azure region. It has to be allowed by the subscription's policy and offer PostgreSQL Flexible Server in postgres_version."
  type        = string
}

variable "name_prefix" {
  description = "Prefix for every resource name: lowercase letters, digits and hyphens. Changing it replaces every resource, the database included."
  type        = string

  # The container app is named "<prefix>-app", and Azure caps that at 32
  # characters.
  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{0,26}[a-z0-9]$", var.name_prefix))
    error_message = "name_prefix must be 2 to 28 lowercase letters, digits or hyphens, start with a letter and not end with a hyphen."
  }
}

# ── Application ───────────────────────────────────────────────────────────────

# kcl: image
#
# Pinned, never :latest. An apply should say which build it rolls out, and a
# moving tag makes two applies of the same configuration run different code.
variable "image" {
  description = "Container image, pinned to a tag. Renovate moves the value in schmetterpause.auto.tfvars."
  type        = string

  validation {
    condition     = !endswith(var.image, ":latest") && can(regex(":[^/]+$", var.image))
    error_message = "image must carry an explicit tag other than latest."
  }
}

# No kcl equivalent: a Deployment keeps its pod running regardless.
variable "min_replicas" {
  description = "1 keeps a replica warm; 0 scales to zero after a quiet period and cold-starts, migrations included, on the next request."
  type        = number

  validation {
    condition     = contains([0, 1], var.min_replicas)
    error_message = "min_replicas must be 0 or 1 — there is never more than one replica, see main.tf."
  }
}

# kcl: logLevel
variable "log_level" {
  description = "SP_LOG_LEVEL."
  type        = string

  validation {
    condition     = contains(["debug", "info", "warn", "error"], var.log_level)
    error_message = "log_level must be one of debug, info, warn, error."
  }
}

# kcl: publicBaseURL
variable "public_base_url" {
  description = "SP_PUBLIC_BASE_URL, the scheme and host the QR sheet points at. Empty derives it from the app's generated *.azurecontainerapps.io address; set it once a custom domain is bound."
  type        = string

  validation {
    condition     = var.public_base_url == "" || can(regex("^https?://[^/?#]+/?$", var.public_base_url))
    error_message = "public_base_url must be empty, or scheme and host only, for example https://schmetterpause.example.com."
  }
}

# kcl: bootstrapAdmin
variable "bootstrap_admin" {
  description = "SP_BOOTSTRAP_ADMIN: display name of the player who gets the admin flag at startup (docs/adr/0008). Empty grants nothing. That player has to have joined already, so a name belongs to a second apply, not the first."
  type        = string
}

# kcl: extraEnvVars
variable "extra_env_vars" {
  description = "Plain environment variables for the app container, for settings this configuration has no variable for yet. Applied last, so they override. Not for secrets."
  type        = map(string)

  validation {
    condition = alltrue([
      for k in keys(var.extra_env_vars) :
      !contains(["SP_DATABASE_URL", "SP_SESSION_KEY", "SP_KIOSK_TOKEN"], k)
    ])
    error_message = "extra_env_vars must not set SP_DATABASE_URL, SP_SESSION_KEY or SP_KIOSK_TOKEN — those are secrets and have variables of their own."
  }
}

# kcl: cpuRequest/cpuLimit
#
# Applied to the app and to its migrate init container each, as kcl gives both
# the same resources. Capped at 2 so the two together stay within the 4 vCPU a
# Consumption app may have.
variable "cpu" {
  description = "vCPU per container. Consumption plan steps only."
  type        = number

  validation {
    condition     = contains([0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2], var.cpu)
    error_message = "cpu must be one of 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2."
  }
}

# kcl: memoryRequest/memoryLimit
#
# Container Apps accepts only fixed pairs, memory in Gi twice the vCPU. Checked
# here so a wrong pair fails the plan rather than the apply.
variable "memory" {
  description = "Memory per container, twice cpu in Gi: 0.25 → \"0.5Gi\", 1 → \"2Gi\"."
  type        = string

  validation {
    condition     = var.memory == "${var.cpu * 2}Gi"
    error_message = "memory must be twice cpu in Gi, for example cpu = 0.25 with memory = \"0.5Gi\"."
  }
}

variable "log_retention_days" {
  description = "How long Log Analytics keeps the container logs."
  type        = number

  validation {
    condition     = var.log_retention_days >= 30 && var.log_retention_days <= 730
    error_message = "log_retention_days must be between 30 and 730."
  }
}

# ── Secrets ───────────────────────────────────────────────────────────────────

# kcl: vaultKeySessionKey (session-key)
variable "session_key" {
  description = "SP_SESSION_KEY, which signs the recognition cookie. In terraform.tfvars. Generate it once with `openssl rand -base64 32` and keep it: a new key logs every player out."
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.session_key) >= 32
    error_message = "session_key must be at least 32 characters."
  }
}

# kcl: kioskEnabled + vaultKeyKioskToken (kiosk-token)
variable "kiosk_token" {
  description = "SP_KIOSK_TOKEN, in terraform.tfvars. Left out or empty, the kiosk does not exist — its routes are not registered, rather than registered and unlocked."
  type        = string
  sensitive   = true
  default     = ""
}

# ── Database ──────────────────────────────────────────────────────────────────

# kcl: dbOwner
variable "postgres_user" {
  description = "Administrator login of the Flexible Server, and the role the application connects as."
  type        = string
}

# kcl: dbName
variable "postgres_db" {
  description = "Database name."
  type        = string
}

# kcl: vaultKeyDBPassword (password)
variable "postgres_password" {
  description = "Password of postgres_user, in terraform.tfvars. Letters and digits only, with upper case, lower case and a digit: openssl rand -base64 48 | tr -dc 'A-Za-z0-9' | cut -c1-40"
  type        = string
  sensitive   = true

  # Two constraints meet here. The password is interpolated into
  # SP_DATABASE_URL unescaped, so an @, /, : or ? would make the URL parse as
  # something else. And Flexible Server demands three of four character
  # classes, which rules out the lowercase hex `task kcl:secrets` generates for
  # the cluster. Mixed-case alphanumerics satisfy both.
  validation {
    condition = (
      can(regex("^[A-Za-z0-9]{16,128}$", var.postgres_password))
      && can(regex("[A-Z]", var.postgres_password))
      && can(regex("[a-z]", var.postgres_password))
      && can(regex("[0-9]", var.postgres_password))
    )
    error_message = "postgres_password must be 16 to 128 letters and digits, with at least one upper-case letter, one lower-case letter and one digit."
  }
}

# kcl: dbImage (ghcr.io/cloudnative-pg/postgresql:18)
# NOT the same major yet: schmetterpause.auto.tfvars still sets 17. Moving an
# existing Flexible Server is a major-version upgrade of its data and needs the
# region to offer 18, so it is a decision of its own, not a side effect of the
# kcl default (#213).
variable "postgres_version" {
  description = "PostgreSQL major version. One major across Compose, Kubernetes and Azure is what lets a dump move between them (docs/adr/0016, #213)."
  type        = string
}

variable "postgres_sku" {
  description = "Flexible Server SKU. Burstable B1ms is the smallest and is plenty for one office."
  type        = string
}

# kcl: dbStorageSize
variable "postgres_storage_mb" {
  description = "Flexible Server storage in MB. 32768 is the smallest it offers; it grows on its own from there."
  type        = number

  validation {
    condition     = var.postgres_storage_mb >= 32768
    error_message = "postgres_storage_mb must be at least 32768."
  }
}

variable "postgres_backup_retention_days" {
  description = "Days of Flexible Server's own backups. They are deleted with the server, so they protect a running instance, not a destroyed one (docs/adr/0016)."
  type        = number

  validation {
    condition     = var.postgres_backup_retention_days >= 7 && var.postgres_backup_retention_days <= 35
    error_message = "postgres_backup_retention_days must be between 7 and 35."
  }
}
