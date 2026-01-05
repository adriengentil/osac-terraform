/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package provider

import (
	"context"
	"log/slog"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fulfillmentv1 "github.com/innabox/fulfillment-common/api/fulfillment/v1"
	"github.com/innabox/fulfillment-common/auth"
	"github.com/innabox/fulfillment-common/network"
	"github.com/innabox/fulfillment-common/oauth"

	"github.com/innabox/terraform-provider-osac/internal/client"
	"github.com/innabox/terraform-provider-osac/internal/datasources"
	"github.com/innabox/terraform-provider-osac/internal/resources"
)

// Ensure OsacProvider satisfies various provider interfaces.
var _ provider.Provider = &OsacProvider{}

// OsacProvider defines the provider implementation.
type OsacProvider struct {
	version string
}

// OsacProviderModel describes the provider data model.
type OsacProviderModel struct {
	Endpoint     types.String `tfsdk:"endpoint"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Issuer       types.String `tfsdk:"issuer"`
	Insecure     types.Bool   `tfsdk:"insecure"`
	Plaintext    types.Bool   `tfsdk:"plaintext"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OsacProvider{
			version: version,
		}
	}
}

func (p *OsacProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "osac"
	resp.Version = p.version
}

func (p *OsacProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for OSAC (OpenShift Assisted Clusters) fulfillment API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "The gRPC endpoint address of the fulfillment API (e.g., api.example.com:443).",
				Required:    true,
			},
			"client_id": schema.StringAttribute{
				Description: "OAuth2 client ID for authentication.",
				Required:    true,
				Sensitive:   true,
			},
			"client_secret": schema.StringAttribute{
				Description: "OAuth2 client secret for authentication.",
				Required:    true,
				Sensitive:   true,
			},
			"issuer": schema.StringAttribute{
				Description: "OAuth2 issuer URL for token endpoint discovery.",
				Required:    true,
			},
			"insecure": schema.BoolAttribute{
				Description: "Skip TLS certificate verification. Not recommended for production.",
				Optional:    true,
			},
			"plaintext": schema.BoolAttribute{
				Description: "Use plaintext connection (no TLS). Not recommended for production.",
				Optional:    true,
			},
		},
	}
}

func (p *OsacProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config OsacProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create a logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Create OAuth token source using client credentials flow
	tokenStore, err := auth.NewMemoryTokenStore().
		SetLogger(logger).
		Build()
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create token store",
			err.Error(),
		)
		return
	}

	tokenSource, err := oauth.NewTokenSource().
		SetLogger(logger).
		SetFlow(oauth.CredentialsFlow).
		SetIssuer(config.Issuer.ValueString()).
		SetClientId(config.ClientID.ValueString()).
		SetClientSecret(config.ClientSecret.ValueString()).
		SetStore(tokenStore).
		Build()
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create OAuth token source",
			err.Error(),
		)
		return
	}

	// Build gRPC client options
	grpcBuilder := network.NewGrpcClient().
		SetLogger(logger).
		SetAddress(config.Endpoint.ValueString()).
		SetTokenSource(tokenSource)

	if !config.Insecure.IsNull() && config.Insecure.ValueBool() {
		grpcBuilder.SetInsecure(true)
	}

	if !config.Plaintext.IsNull() && config.Plaintext.ValueBool() {
		grpcBuilder.SetPlaintext(true)
	}

	conn, err := grpcBuilder.Build()
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create gRPC connection",
			err.Error(),
		)
		return
	}

	// Create provider data with all service clients
	providerData := &client.ProviderData{
		Conn:                           conn,
		ClustersClient:                 fulfillmentv1.NewClustersClient(conn),
		ClusterTemplatesClient:         fulfillmentv1.NewClusterTemplatesClient(conn),
		ComputeInstancesClient:         fulfillmentv1.NewComputeInstancesClient(conn),
		ComputeInstanceTemplatesClient: fulfillmentv1.NewComputeInstanceTemplatesClient(conn),
		HostsClient:                    fulfillmentv1.NewHostsClient(conn),
		HostClassesClient:              fulfillmentv1.NewHostClassesClient(conn),
		HostPoolsClient:                fulfillmentv1.NewHostPoolsClient(conn),
	}

	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func (p *OsacProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewClusterResource,
		resources.NewComputeInstanceResource,
		resources.NewHostResource,
		resources.NewHostPoolResource,
	}
}

func (p *OsacProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewClusterDataSource,
		datasources.NewClusterTemplateDataSource,
		datasources.NewComputeInstanceDataSource,
		datasources.NewComputeInstanceTemplateDataSource,
		datasources.NewHostDataSource,
		datasources.NewHostClassDataSource,
		datasources.NewHostPoolDataSource,
	}
}
