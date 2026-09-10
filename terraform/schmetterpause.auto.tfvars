# Every value of the Azure deployment that is not a secret, and the only place
# they are written down. Terraform loads this file on every plan, apply and
# destroy — no -var-file needed. The subscription and the secrets live in
# terraform.tfvars, which is gitignored; see terraform.tfvars.example.
#
# variables.tf declares and validates each of these and names the kcl field it
# mirrors. A new application setting lands in kcl/, variables.tf and here in
# the same pull request.

# ── Azure ─────────────────────────────────────────────────────────────────────

location    = "westeurope"
name_prefix = "schmetterpause"

# ── Application ───────────────────────────────────────────────────────────────

# Renovate moves this tag.
image = "ghcr.io/stuttgart-things/schmetterpause:v0.6.0"

min_replicas = 1
log_level    = "info"

# Empty: the app's generated *.azurecontainerapps.io address.
public_base_url = ""

# Empty grants nothing. A display name, once that player has joined.
bootstrap_admin = ""

extra_env_vars = {}

# Per container, for the app and its migrate init container each. Consumption
# pairs only: memory in Gi is twice the vCPU.
cpu    = 0.25
memory = "0.5Gi"

log_retention_days = 30

# ── Database ──────────────────────────────────────────────────────────────────

postgres_user    = "schmetterpause"
postgres_db      = "schmetterpause"
postgres_version = "17"
postgres_sku     = "B_Standard_B1ms"

postgres_storage_mb            = 32768
postgres_backup_retention_days = 7
