# Every application setting below names the kcl/schema.k field it mirrors. The
# two describe the same contract, and a change to one lands in the other in the
# same pull request — see README.md.

# ── Azure ─────────────────────────────────────────────────────────────────────

variable "subscription_id" {
  description = "Azure subscription to deploy into."
  type        = string
}

variable "location" {
  description = "Azure region. It has to be allowed by the subscription's policy and offer PostgreSQL Flexible Server in postgres_version."
  type        = string
  default     = "westeurope"
}

variable "name_prefix" {
  description = "Prefix for every resource name: lowercase letters, digits and hyphens."
  type        = string
  default     = "schmetterpause"

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
# Renovate moves the default.
variable "image" {
  description = "Container image, pinned to a tag."
  type        = string
  default     = "ghcr.io/stuttgart-things/schmetterpause:v0.5.0"
}

# No kcl equivalent: a Deployment keeps its pod running regardless.
variable "min_replicas" {
  description = "1 keeps a replica warm; 0 scales to zero after a quiet period and cold-starts, migrations included, on the next request."
  type        = number
  default     = 1

  validation {
    condition     = contains([0, 1], var.min_replicas)
    error_message = "min_replicas must be 0 or 1 — there is never more than one replica, see main.tf."
  }
}

# kcl: logLevel
variable "log_level" {
  description = "SP_LOG_LEVEL."
  type        = string
  default     = "info"

  validation {
    condition     = contains(["debug", "info", "warn", "error"], var.log_level)
    error_message = "log_level must be one of debug, info, warn, error."
  }
}

# kcl: publicBaseURL
variable "public_base_url" {
  description = "SP_PUBLIC_BASE_URL, the scheme and host the QR sheet points at. Empty derives it from the app's generated *.azurecontainerapps.io address; set it once a custom domain is bound."
  type        = string
  default     = ""

  validation {
    condition     = var.public_base_url == "" || can(regex("^https?://[^/?#]+/?$", var.public_base_url))
    error_message = "public_base_url must be scheme and host only, for example https://schmetterpause.example.com."
  }
}

# kcl: bootstrapAdmin
variable "bootstrap_admin" {
  description = "SP_BOOTSTRAP_ADMIN: display name of the player who gets the admin flag at startup (docs/adr/0008). That player has to have joined already, so this belongs to a second apply, not the first."
  type        = string
  default     = ""
}

# kcl: extraEnvVars
variable "extra_env_vars" {
  description = "Plain environment variables for the app container, for settings this configuration has no variable for yet. Applied last, so they override. Not for secrets."
  type        = map(string)
  default     = {}

  validation {
    condition = alltrue([
      for k in keys(var.extra_env_vars) :
      !contains(["SP_DATABASE_URL", "SP_SESSION_KEY", "SP_KIOSK_TOKEN"], k)
    ])
    error_message = "extra_env_vars must not set SP_DATABASE_URL, SP_SESSION_KEY or SP_KIOSK_TOKEN — those are secrets and have variables of their own."
  }
}

# ── Secrets ───────────────────────────────────────────────────────────────────

# kcl: vaultKeySessionKey (session-key)
variable "session_key" {
  description = "SP_SESSION_KEY, which signs the recognition cookie. Generate it once with `openssl rand -base64 32` and keep it: a new key logs every player out."
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.session_key) >= 32
    error_message = "session_key must be at least 32 characters."
  }
}

# kcl: kioskEnabled + vaultKeyKioskToken (kiosk-token)
variable "kiosk_token" {
  description = "SP_KIOSK_TOKEN. Empty means the kiosk does not exist — its routes are not registered, rather than registered and unlocked."
  type        = string
  sensitive   = true
  default     = ""
}

# ── Database ──────────────────────────────────────────────────────────────────

# kcl: dbOwner
variable "postgres_user" {
  description = "Administrator login of the Flexible Server, and the role the application connects as."
  type        = string
  default     = "schmetterpause"
}

# kcl: dbName
variable "postgres_db" {
  description = "Database name."
  type        = string
  default     = "schmetterpause"
}

# kcl: vaultKeyDBPassword (password)
variable "postgres_password" {
  description = "Password of postgres_user. Letters and digits only, with upper case, lower case and a digit: openssl rand -base64 48 | tr -dc 'A-Za-z0-9' | cut -c1-40"
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

# kcl: dbImage (ghcr.io/cloudnative-pg/postgresql:17)
variable "postgres_version" {
  description = "PostgreSQL major version. Kept equal to the cluster's, so a dump moves between them without surprises."
  type        = string
  default     = "17"
}

variable "postgres_sku" {
  description = "Flexible Server SKU. Burstable B1ms is the smallest and is plenty for one office."
  type        = string
  default     = "B_Standard_B1ms"
}
