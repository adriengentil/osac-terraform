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
  token         = var.osac_token
  insecure      = true
}

# Variables
variable "osac_endpoint" {
  description = "The gRPC endpoint of the OSAC fulfillment API"
  type        = string
}

variable "osac_token" {
  description = "OAuth2 client ID"
  type        = string
  sensitive   = true
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
  ssh_public_key = file(pathexpand(var.ssh_public_key_file))

  # Cloud-init configuration
  cloud_init = <<-EOF
    #cloud-config
    packages:
      - nginx
    runcmd:
      - systemctl enable --now nginx
  EOF
}

# Look up the compute instance template
data "osac_compute_instance_template" "vm" {
  id = "osac.templates.ocp_virt_vm"
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