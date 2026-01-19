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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	fulfillmentv1 "github.com/innabox/fulfillment-common/api/fulfillment/v1"
	sharedv1 "github.com/innabox/fulfillment-common/api/shared/v1"

	"github.com/innabox/terraform-provider-osac/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ComputeInstanceResource{}
var _ resource.ResourceWithImportState = &ComputeInstanceResource{}

func NewComputeInstanceResource() resource.Resource {
	return &ComputeInstanceResource{}
}

// ComputeInstanceResource defines the resource implementation.
type ComputeInstanceResource struct {
	client fulfillmentv1.ComputeInstancesClient
}

// ComputeInstanceResourceModel describes the resource data model.
type ComputeInstanceResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Template           types.String `tfsdk:"template"`
	TemplateParameters types.Map    `tfsdk:"template_parameters"`
	// Computed status fields
	State     types.String `tfsdk:"state"`
	IPAddress types.String `tfsdk:"ip_address"`
}

func (r *ComputeInstanceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance"
}

func (r *ComputeInstanceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an OSAC compute instance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the compute instance.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Human-friendly name of the compute instance.",
				Optional:    true,
			},
			"template": schema.StringAttribute{
				Description: "Reference to the compute instance template ID.",
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
			"state": schema.StringAttribute{
				Description: "Current state of the compute instance (PROGRESSING, READY, FAILED).",
				Computed:    true,
			},
			"ip_address": schema.StringAttribute{
				Description: "IP address of the compute instance.",
				Computed:    true,
			},
		},
	}
}

func (r *ComputeInstanceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = providerData.ComputeInstancesClient
}

