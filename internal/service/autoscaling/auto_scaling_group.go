package autoscaling

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vautoscaling"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"

	"github.com/terraform-providers/terraform-provider-ncloud/internal/common"
	"github.com/terraform-providers/terraform-provider-ncloud/internal/conn"
	"github.com/terraform-providers/terraform-provider-ncloud/internal/framework"
	"github.com/terraform-providers/terraform-provider-ncloud/internal/service/vpc"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &autoScalingGroupResource{}
	_ resource.ResourceWithConfigure   = &autoScalingGroupResource{}
	_ resource.ResourceWithImportState = &autoScalingGroupResource{}
)

func NewAutoScalingGroupResource() resource.Resource {
	return &autoScalingGroupResource{}
}

type autoScalingGroupResource struct {
	config *conn.ProviderConfig
}

func (r *autoScalingGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *autoScalingGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*conn.ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.config = config
}

func (r *autoScalingGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auto_scaling_group"
}

func (r *autoScalingGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": framework.IDAttribute(),
			"access_control_group_no_list": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Validators: []validators.List{
					listvalidator.SizeBetween(1, 3),
				},
			},
			"auto_scaling_group_list": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"server_instance_no": schema.StringAttribute{
							Computed: true,
						},
						"health_status": schema.StringAttribute{
							Computed: true,
						},
						"lifecycle_state": schema.StringAttribute{
							Computed: true,
						},
					},
					Computed: true,
				},
			},
			"suspended_process_list": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attribute: map[string]schema.Attribute{
						"process": schema.StringAttribute{
							Computed: true,
						},
						"suspension_reason": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
			"target_group_no_list": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    tryue,
			},
			"vpc_no": schmea.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subnet_no": schmea.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"default_cool_down": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.Between(0, 2147483647),
				},
				Description: "default: 300s",
			},
			"desired_capacity": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.Between(0, 30),
				},
			},
			"health_check_grace_period": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.Between(0, 2147483647),
				},
				Description: "default: 300s",
			},
			"health_check_type_code": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"launch_configuration_no": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"max_size": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.Between(0, 30),
				},
			},
			"min_size": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.Between(0, 30),
				},
			},
			"wait_for_capacity_timeout": {
				Optional: true,
				Default:  stringdefault.StaticString("10m"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z]+[a-z0-9-]+[a-z0-9]$`),
						"Composed of lowercase alphabets, numbers, hyphen (-). Must start with an alphabetic character, and the last character can only be an English letter or number.",
					),
				},
			},
			"server_name_prefix": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(3, 7),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z]+[a-z0-9-]+[a-z0-9]$`),
						"Composed of lowercase alphabets, numbers, hyphen (-). Must start with an alphabetic character, and the last character can only be an English letter or number.",
					),
				},
			},
			"target_group_no_list": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *autoScalingGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan autoScalingGroupResourceModel

	// if !r.config.SupportVPC {
	// 	resp.Diagnostics.AddError(
	// 		"NOT SUPPORT CLASSIC",
	// 		"resource does not support CLASSIC. only VPC.",
	// 	)
	// 	return
	// }

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subnet, err := vpc.GetSubnetInstance(r.config, plan.SubnetNo.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"CREATING ERROR",
			err.Error(),
		)
	}

	reqParams := &vautoscaling.CreateAutoScalingGroupRequest{
		RegionCode: &r.config.RegionCode,
		VpcNo:      subnet.VpcNo,
		SubnetNo:   subnet.SubnetNo,
		MinSize:    ncloud.Int32(int32(plan.MinSize.ValueInt64())),
		MaxSize:    ncloud.Int32(int32(plan.MaxSize.ValueInt64())),
	}

	if !plan.ServerNamePrefix.IsNull() {
		reqParams.ServerNamePrefix = plan.ServerNamePrefix.ValueStringPointer()
	}

	plan.VpcNo = types.StringPointerValue(subnet.VpcNo)

	if !plan.Name.IsNull() {
		reqParams.AutoScalingGroupName = plan.Name.ValueStringPointer()
	}

	if !plan.DesiredCapacity.IsNull() && !plan.DesiredCapacity.IsUnknown() {
		reqParams.DesiredCapacity = ncloud.Int32(int32(plan.DesiredCapacity.ValueInt64()))
	}

	if !plan.DefaultCoolDown.IsNull() && !plan.DefaultCoolDown.IsUnknown() {
		reqParams.DefaultCoolDown = ncloud.Int32(int32(plan.DefaultCoolDown.ValueInt64()))
	}

	if !plan.HealthCheckGracePeriod.IsNull() && !plan.HealthCheckGracePeriod.IsUnknown() {
		reqParams.HealthCheckGracePeriod = ncloud.Int32(int32(plan.HealthCheckGracePeriod.ValueInt64()))
	}

	if !plan.HealthCheckTypeCode.IsNull() {
		reqParams.HealthCheckTypeCode = plan.HealthCheckTypeCode.ValueStringPointer()
	}

	tflog.Info(ctx, "CreateAutoScalingGroup reqParams="+common.MarshalUncheckedString(reqParams))

	response, err := r.config.Client.Vautoscaling.V2Api.CreateAutoScalingGroup(reqParams)
	if err != nil {
		resp.Diagnostics.AddError("CREATING ERROR", err.Error())
		return
	}
	tflog.Info(ctx, "CreateAutoSCalingGroup response="+common.MarshalUncheckedString(response))

	if response == nil || len(response.AutoScalingGroupList) < 1 {
		resp.Diagnostics.AddError("CREATING ERROR", "response invalid")
		return
	}

	plan.ID = types.StringPointerValue(ctx, r.config, *autoscaling.AutoScalingGroupNo)
	if err != nil {
		resp.Diagnostics.AddError("WAITNG FOR CREATION ERROR", err.Error())
		return
	}

	autoScalingGroup := response.AutoScalingGroupList[0]

	wait, err := time.ParseDuration(r.Get("wait_for_capacity_timeout").(string))
	if err != nil {
		return err
	}

	if wait == 0 {
		return nil
	}

	output, err := waitForAutoScalingGroupCapacity(ctx, r.config, *autoScalingGroup.AutoScalingGroupNo, wait)
	if err != nil {
		resp.Diagnostics.AddError("WAITING FOR CREATION ERROR", err.Error())
		return
	}

	plan.refreshFromOutput(ctx, output)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *autoScalingGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state autoScalingGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := GetAutoScalingGroup(ctx, r.config, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("READING ERROR", err.Error())
		return
	}

	if output == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.refreshFromOutput(ctx, output)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *autoScalingGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state autoScalingGroupResourceModel

	// if !r.config.SupportVPC {
	// 	resp.Diagnostics.AddError(
	// 		"NOT SUPPORT CLASSIC",
	// 		"resource does not support CLASSIC. only VPC.",
	// 	)
	// 	return
	// }

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	reqParams := &vautoscaling.UpdateAutoScalingGroupRequest{
		AutoScalingGroupNo: asg.AutoScalingGroupNo,
	}

	if !plan.MinSize.Equal(state.MinSize) {
		reqParams.MinSize = ncloud.Int32(int32(plan.MinSize.ValueInt64()))
	}

	if !plan.MaxSize.Equal(state.MaxSize) {
		reqParams.MaxSize = ncloud.Int32(int32(plan.MaxSize.ValueInt64()))
	}

	if !plan.LaunchConfigurationNo.Equal(state.LaunchConfigurationNo) {
		reqParams.LaunchConfigurationNo = plan.LaunchConfigurationNo.ValueStringPointer()
	}

	if !plan.DesiredCapacity.Equal(state.DesiredCapacity) {
		reqParams.DesiredCapacity = ncloud.Int32(int32(plan.DesiredCapacity.ValueInt64()))
	}

	if !plan.DefaultCoolDown.Equal(state.DefaultCoolDown) {
		reqParams.DefaultCoolDown = ncloud.Int32(int32(plan.DefaultCoolDown.ValueInt64()))
	}

	if !plan.HealthCheckGracePeriod.Equal(state.HealthCheckGracePeriod) {
		reqParams.HealthCheckGracePeriod = ncloud.Int32(int32(plan.HealthCheckGracePeriod.ValueInt64()))
	}

	if !plan.HealthCheckTypeCode.Equal(state.HealthCheckTypeCode) {
		reqParams.HealthCheckTypeCode = plan.HealthCheckTypeCode.ValueStringPointer()
	}

	if !plan.ServerNamePrefix.Equal(state.ServerNamePrefix) {
		reqParams.ServerNamePrefix = plan.ServerNamePrefix.ValueStringPointer()
	}

	tflog.Info(ctx, "UpdateAutoScalingGroup reqParams="+common.MarshalUncheckedString(reqParams))

	response, err := n.config.Client.Vpc.V2Api.UpdateAutoScalingGroup(reqParams)
	if err != nil {
		resp.Diagnostics.AddError("UPDATE ERROR", err.Error())
		return
	}
	tflog.Info(ctx, "UpdateAutoScalingGroup response="+common.MarshalUncheckedString(response))

	wait, err := time.ParseDuration(r.Get("wait_for_capacity_timeout").(string))
	if err != nil {
		return err
	}

	if wait == 0 {
		return nil
	}

	output, err := waitForAutoScalingGroupCapacity(ctx, r.config, *autoScalingGroup.AutoScalingGroupNo, wait)
	if err != nil {
		resp.Diagnostics.AddError("WAITING FOR UPDATE ERROR", err.Error())
		return
	}

	state.refreshFromOutput(output)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *autoScalingGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state autoScalingGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqParams := &autoscaling.UpdateAutoScalingGroupRequest{
		AutoScalingGroupName: asg.AutoScalingGroupName,
		DesiredCapacity:      ncloud.Int32(0),
		MinSize:              ncloud.Int32(0),
		MaxSize:              ncloud.Int32(0),
	}
	tflog.Info(ctx, "DeleteAutoScalingGroup reqParams="+common.MarshalUncheckedString(reqParams))

	response, err := r.config.Client.Vautoscaling.V2Api.UpdateAutoScalingGroup(reqParams)
	if err != nil {
		resp.Diagnostics.AddError("DELETING ERROR", err.Error())
		return
	}
	tflog.Info(ctx, "DeleteAutoScalingGroup response="+common.MarshalUncheckedString(response))

	if err := waitForInAutoScalingGroupServerInstanceListDeletion(ctx, r.config, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("WAITING FOR DELETE ERROR", err.Error())
	}

	if err := waitForAutoScalingGroupDeletion(ctx, r.config, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("WAITING FOR DELETE ERROR", err.Error())
	}
}

