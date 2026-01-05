# Example Terraform configuration for the OSAC provider

terraform {
  required_providers {
    osac = {
      source = "registry.terraform.io/innabox/osac"
    }
  }
}

# Configure the OSAC provider
provider "osac" {
  endpoint      = var.osac_endpoint
  client_id     = var.osac_client_id
  client_secret = var.osac_client_secret
  issuer        = var.osac_issuer
  # Optional: for development environments
  # insecure  = true
  # plaintext = true
}

# Variables
variable "osac_endpoint" {
  description = "The gRPC endpoint of the OSAC fulfillment API"
  type        = string
}

variable "osac_client_id" {
  description = "OAuth2 client ID"
  type        = string
  sensitive   = true
}

variable "osac_client_secret" {
  description = "OAuth2 client secret"
  type        = string
  sensitive   = true
}

variable "osac_issuer" {
  description = "OAuth2 issuer URL"
  type        = string
}

# Data source: Look up a cluster template
data "osac_cluster_template" "example" {
  id = "my-template-id"
}

# Data source: Look up a host class
data "osac_host_class" "compute" {
  id = "acme_1tb"
}

# Resource: Create a cluster
resource "osac_cluster" "example" {
  name     = "my-cluster"
  template = data.osac_cluster_template.example.id

  node_sets = {
    compute = {
      host_class = data.osac_host_class.compute.id
      size       = 3
    }
  }
}

# Resource: Create a host pool
resource "osac_host_pool" "example" {
  name = "my-host-pool"

  host_sets = {
    main = {
      host_class = data.osac_host_class.compute.id
      size       = 5
    }
  }
}

# Outputs
output "cluster_id" {
  description = "The ID of the created cluster"
  value       = osac_cluster.example.id
}

output "cluster_api_url" {
  description = "The API URL of the cluster"
  value       = osac_cluster.example.api_url
}

output "cluster_console_url" {
  description = "The console URL of the cluster"
  value       = osac_cluster.example.console_url
}

output "cluster_state" {
  description = "The current state of the cluster"
  value       = osac_cluster.example.state
}

output "host_pool_id" {
  description = "The ID of the created host pool"
  value       = osac_host_pool.example.id
}

output "host_pool_hosts" {
  description = "The hosts assigned to the pool"
  value       = osac_host_pool.example.hosts
}

