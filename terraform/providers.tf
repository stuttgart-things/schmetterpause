# Authenticates with the Azure CLI session of whoever runs it (`az login`), so a
# manual apply needs no service principal. azurerm v4 requires subscription_id.
provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
}
