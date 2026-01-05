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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fulfillmentv1 "github.com/innabox/fulfillment-common/api/fulfillment/v1"
	sharedv1 "github.com/innabox/fulfillment-common/api/shared/v1"

	"github.com/innabox/terraform-provider-osac/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ClusterResource{}
var _ resource.ResourceWithImportState = &ClusterResource{}

func NewClusterResource() resource.Resource {
	return &ClusterResource{}
}

// ClusterResource defines the resource implementation.
type ClusterResource struct {
	client fulfillmentv1.ClustersClient
}

// ClusterResourceModel describes the resource data model.
type ClusterResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Template           types.String `tfsdk:"template"`
	TemplateParameters types.Map    `tfsdk:"template_parameters"`
	NodeSets           types.Map    `tfsdk:"node_sets"`
	// Computed status fields
	State      types.String `tfsdk:"state"`
	ApiURL     types.String `tfsdk:"api_url"`
	ConsoleURL types.String `tfsdk:"console_url"`
}

func (r *ClusterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (r *ClusterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an OSAC cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the cluster.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Human-friendly name of the cluster.",
				Optional:    true,
			},
			"template": schema.StringAttribute{
				Description: "Reference to the cluster template ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"template_parameters": schema.MapAttribute{
				Description: "Values of the template parameters as a map of strings.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"node_sets": schema.MapNestedAttribute{
				Description: "Desired node sets of the cluster.",
				Optional:    true,
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"host_class": schema.StringAttribute{
							Description: "Identifier of the class of hosts in this set.",
							Required:    true,
						},
						"size": schema.Int32Attribute{
							Description: "Number of nodes in the set.",
							Required:    true,
						},
					},
				},
			},
			"state": schema.StringAttribute{
				Description: "Current state of the cluster (PROGRESSING, READY, FAILED).",
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

func (r *ClusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = providerData.ClustersClient
}

func (r *ClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the cluster spec
	clusterSpec := &fulfillmentv1.ClusterSpec{
		Template: data.Template.ValueString(),
	}

	// Build node sets if provided
	if !data.NodeSets.IsNull() && !data.NodeSets.IsUnknown() {
		nodeSetsMap := make(map[string]NodeSetModel)
		resp.Diagnostics.Append(data.NodeSets.ElementsAs(ctx, &nodeSetsMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		clusterSpec.NodeSets = make(map[string]*fulfillmentv1.ClusterNodeSet)
		for name, ns := range nodeSetsMap {
			clusterSpec.NodeSets[name] = &fulfillmentv1.ClusterNodeSet{
				HostClass: ns.HostClass.ValueString(),
				Size:      ns.Size.ValueInt32(),
			}
		}
	}

	// Build the cluster
	cluster := &fulfillmentv1.Cluster{
		Spec: clusterSpec,
	}

	// Set metadata if name is provided
	if !data.Name.IsNull() {
		cluster.Metadata = &sharedv1.Metadata{
			Name: data.Name.ValueString(),
		}
	}

	// Create the cluster
	createResp, err := r.client.Create(ctx, &fulfillmentv1.ClustersCreateRequest{
		Object: cluster,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create cluster", err.Error())
		return
	}

	// Update state with response
	r.updateModelFromCluster(ctx, &data, createResp.Object, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Get(ctx, &fulfillmentv1.ClustersGetRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read cluster", err.Error())
		return
	}

	r.updateModelFromCluster(ctx, &data, getResp.Object, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the update request
	cluster := &fulfillmentv1.Cluster{
		Id: data.ID.ValueString(),
		Spec: &fulfillmentv1.ClusterSpec{
			Template: data.Template.ValueString(),
		},
	}

	// Update node sets if provided
	if !data.NodeSets.IsNull() && !data.NodeSets.IsUnknown() {
		nodeSetsMap := make(map[string]NodeSetModel)
		resp.Diagnostics.Append(data.NodeSets.ElementsAs(ctx, &nodeSetsMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		cluster.Spec.NodeSets = make(map[string]*fulfillmentv1.ClusterNodeSet)
		for name, ns := range nodeSetsMap {
			cluster.Spec.NodeSets[name] = &fulfillmentv1.ClusterNodeSet{
				HostClass: ns.HostClass.ValueString(),
				Size:      ns.Size.ValueInt32(),
			}
		}
	}

	if !data.Name.IsNull() {
		cluster.Metadata = &sharedv1.Metadata{
			Name: data.Name.ValueString(),
		}
	}

	updateResp, err := r.client.Update(ctx, &fulfillmentv1.ClustersUpdateRequest{
		Object: cluster,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update cluster", err.Error())
		return
	}

	r.updateModelFromCluster(ctx, &data, updateResp.Object, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ClusterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, &fulfillmentv1.ClustersDeleteRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete cluster", err.Error())
		return
	}
}

func (r *ClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ClusterResource) updateModelFromCluster(ctx context.Context, model *ClusterResourceModel, cluster *fulfillmentv1.Cluster, diags *diag.Diagnostics) {
	model.ID = types.StringValue(cluster.Id)

	if cluster.Metadata != nil {
		model.Name = types.StringValue(cluster.Metadata.Name)
	}

	if cluster.Spec != nil {
		model.Template = types.StringValue(cluster.Spec.Template)

		// Convert node sets
		if cluster.Spec.NodeSets != nil {
			nodeSets := make(map[string]NodeSetModel)
			for name, ns := range cluster.Spec.NodeSets {
				nodeSets[name] = NodeSetModel{
					HostClass: types.StringValue(ns.HostClass),
					Size:      types.Int32Value(ns.Size),
				}
			}
			nodeSetsValue, d := types.MapValueFrom(ctx, types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"host_class": types.StringType,
					"size":       types.Int32Type,
				},
			}, nodeSets)
			diags.Append(d...)
			model.NodeSets = nodeSetsValue
		}
	}

	if cluster.Status != nil {
		model.State = types.StringValue(cluster.Status.State.String())
		model.ApiURL = types.StringValue(cluster.Status.ApiUrl)
		model.ConsoleURL = types.StringValue(cluster.Status.ConsoleUrl)
	}
}

// NodeSetModel represents a node set in Terraform state
type NodeSetModel struct {
	HostClass types.String `tfsdk:"host_class"`
	Size      types.Int32  `tfsdk:"size"`
}
