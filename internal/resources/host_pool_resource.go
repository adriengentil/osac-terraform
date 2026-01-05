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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
var _ resource.Resource = &HostPoolResource{}
var _ resource.ResourceWithImportState = &HostPoolResource{}

func NewHostPoolResource() resource.Resource {
	return &HostPoolResource{}
}

// HostPoolResource defines the resource implementation.
type HostPoolResource struct {
	client fulfillmentv1.HostPoolsClient
}

// HostPoolResourceModel describes the resource data model.
type HostPoolResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	HostSets types.Map    `tfsdk:"host_sets"`
	// Computed status fields
	State types.String `tfsdk:"state"`
	Hosts types.List   `tfsdk:"hosts"`
}

// HostSetModel represents a host set in Terraform state
type HostSetModel struct {
	HostClass types.String `tfsdk:"host_class"`
	Size      types.Int32  `tfsdk:"size"`
}

func (r *HostPoolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host_pool"
}

func (r *HostPoolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an OSAC host pool.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the host pool.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Human-friendly name of the host pool.",
				Optional:    true,
			},
			"host_sets": schema.MapNestedAttribute{
				Description: "Desired host sets of the host pool.",
				Optional:    true,
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"host_class": schema.StringAttribute{
							Description: "Identifier of the class of hosts in this set.",
							Required:    true,
						},
						"size": schema.Int32Attribute{
							Description: "Number of hosts in the set.",
							Required:    true,
						},
					},
				},
			},
			"state": schema.StringAttribute{
				Description: "Current state of the host pool (PROGRESSING, READY, FAILED).",
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

func (r *HostPoolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = providerData.HostPoolsClient
}

func (r *HostPoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HostPoolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the host pool spec
	spec := &fulfillmentv1.HostPoolSpec{}

	// Build host sets if provided
	if !data.HostSets.IsNull() && !data.HostSets.IsUnknown() {
		hostSetsMap := make(map[string]HostSetModel)
		resp.Diagnostics.Append(data.HostSets.ElementsAs(ctx, &hostSetsMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		spec.HostSets = make(map[string]*fulfillmentv1.HostPoolHostSet)
		for name, hs := range hostSetsMap {
			spec.HostSets[name] = &fulfillmentv1.HostPoolHostSet{
				HostClass: hs.HostClass.ValueString(),
				Size:      hs.Size.ValueInt32(),
			}
		}
	}

	// Build the host pool
	hostPool := &fulfillmentv1.HostPool{
		Spec: spec,
	}

	// Set metadata if name is provided
	if !data.Name.IsNull() {
		hostPool.Metadata = &sharedv1.Metadata{
			Name: data.Name.ValueString(),
		}
	}

	// Create the host pool
	createResp, err := r.client.Create(ctx, &fulfillmentv1.HostPoolsCreateRequest{
		Object: hostPool,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create host pool", err.Error())
		return
	}

	// Update state with response
	r.updateModelFromHostPool(ctx, &data, createResp.Object, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostPoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HostPoolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Get(ctx, &fulfillmentv1.HostPoolsGetRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read host pool", err.Error())
		return
	}

	r.updateModelFromHostPool(ctx, &data, getResp.Object, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostPoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data HostPoolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the update request
	spec := &fulfillmentv1.HostPoolSpec{}

	// Update host sets if provided
	if !data.HostSets.IsNull() && !data.HostSets.IsUnknown() {
		hostSetsMap := make(map[string]HostSetModel)
		resp.Diagnostics.Append(data.HostSets.ElementsAs(ctx, &hostSetsMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		spec.HostSets = make(map[string]*fulfillmentv1.HostPoolHostSet)
		for name, hs := range hostSetsMap {
			spec.HostSets[name] = &fulfillmentv1.HostPoolHostSet{
				HostClass: hs.HostClass.ValueString(),
				Size:      hs.Size.ValueInt32(),
			}
		}
	}

	hostPool := &fulfillmentv1.HostPool{
		Id:   data.ID.ValueString(),
		Spec: spec,
	}

	if !data.Name.IsNull() {
		hostPool.Metadata = &sharedv1.Metadata{
			Name: data.Name.ValueString(),
		}
	}

	updateResp, err := r.client.Update(ctx, &fulfillmentv1.HostPoolsUpdateRequest{
		Object: hostPool,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update host pool", err.Error())
		return
	}

	r.updateModelFromHostPool(ctx, &data, updateResp.Object, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostPoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HostPoolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, &fulfillmentv1.HostPoolsDeleteRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete host pool", err.Error())
		return
	}
}

func (r *HostPoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *HostPoolResource) updateModelFromHostPool(ctx context.Context, model *HostPoolResourceModel, hostPool *fulfillmentv1.HostPool, diags *diag.Diagnostics) {
	model.ID = types.StringValue(hostPool.Id)

	if hostPool.Metadata != nil {
		model.Name = types.StringValue(hostPool.Metadata.Name)
	}

	if hostPool.Spec != nil && hostPool.Spec.HostSets != nil {
		hostSets := make(map[string]HostSetModel)
		for name, hs := range hostPool.Spec.HostSets {
			hostSets[name] = HostSetModel{
				HostClass: types.StringValue(hs.HostClass),
				Size:      types.Int32Value(hs.Size),
			}
		}
		hostSetsValue, d := types.MapValueFrom(ctx, types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"host_class": types.StringType,
				"size":       types.Int32Type,
			},
		}, hostSets)
		diags.Append(d...)
		model.HostSets = hostSetsValue
	}

	if hostPool.Status != nil {
		model.State = types.StringValue(hostPool.Status.State.String())

		// Convert hosts list
		hosts := make([]types.String, len(hostPool.Status.Hosts))
		for i, h := range hostPool.Status.Hosts {
			hosts[i] = types.StringValue(h)
		}
		hostsValue, d := types.ListValueFrom(ctx, types.StringType, hosts)
		diags.Append(d...)
		model.Hosts = hostsValue
	}
}