func GetAutoScalingGroup(ctx context.Context, config *conn.ProviderConfig, no string) (*vautoscaling.AutoScalingGroup, error) {
	reqParams := &vautoscaling.GetAutoScalingGroupListRequest{
		RegionCode:             &config.RegionCode,
		AutoScalingGroupNoList: []*string{ncloud.String(no)},
	}
	tflog.Info(ctx, "GetAutoScalingGroupDetail reqParams="+common.MarshalUncheckedString(reqParams))

	resp, err := config.Client.Vautoscaling.V2Api.GetAutoScalingGroupList(reqParams)
	if err != nil {
		return nil, err
	}
	tflog.Info(ctx, "GetAutoScalingGroupDetail response="+common.MarshalUncheckedString(resp))

	if resp == nil || len(resp.AutoScalingGroupList) < 1 {
		return nil, nil
	}

	return resp.AutoScalingGroupList[0], nil
}

func waitForInAutoScalingGroupServerInstanceListDeletion(ctx context.Context, config *conn.ProviderConfig, id string) error {
	stateConf := &retry.StateChangeConf{
		Pending: []string{"INSVC"},
		Target:  []string{"TERMT"},
		Refresh: func() (interface{}, string, error) {
			asg, err := GetAutoScalingGroup(ctx, config, id)
			if err != nil {
				return 0, "", err
			}
			if len(asg.InAutoScalingGroupServerInstanceList) > 0 {
				return asg, "INSVC", nil
			} else {
				return asg, "TERMT", nil
			}
		},
		Delay:      2 * time.Second,
		MinTimeout: 3 * time.Second,
		Timeout:    conn.DefaultStopTimeout * 3,
	}

	if _, err := stateConf.WaitForStateContext(ctx); err != nil {
		return fmt.Errorf("Error waiting for InAutoScalingGroupServerInstanceList (%s) to become deleting: %s", id, err)
	}
	return nil
}

