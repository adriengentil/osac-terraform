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
var _ datasource.DataSource = &HostPoolDataSource{}

func NewHostPoolDataSource() datasource.DataSource {
	return &HostPoolDataSource{}
}

// HostPoolDataSource defines the data source implementation.
type HostPoolDataSource struct {
	client fulfillmentv1.HostPoolsClient
}

// HostPoolDataSourceModel describes the data source data model.
type HostPoolDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	State types.String `tfsdk:"state"`
	Hosts types.List   `tfsdk:"hosts"`
}

func (d *HostPoolDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host_pool"
}

func (d *HostPoolDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an existing OSAC host pool.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the host pool.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Human-friendly name of the host pool.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "Current state of the host pool.",
				Computed:    true,
			},
			"hosts": schema.ListAttribute{
				Description: "List of host IDs assigned to this pool.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *HostPoolDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = providerData.HostPoolsClient
}

func (d *HostPoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data HostPoolDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Get(ctx, &fulfillmentv1.HostPoolsGetRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read host pool", err.Error())
		return
	}

	hostPool := getResp.Object
	data.ID = types.StringValue(hostPool.Id)

	if hostPool.Metadata != nil {
		data.Name = types.StringValue(hostPool.Metadata.Name)
	}

	if hostPool.Status != nil {
		data.State = types.StringValue(hostPool.Status.State.String())

		// Convert hosts list
		hosts := make([]types.String, len(hostPool.Status.Hosts))
		for i, h := range hostPool.Status.Hosts {
			hosts[i] = types.StringValue(h)
		}
		hostsValue, diags := types.ListValueFrom(ctx, types.StringType, hosts)
		resp.Diagnostics.Append(diags...)
		data.Hosts = hostsValue
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
