# Compute Instance with SSH and Cloud-Init Example

This example demonstrates how to provision a compute instance with:
- SSH public key for authentication
- Cloud-init configuration for initial setup

## Prerequisites

1. An SSH key pair. Generate one if you don't have it:
   ```bash
   ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa
   ```

2. OSAC provider credentials (client_id, client_secret, issuer)

## Usage

1. Copy the example variables file:
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   ```

2. Edit `terraform.tfvars` with your values.

3. Initialize Terraform:
   ```bash
   terraform init
   ```

4. Review the plan:
   ```bash
   terraform plan
   ```

5. Apply the configuration:
   ```bash
   terraform apply
   ```

6. Connect to the VM:
   ```bash
   ssh admin@<ip_address>
   ```

## Cloud-Init Configuration

The example cloud-init configuration:
- Creates an `admin` user with sudo access
- Installs the SSH public key for passwordless authentication
- Updates packages
- Installs common utilities (vim, curl, wget, git)

### Customizing Cloud-Init

You can customize the cloud-init configuration in `main.tf` by modifying the `cloud_init` local. Common customizations:

```yaml
#cloud-config
# Add more users
users:
  - name: admin
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - ssh-rsa AAAA...

# Install additional packages
packages:
  - docker.io
  - python3-pip

# Run custom commands
runcmd:
  - systemctl enable docker
  - systemctl start docker

# Write files
write_files:
  - path: /etc/myapp/config.yaml
    content: |
      setting: value
```

## Outputs

| Output | Description |
|--------|-------------|
| `vm_id` | The unique identifier of the VM |
| `vm_state` | Current state (PROGRESSING, READY, FAILED) |
| `vm_ip_address` | IP address to connect to |
| `ssh_command` | Ready-to-use SSH command |

## Cleanup

To destroy the VM:
```bash
terraform destroy
```