func (r *ComputeInstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ComputeInstanceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert template parameters
	templateParams, err := convertTemplateParameters(ctx, data.TemplateParameters)
	if err != nil {
		resp.Diagnostics.AddError("Failed to convert template parameters", err.Error())
		return
	}

	// Build the compute instance spec
	spec := &fulfillmentv1.ComputeInstanceSpec{
		Template:           data.Template.ValueString(),
		TemplateParameters: templateParams,
	}

	// Build the compute instance
	instance := &fulfillmentv1.ComputeInstance{
		Spec: spec,
	}

	// Set metadata if name is provided
	if !data.Name.IsNull() {
		instance.Metadata = &sharedv1.Metadata{
			Name: data.Name.ValueString(),
		}
	}

	// Create the compute instance
	createResp, err := r.client.Create(ctx, &fulfillmentv1.ComputeInstancesCreateRequest{
		Object: instance,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create compute instance", err.Error())
		return
	}

	instanceID := createResp.Object.Id

	// Wait for instance to reach READY state
	result, err := WaitForReady(ctx, WaitForReadyConfig{
		PendingStates: []string{
			fulfillmentv1.ComputeInstanceState_COMPUTE_INSTANCE_STATE_UNSPECIFIED.String(),
			fulfillmentv1.ComputeInstanceState_COMPUTE_INSTANCE_STATE_PROGRESSING.String(),
		},
		TargetStates: []string{
			fulfillmentv1.ComputeInstanceState_COMPUTE_INSTANCE_STATE_READY.String(),
		},
		RefreshFunc: r.instanceStateRefreshFunc(ctx, instanceID),
		Timeout:     DefaultCreateTimeout,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error waiting for compute instance to be ready",
			fmt.Sprintf("Instance %s: %s", instanceID, err.Error()),
		)
		return
	}

	// Update state with the final instance data
	finalInstance := result.(*fulfillmentv1.ComputeInstance)
	r.updateModelFromComputeInstance(&data, finalInstance)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ComputeInstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ComputeInstanceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Get(ctx, &fulfillmentv1.ComputeInstancesGetRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read compute instance", err.Error())
		return
	}

	r.updateModelFromComputeInstance(&data, getResp.Object)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ComputeInstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ComputeInstanceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert template parameters
	templateParams, err := convertTemplateParameters(ctx, data.TemplateParameters)
	if err != nil {
		resp.Diagnostics.AddError("Failed to convert template parameters", err.Error())
		return
	}

	// Build the update request
	spec := &fulfillmentv1.ComputeInstanceSpec{
		Template:           data.Template.ValueString(),
		TemplateParameters: templateParams,
	}

	instance := &fulfillmentv1.ComputeInstance{
		Id:   data.ID.ValueString(),
		Spec: spec,
	}

	if !data.Name.IsNull() {
		instance.Metadata = &sharedv1.Metadata{
			Name: data.Name.ValueString(),
		}
	}

	updateResp, err := r.client.Update(ctx, &fulfillmentv1.ComputeInstancesUpdateRequest{
		Object: instance,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update compute instance", err.Error())
		return
	}

	instanceID := updateResp.Object.Id

	// Wait for instance to reach READY state
	result, err := WaitForReady(ctx, WaitForReadyConfig{
		PendingStates: []string{
			fulfillmentv1.ComputeInstanceState_COMPUTE_INSTANCE_STATE_UNSPECIFIED.String(),
			fulfillmentv1.ComputeInstanceState_COMPUTE_INSTANCE_STATE_PROGRESSING.String(),
		},
		TargetStates: []string{
			fulfillmentv1.ComputeInstanceState_COMPUTE_INSTANCE_STATE_READY.String(),
		},
		RefreshFunc: r.instanceStateRefreshFunc(ctx, instanceID),
		Timeout:     DefaultUpdateTimeout,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error waiting for compute instance to be ready after update",
			fmt.Sprintf("Instance %s: %s", instanceID, err.Error()),
		)
		return
	}

	// Update state with the final instance data
	finalInstance := result.(*fulfillmentv1.ComputeInstance)
	r.updateModelFromComputeInstance(&data, finalInstance)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ComputeInstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ComputeInstanceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, &fulfillmentv1.ComputeInstancesDeleteRequest{
		Id: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete compute instance", err.Error())
		return
	}
}

func (r *ComputeInstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// instanceStateRefreshFunc returns a StateRefreshFunc that fetches the instance and returns its state.
// This follows the AWS provider pattern for polling resource status.
func (r *ComputeInstanceResource) instanceStateRefreshFunc(ctx context.Context, instanceID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		getResp, err := r.client.Get(ctx, &fulfillmentv1.ComputeInstancesGetRequest{Id: instanceID})
		if err != nil {
			return nil, "", fmt.Errorf("failed to get compute instance: %w", err)
		}

		instance := getResp.Object
		if instance.Status == nil {
			// Status not yet available, return unspecified state to continue polling
			return instance, fulfillmentv1.ComputeInstanceState_COMPUTE_INSTANCE_STATE_UNSPECIFIED.String(), nil
		}

		state := instance.Status.State

		// If the instance has failed, return an error to stop polling
		if state == fulfillmentv1.ComputeInstanceState_COMPUTE_INSTANCE_STATE_FAILED {
			return nil, state.String(), fmt.Errorf("compute instance reached FAILED state")
		}

		return instance, state.String(), nil
	}
}

// convertTemplateParameters converts a Terraform map of strings to a protobuf map of Any values.
func convertTemplateParameters(ctx context.Context, tfMap types.Map) (map[string]*anypb.Any, error) {
	if tfMap.IsNull() || tfMap.IsUnknown() {
		return nil, nil
	}

	// Extract the map as Go strings
	stringMap := make(map[string]string)
	diags := tfMap.ElementsAs(ctx, &stringMap, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to extract template parameters")
	}

	// Convert each string to anypb.Any wrapping a StringValue
	result := make(map[string]*anypb.Any)
	for key, value := range stringMap {
		anyValue, err := anypb.New(wrapperspb.String(value))
		if err != nil {
			return nil, fmt.Errorf("could not convert parameter %q: %w", key, err)
		}
		result[key] = anyValue
	}

	return result, nil
}

func (r *ComputeInstanceResource) updateModelFromComputeInstance(model *ComputeInstanceResourceModel, instance *fulfillmentv1.ComputeInstance) {
	model.ID = types.StringValue(instance.Id)

	if instance.Metadata != nil {
		model.Name = types.StringValue(instance.Metadata.Name)
	}

	if instance.Spec != nil {
		model.Template = types.StringValue(instance.Spec.Template)
	}

	if instance.Status != nil {
		model.State = types.StringValue(instance.Status.State.String())
		model.IPAddress = types.StringValue(instance.Status.IpAddress)
	} else {
		model.State = types.StringNull()
		model.IPAddress = types.StringNull()
	}
}
