terraform {
  # 1.9 for the memory validation, which reads var.cpu — a validation that
  # refers to another variable is refused before that. tests/ need 1.7 for
  # mock_provider.
  required_version = ">= 1.9"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 5.0"
    }
  }
}
