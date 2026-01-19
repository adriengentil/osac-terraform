# Terraform Provider for OSAC

Terraform provider for managing resources in the OSAC (OpenShift Assisted Clusters) fulfillment API.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.24 (to build the provider)

## Building the Provider

```bash
go build -o terraform-provider-osac
```

## Installing the Provider

To install the provider locally for development:

```bash
go install .
```

## Using the Provider

### Provider Configuration

The provider supports two authentication methods: **token authentication** or **OAuth2 client credentials**.

#### Option 1: Token Authentication

Use a static access token for authentication:

```hcl
terraform {
  required_providers {
    osac = {
      source = "innabox/osac"
    }
  }
}

provider "osac" {
  endpoint = "api.example.com:443"
  token    = var.osac_token
}
```

#### Option 2: OAuth2 Client Credentials

Use OAuth2 client credentials flow for authentication:

```hcl
terraform {
  required_providers {
    osac = {
      source = "innabox/osac"
    }
  }
}

provider "osac" {
  endpoint      = "api.example.com:443"
  client_id     = var.osac_client_id
  client_secret = var.osac_client_secret
  issuer        = "https://auth.example.com"
}
```

#### Development Options

For development environments, you can disable TLS verification:

```hcl
provider "osac" {
  endpoint = "api.example.com:443"
  token    = var.osac_token

  # Skip TLS certificate verification (not recommended for production)
  insecure = true

  # Use plaintext connection without TLS (not recommended for production)
  # plaintext = true
}
```

### Provider Arguments

| Argument | Description | Required |
|----------|-------------|----------|
| `endpoint` | gRPC endpoint address of the fulfillment API | Yes |
| `token` | Access token for authentication (use this OR OAuth2 credentials) | No* |
| `client_id` | OAuth2 client ID for authentication | No* |
| `client_secret` | OAuth2 client secret for authentication | No* |
| `issuer` | OAuth2 issuer URL for token endpoint discovery | No* |
| `insecure` | Skip TLS certificate verification (not recommended for production) | No |
| `plaintext` | Use plaintext connection without TLS (not recommended for production) | No |

\* You must provide either `token` OR all three OAuth2 credentials (`client_id`, `client_secret`, `issuer`)

## Resources

### osac_cluster

Manages an OSAC cluster.

```hcl
resource "osac_cluster" "example" {
  name     = "my-cluster"
  template = "my-template-id"

  node_sets = {
    compute = {
      host_class = "acme_1tb"
      size       = 3
    }
  }
}
```

#### Arguments

- `name` - (Optional) Human-friendly name of the cluster.
- `template` - (Required) Reference to the cluster template ID. Cannot be changed after creation.
- `template_parameters` - (Optional) Map of template parameter values. Cannot be changed after creation.
- `node_sets` - (Optional) Map of node sets, each with `host_class` and `size`.

#### Attributes

- `id` - Unique identifier of the cluster.
- `state` - Current state of the cluster (PROGRESSING, READY, FAILED).
- `api_url` - URL of the API server of the cluster.
- `console_url` - URL of the console of the cluster.

### osac_compute_instance

Manages an OSAC compute instance.

```hcl
resource "osac_compute_instance" "example" {
  name     = "my-instance"
  template = "my-template-id"
}
```

#### Arguments

- `name` - (Optional) Human-friendly name of the compute instance.
- `template` - (Required) Reference to the compute instance template ID.
- `template_parameters` - (Optional) Map of template parameter values.

#### Attributes

- `id` - Unique identifier of the compute instance.
- `state` - Current state (PROGRESSING, READY, FAILED).
- `ip_address` - IP address of the compute instance.

### osac_host

Manages an OSAC host.

```hcl
resource "osac_host" "example" {
  name        = "my-host"
  power_state = "ON"
}
```

#### Arguments

- `name` - (Optional) Human-friendly name of the host.
- `power_state` - (Optional) Desired power state (ON, OFF).

#### Attributes

- `id` - Unique identifier of the host.
- `state` - Current state (PROGRESSING, READY, FAILED).
- `current_power_state` - Current power state of the host.

### osac_host_pool

Manages an OSAC host pool.

```hcl
resource "osac_host_pool" "example" {
  name = "my-host-pool"

  host_sets = {
    main = {
      host_class = "acme_1tb"
      size       = 5
    }
  }
}
```

#### Arguments

- `name` - (Optional) Human-friendly name of the host pool.
- `host_sets` - (Optional) Map of host sets, each with `host_class` and `size`.

#### Attributes

- `id` - Unique identifier of the host pool.
- `state` - Current state (PROGRESSING, READY, FAILED).
- `hosts` - List of host IDs assigned to this pool.

## Data Sources

### osac_cluster

Fetches information about an existing cluster.

```hcl
data "osac_cluster" "example" {
  id = "cluster-id"
}
```

### osac_cluster_template

Fetches information about a cluster template.

```hcl
data "osac_cluster_template" "example" {
  id = "template-id"
}
```

### osac_compute_instance

Fetches information about an existing compute instance.

```hcl
data "osac_compute_instance" "example" {
  id = "instance-id"
}
```

### osac_compute_instance_template

Fetches information about a compute instance template.

```hcl
data "osac_compute_instance_template" "example" {
  id = "template-id"
}
```

### osac_host

Fetches information about an existing host.

```hcl
data "osac_host" "example" {
  id = "host-id"
}
```

### osac_host_class

Fetches information about a host class.

```hcl
data "osac_host_class" "example" {
  id = "acme_1tb"
}
```

### osac_host_pool

Fetches information about an existing host pool.

```hcl
data "osac_host_pool" "example" {
  id = "pool-id"
}
```

## Development

### Running Tests

```bash
go test ./...
```

### Generating Documentation

Documentation is generated from the schema descriptions in the provider code.

## License

Apache License 2.0

