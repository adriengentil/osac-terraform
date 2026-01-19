# Example: Provision a compute instance with SSH public key and cloud-init

terraform {
  required_providers {
    osac = {
      source = "innabox/osac"
    }
  }
}

# Configure the OSAC provider
provider "osac" {
  endpoint      = var.osac_endpoint
  client_id     = var.osac_client_id
  client_secret = var.osac_client_secret
  issuer        = var.osac_issuer
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

variable "ssh_public_key" {
  description = "SSH public key for accessing the VM"
  type        = string
  default     = ""
}

variable "ssh_public_key_file" {
  description = "Path to SSH public key file (alternative to ssh_public_key)"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

variable "vm_name" {
  description = "Name of the compute instance"
  type        = string
  default     = "my-vm"
}

# Read SSH public key from file if not provided directly
locals {
  ssh_public_key = var.ssh_public_key != "" ? var.ssh_public_key : file(pathexpand(var.ssh_public_key_file))

  # Cloud-init configuration
  cloud_init = <<-EOF
    #cloud-config
    users:
      - name: admin
        sudo: ALL=(ALL) NOPASSWD:ALL
        shell: /bin/bash
        ssh_authorized_keys:
          - ${local.ssh_public_key}

    package_update: true
    package_upgrade: true

    packages:
      - vim
      - curl
      - wget
      - git

    runcmd:
      - echo "VM provisioned at $(date)" > /var/log/provisioning.log

    final_message: "Cloud-init completed after $UPTIME seconds"
  EOF
}

# Look up the compute instance template
data "osac_compute_instance_template" "vm" {
  id = "standard-vm"  # Replace with your actual template ID
}

# Create the compute instance
resource "osac_compute_instance" "vm" {
  name     = var.vm_name
  template = data.osac_compute_instance_template.vm.id

  template_parameters = {
    ssh_public_key = local.ssh_public_key
    cloud_init     = base64encode(local.cloud_init)
  }
}

# Outputs
output "vm_id" {
  description = "The ID of the compute instance"
  value       = osac_compute_instance.vm.id
}

output "vm_state" {
  description = "The current state of the compute instance"
  value       = osac_compute_instance.vm.state
}

output "vm_ip_address" {
  description = "The IP address of the compute instance"
  value       = osac_compute_instance.vm.ip_address
}

output "ssh_command" {
  description = "SSH command to connect to the VM"
  value       = osac_compute_instance.vm.ip_address != "" ? "ssh admin@${osac_compute_instance.vm.ip_address}" : "VM not ready yet"
}
