resource "azurerm_resource_group" "main" {
  name     = "main"
  location = "westeurope"
}

resource "azurerm_kubernetes_cluster" "aks" {
  name                = "aks"
  resource_group_name = azurerm_resource_group.main.name
}

resource "azurerm_cosmosdb_account" "db" {
  name = "orders"
}

resource "azurerm_linux_function_app" "fn" {
  name = "orders-fn"
}