func waitForAutoScalingGroupDeletion(ctx context.Context, config *conn.ProviderConfig, id string) error {
	stateConf := &retry.StateChangeConf{
		Pending: []string{"RUN"},
		Target:  []string{"DELETE"},
		Refresh: func() (interface{}, string, error) {
			client := config.Client
			reqParams := &vautoscaling.DeleteAutoScalingGroupRequest{
				AutoScalingGroupNo: ncloud.String(id),
			}
			resp, err := client.Vautoscaling.V2Api.DeleteAutoScalingGroup(reqParams)
			if err != nil {
				errBody, _ := GetCommonErrorBody(err)
				if errBody.ReturnCode == ApiErrorASGIsUsingPolicyOrLaunchConfigurationOnVpc {
					return resp, "RUN", nil
				} else {
					return 0, "", err
				}
			} else {
				return resp, "DELETE", nil
			}
		},
		Delay:      2 * time.Second,
		MinTimeout: 3 * time.Second,
		Timeout:    conn.DefaultTimeout,
	}

	if _, err := stateConf.WaitForStateContext(ctx); err != nil {
		return fmt.Errorf("Error waiting for AutoScalingGroup (%s) to become deleting: %s", id, err)
	}
	return nil
}

func waitForAutoScalingGroupCapacity(ctx context.Context, config *conn.ProviderConfig, id string, wait time.Duration) (*vautoscaling.AutoScalingGroup, error) {
	var asg *vautoscaling.AutoScalingGroup
	stateConf := &retry.StateChangeConf{
		Pending: []string{"pending", "waiting"},
		Target:  []string{"ready"},
		Refresh: func() (interface{}, string, error) {
			// Auto Scaling Group 가져오기
			group, err := getVpcAutoScalingGroup(config, id)
			asg = group
			if err != nil {
				return nil, "", err
			}

			// 서버 인스턴스 리스트 가져오기
			asgServerInstanceList, err := getVpcInAutoScalingGroupServerInstanceList(config, id)
			if err != nil {
				return nil, "", err
			}

			var currentServerInstanceCnt int32
			for _, i := range asgServerInstanceList {
				if !strings.EqualFold(*i.HealthStatus, "HLTHY") || !strings.EqualFold(*i.LifecycleState, "INSVC") {
					continue
				}
				currentServerInstanceCnt++
			}

			minASG := asg.MinSize
			if asg.DesiredCapacity != nil {
				minASG = asg.DesiredCapacity
			}

			if currentServerInstanceCnt < *minASG {
				return group, "waiting", nil
			}

			return group, "ready", nil
		},
		Timeout:    wait,
		Delay:      2 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	_, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error waiting for AutoScalingGroup capacity: %s", err)
	}

	return asg, nil
}

