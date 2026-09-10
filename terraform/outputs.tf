output "app_url" {
  description = "The app's generated HTTPS address."
  value       = "https://${azurerm_container_app.this.ingress[0].fqdn}"
}

output "public_base_url" {
  description = "What SP_PUBLIC_BASE_URL was set to — where the QR sheet points."
  value       = local.app_env["SP_PUBLIC_BASE_URL"]
}

output "resource_group" {
  description = "The resource group holding everything. Deleting it removes the database and its data."
  value       = azurerm_resource_group.this.name
}

output "postgres_fqdn" {
  description = "Host name of the Flexible Server, for psql and pg_restore."
  value       = azurerm_postgresql_flexible_server.this.fqdn
}
