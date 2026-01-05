/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fulfillmentv1 "github.com/innabox/fulfillment-common/api/fulfillment/v1"
	sharedv1 "github.com/innabox/fulfillment-common/api/shared/v1"

	"github.com/innabox/terraform-provider-osac/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &HostResource{}
var _ resource.ResourceWithImportState = &HostResource{}

func NewHostResource() resource.Resource {
	return &HostResource{}
}

// HostResource defines the resource implementation.
type HostResource struct {
	client fulfillmentv1.HostsClient
}

// HostResourceModel describes the resource data model.
type HostResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	PowerState types.String `tfsdk:"power_state"`
	// Computed status fields
	State             types.String `tfsdk:"state"`
	CurrentPowerState types.String `tfsdk:"current_power_state"`
}

func (r *HostResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (r *HostResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an OSAC host.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the host.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Human-friendly name of the host.",
				Optional:    true,
			},
			"power_state": schema.StringAttribute{
				Description: "Desired power state of the host (ON, OFF).",
				Optional:    true,
			},
			"state": schema.StringAttribute{
				Description: "Current state of the host (PROGRESSING, READY, FAILED).",
				Computed:    true,
			},
			"current_power_state": schema.StringAttribute{
				Description: "Current power state of the host.",
				Computed:    true,
			},
		},
	}
}

func (r *HostResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*client.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ProviderData, got: %T", req.ProviderData),
		)
		return
	}

	r.client = providerData.HostsClient
}

func (r *HostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HostResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the host spec
	spec := &fulfillmentv1.HostSpec{}

	if !data.PowerState.IsNull() {
		spec.PowerState = parsePowerState(data.PowerState.ValueString())
	}

	// Build the host
	host := &fulfillmentv1.Host{
		Spec: spec,
	}

	// Set metadata if name is provided
	if !data.Name.IsNull() {
		host.Metadata = &sharedv1.Metadata{
			Name: data.Name.ValueString(),
		}
	}

	// Create the host
	createResp, err := r.client.Create(ctx, &fulfillmentv1.HostsCreateRequest{
		Object: host,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create host", err.Error())
		return
	}

	// Update state with response
	r.updateModelFromHost(&data, createResp.Object)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HostResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Get(ctx, &fulfillmentv1.HostsGetRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read host", err.Error())
		return
	}

	r.updateModelFromHost(&data, getResp.Object)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data HostResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the update request
	spec := &fulfillmentv1.HostSpec{}

	if !data.PowerState.IsNull() {
		spec.PowerState = parsePowerState(data.PowerState.ValueString())
	}

	host := &fulfillmentv1.Host{
		Id:   data.ID.ValueString(),
		Spec: spec,
	}

	if !data.Name.IsNull() {
		host.Metadata = &sharedv1.Metadata{
			Name: data.Name.ValueString(),
		}
	}

	updateResp, err := r.client.Update(ctx, &fulfillmentv1.HostsUpdateRequest{
		Object: host,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update host", err.Error())
		return
	}

	r.updateModelFromHost(&data, updateResp.Object)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HostResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, &fulfillmentv1.HostsDeleteRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete host", err.Error())
		return
	}
}

func (r *HostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *HostResource) updateModelFromHost(model *HostResourceModel, host *fulfillmentv1.Host) {
	model.ID = types.StringValue(host.Id)

	if host.Metadata != nil {
		model.Name = types.StringValue(host.Metadata.Name)
	}

	if host.Spec != nil {
		model.PowerState = types.StringValue(host.Spec.PowerState.String())
	}

	if host.Status != nil {
		model.State = types.StringValue(host.Status.State.String())
		model.CurrentPowerState = types.StringValue(host.Status.PowerState.String())
	}
}

func parsePowerState(s string) fulfillmentv1.HostPowerState {
	switch s {
	case "HOST_POWER_STATE_ON", "ON":
		return fulfillmentv1.HostPowerState_HOST_POWER_STATE_ON
	case "HOST_POWER_STATE_OFF", "OFF":
		return fulfillmentv1.HostPowerState_HOST_POWER_STATE_OFF
	default:
		return fulfillmentv1.HostPowerState_HOST_POWER_STATE_UNSPECIFIED
	}
}