type autoScalingGroupResourceModel struct {
	ID                                   types.String `tfsdk:"id"`
	VpcNo                                types.String `tfsdk:"vpc_no"`
	SubnetNo                             types.String `tfsdk:"subnet_no"`
	AccessControlGroupNoList             types.List   `tfsdk:"access_control_group_no_list"`
	AutoScalingGroupList                 types.List   `tfsdk:"auto_scaling_group_list"`
	DefaultCoolDown                      types.Int64  `tfsdk:"default_cool_down"`
	DesiredCapacity                      types.Int64  `tfsdk:"desired_capacity"`
	HealthCheckGracePeriod               types.Int64  `tfsdk:"health_check_grace_period"`
	HealthCheckTypeCode                  types.Int64  `tfsdk:"health_check_type_code"`
	LaunchConfigurationNo                types.String `tfsdk:"launch_configuration_no"`
	MaxSize                              types.Int64  `tfsdk:"max_size"`
	MinSize                              types.Int64  `tfsdk:"min_size"`
	Name                                 types.String `tfsdk:"name"`
	ServerNamePrefix                     types.String `tfsdk:"server_name_prefix"`
	TargetGroupNoList                    types.List   `tfsdk:"target_group_no_list"`
	AccessControlGroupNoList             types.List   `tfsdk:"access_control_group_no_list"`
	InAutoScalingGroupServerInstanceList types.List   `tfsdk:"in_auto_scaling_group_server_instance_list"`
	SuspendedProcessList                 types.List   `tfsdk:"suspended_process_list"`
}

