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
var _ datasource.DataSource = &ComputeInstanceTemplateDataSource{}

func NewComputeInstanceTemplateDataSource() datasource.DataSource {
	return &ComputeInstanceTemplateDataSource{}
}

// ComputeInstanceTemplateDataSource defines the data source implementation.
type ComputeInstanceTemplateDataSource struct {
	client fulfillmentv1.ComputeInstanceTemplatesClient
}

// ComputeInstanceTemplateDataSourceModel describes the data source data model.
type ComputeInstanceTemplateDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
}

func (d *ComputeInstanceTemplateDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance_template"
}

func (d *ComputeInstanceTemplateDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an OSAC compute instance template.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the compute instance template.",
				Required:    true,
			},
			"title": schema.StringAttribute{
				Description: "Human-friendly short description of the template.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Human-friendly long description of the template in Markdown format.",
				Computed:    true,
			},
		},
	}
}

func (d *ComputeInstanceTemplateDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = providerData.ComputeInstanceTemplatesClient
}

func (d *ComputeInstanceTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ComputeInstanceTemplateDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Get(ctx, &fulfillmentv1.ComputeInstanceTemplatesGetRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read compute instance template", err.Error())
		return
	}

	template := getResp.Object
	data.ID = types.StringValue(template.Id)
	data.Title = types.StringValue(template.Title)
	data.Description = types.StringValue(template.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
