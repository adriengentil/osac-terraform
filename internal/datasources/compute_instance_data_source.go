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
var _ datasource.DataSource = &ComputeInstanceDataSource{}

func NewComputeInstanceDataSource() datasource.DataSource {
	return &ComputeInstanceDataSource{}
}

// ComputeInstanceDataSource defines the data source implementation.
type ComputeInstanceDataSource struct {
	client fulfillmentv1.ComputeInstancesClient
}

// ComputeInstanceDataSourceModel describes the data source data model.
type ComputeInstanceDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Template  types.String `tfsdk:"template"`
	State     types.String `tfsdk:"state"`
	IPAddress types.String `tfsdk:"ip_address"`
}

func (d *ComputeInstanceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance"
}

func (d *ComputeInstanceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an existing OSAC compute instance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the compute instance.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Human-friendly name of the compute instance.",
				Computed:    true,
			},
			"template": schema.StringAttribute{
				Description: "Reference to the compute instance template ID.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "Current state of the compute instance.",
				Computed:    true,
			},
			"ip_address": schema.StringAttribute{
				Description: "IP address of the compute instance.",
				Computed:    true,
			},
		},
	}
}

func (d *ComputeInstanceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = providerData.ComputeInstancesClient
}

func (d *ComputeInstanceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ComputeInstanceDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Get(ctx, &fulfillmentv1.ComputeInstancesGetRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read compute instance", err.Error())
		return
	}

	instance := getResp.Object
	data.ID = types.StringValue(instance.Id)

	if instance.Metadata != nil {
		data.Name = types.StringValue(instance.Metadata.Name)
	}

	if instance.Spec != nil {
		data.Template = types.StringValue(instance.Spec.Template)
	}

	if instance.Status != nil {
		data.State = types.StringValue(instance.Status.State.String())
		data.IPAddress = types.StringValue(instance.Status.IpAddress)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
