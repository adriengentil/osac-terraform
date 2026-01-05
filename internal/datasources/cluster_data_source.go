/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fulfillmentv1 "github.com/innabox/fulfillment-common/api/fulfillment/v1"

	"github.com/innabox/terraform-provider-osac/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ClusterDataSource{}

func NewClusterDataSource() datasource.DataSource {
	return &ClusterDataSource{}
}

// ClusterDataSource defines the data source implementation.
type ClusterDataSource struct {
	client fulfillmentv1.ClustersClient
}

// ClusterDataSourceModel describes the data source data model.
type ClusterDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Template   types.String `tfsdk:"template"`
	State      types.String `tfsdk:"state"`
	ApiURL     types.String `tfsdk:"api_url"`
	ConsoleURL types.String `tfsdk:"console_url"`
}

func (d *ClusterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (d *ClusterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an existing OSAC cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the cluster.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Human-friendly name of the cluster.",
				Computed:    true,
			},
			"template": schema.StringAttribute{
				Description: "Reference to the cluster template ID.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "Current state of the cluster.",
				Computed:    true,
			},
			"api_url": schema.StringAttribute{
				Description: "URL of the API server of the cluster.",
				Computed:    true,
			},
			"console_url": schema.StringAttribute{
				Description: "URL of the console of the cluster.",
				Computed:    true,
			},
		},
	}
}

func (d *ClusterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*client.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ProviderData, got: %T", req.ProviderData),
		)
		return
	}

	d.client = providerData.ClustersClient
}

func (d *ClusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ClusterDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Get(ctx, &fulfillmentv1.ClustersGetRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read cluster", err.Error())
		return
	}

	cluster := getResp.Object
	data.ID = types.StringValue(cluster.Id)

	if cluster.Metadata != nil {
		data.Name = types.StringValue(cluster.Metadata.Name)
	}

	if cluster.Spec != nil {
		data.Template = types.StringValue(cluster.Spec.Template)
	}

	if cluster.Status != nil {
		data.State = types.StringValue(cluster.Status.State.String())
		data.ApiURL = types.StringValue(cluster.Status.ApiUrl)
		data.ConsoleURL = types.StringValue(cluster.Status.ConsoleUrl)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