type autoScalingGroupServer struct {
	ServerInstanceNo types.String `tfsdk:"server_instance_no"`
	HealthStatus     types.String `tfsdk:"health_status"`
	LifecycleState   types.String `tfsdk:"lifecycle_state"`
}

type suspendedProcess struct {
	Process          types.String `tfsdk:"process"`
	SuspensionReason types.String `tfsdk:"suspension_reason`
}

func (r autoScalingGroup) autoScalingGroupAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"server_instance_no": types.StringType,
		"health_status":      types.StringType,
		"lifecycle_state":    types.StringType,
	}
}

func (r autoScalingGroup) suspendedProcessAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"process":         types.StringType,
		"lifecycle_state": types.StringType,
	}
}

func (r *autoScalingGroupResourceModel) refreshFromOutput(ctx context.Context, output *autoScalingGroup.AutoScalingGroup) {
	r.ID = types.StringPointerValue(output.AutoScalingGroupNo)
	r.Name = types.StringPointerValue(output.AutoScalingGroupName)
	r.ServerNamePrefix = types.StringPointerValue(output.ServerNAmePrefix)
	r.LaunchConfigurationNo = types.StringPointerValue(output.LaunchConfigurationNo)
	r.MinSize = common.Int64ValueFromInt32(output.MinSize)
	r.MaxSize = common.Int64ValueFromInt32(output.MaxSize)
	r.DesiredCapacity = common.Int64ValueFromInt32(output.DesiredCapacity)
	r.DefaultCoolDown = common.Int64ValueFromInt32(output.DefaultCoolDown)
	r.HealthCheckGracePeriod = common.Int64ValueFromInt32(output.HealthCheckGracePeriod)
	r.HealthCheckTypeCode = common.Int64ValueFromInt32(output.HealthCheckType)

	acgList, _ := types.ListValueFrom(ctx, types.StringType, output.AccessControlGroupNoList)
	r.AccessControlGroup = acgList
	targetGroupList, _ := types.ListValueFrom(ctx, types.StringType, output.TargetGroupNoList)
	r.TargetGroupNoList = targetGroupList

	var autoScalingGroupList []autoScalingGroupServer
	for _, server := range output.InAutoScalingGroupServerInstanceList {
		autoScalingGroupServerInstance := autoScalingGroupServer{
			ServerInstanceNo: types.StringPointerValue(server.ServerInstanceNo),
			HealthStatus:     types.StringPointerValue(server.HealthStatus),
			LifecycleState:   types.StringPointerValue(server.LifecycleState),
		}

		autoScalingGroupList = append(autoScalingGroupList, autoScalingGroupServerInstance)
	}
	autoScalingGroups, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: autoScalingGroupServer{}.autoScalingGroupAttrTypes()}, autoScalingGroupList)
	r.InAutoScalingGroupServerInstanceList = autoScalingGroups

	var suspendedProcessList []suspendedProcess
	for _, process := range output.SuspendedProcessList {
		suspended := suspendedProcess{
			Process:        types.StringPointerValue(process.Process),
			LifecycleState: types.StringPointerValue(process.LifecycleState),
		}

		suspendedProcessList = append(suspendedProcessList, suspended)
	}
	suspendedProcesses, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: suspendedProcess{}.attrTypes{}}, suspendedProcessList)
	r.SuspendedProcessList = suspendedProcesses
}
