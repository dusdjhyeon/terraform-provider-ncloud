package mysql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vmysql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/terraform-providers/terraform-provider-ncloud/internal/common"
	"github.com/terraform-providers/terraform-provider-ncloud/internal/conn"
)

var (
	_ datasource.DataSource              = &mysqlUsersDataSource{}
	_ datasource.DataSourceWithConfigure = &mysqlUsersDataSource{}
)

func NewMysqlUsersDataSource() datasource.DataSource {
	return &mysqlUsersDataSource{}
}

type mysqlUsersDataSource struct {
	config *conn.ProviderConfig
}

func (d *mysqlUsersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mysql_users"
}

func (d *mysqlUsersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*conn.ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.config = config
}

func (d *mysqlUsersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRelative().AtParent().AtName("mysql_instance_no"),
					),
				},
			},
			"mysql_instance_no": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRelative().AtParent().AtName("id"),
					),
				},
			},
			"output_file": schema.StringAttribute{
				Optional: true,
			},
			"mysql_user_list": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed: true,
						},
						"host_ip": schema.StringAttribute{
							Computed: true,
						},
						"authority": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
		Blocks: map[string]schema.Block{
			"filter": common.DataSourceFiltersBlock(),
		},
	}
}

func (d *mysqlUsersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data mysqlUsersDataSourceModel
	var mysqlId string

	if !d.config.SupportVPC {
		resp.Diagnostics.AddError(
			"NOT SUPPORT CLASSIC",
			"does not support CLASSIC. only VPC.",
		)
		return
	}

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		mysqlId = data.ID.ValueString()
		data.MysqlInstanceNo = data.ID
	}

	if !data.MysqlInstanceNo.IsNull() && !data.MysqlInstanceNo.IsUnknown() {
		mysqlId = data.MysqlInstanceNo.ValueString()
		data.ID = data.MysqlInstanceNo
	}

	output, err := GetMysqlUserAllList(ctx, d.config, mysqlId)
	if err != nil {
		resp.Diagnostics.AddError("READING ERROR", err.Error())
		return
	}

	if output == nil {
		resp.Diagnostics.AddError("READING ERROR", "no result. please change search criteria and try again.")
		return
	}

	mysqlUserList := flattenMysqlUsers(output)
	fillteredList := common.FilterModels(ctx, data.Filters, mysqlUserList)
	if err = data.refreshFromOutput(ctx, fillteredList, data.MysqlInstanceNo.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error while getting output values of mysql users",
			err.Error(),
		)
		return
	}

	if !data.OutputFile.IsNull() && data.OutputFile.String() != "" {
		outputPath := data.OutputFile.ValueString()

		if convertedList, err := convertUsersToJsonStruct(data.MysqlUserList.Elements()); err != nil {
			resp.Diagnostics.AddError("OUTPUT FILE ERROR", err.Error())
			return
		} else if err := common.WriteToFile(outputPath, convertedList); err != nil {
			resp.Diagnostics.AddError("OUTPUT FILE ERROR", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func GetMysqlUserAllList(ctx context.Context, config *conn.ProviderConfig, id string) ([]*vmysql.CloudMysqlUser, error) {
	var allUsers []*vmysql.CloudMysqlUser
	pageNo := int32(0)
	pageSize := int32(100)
	hasMore := true

	for hasMore {
		reqParams := &vmysql.GetCloudMysqlUserListRequest{
			RegionCode:           &config.RegionCode,
			CloudMysqlInstanceNo: ncloud.String(id),
			PageNo:               ncloud.Int32(pageNo),
			PageSize:             ncloud.Int32(pageSize),
		}
		tflog.Info(ctx, "GetMysqlUserList reqParams="+common.MarshalUncheckedString(reqParams))

		resp, err := config.Client.Vmysql.V2Api.GetCloudMysqlUserList(reqParams)
		if err != nil {
			return nil, err
		}

		if resp == nil {
			break
		}

		allUsers = append(allUsers, resp.CloudMysqlUserList...)

		hasMore = len(resp.CloudMysqlUserList) == int(pageSize)
		pageNo++
	}

	if len(allUsers) == 0 {
		return nil, nil
	}

	allUsers = reverseAndExcludeFirst(allUsers)

	tflog.Info(ctx, "GetMysqlUserList response="+common.MarshalUncheckedString(allUsers))

	return allUsers, nil
}

type mysqlUsersDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	MysqlInstanceNo types.String `tfsdk:"mysql_instance_no"`
	MysqlUserList   types.List   `tfsdk:"mysql_user_list"`
	OutputFile      types.String `tfsdk:"output_file"`
	Filters         types.Set    `tfsdk:"filter"`
}

type mysqlUser struct {
	UserName  types.String `tfsdk:"name"`
	HostIp    types.String `tfsdk:"host_ip"`
	Authority types.String `tfsdk:"authority"`
}

type mysqlUserToJsonConvert struct {
	UserName  string `json:"name"`
	HostIp    string `json:"host_ip"`
	Authority string `json:"authority"`
}

func (d mysqlUser) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":      types.StringType,
		"host_ip":   types.StringType,
		"authority": types.StringType,
	}
}

func convertUsersToJsonStruct(users []attr.Value) ([]mysqlUserToJsonConvert, error) {
	var userToConvert = []mysqlUserToJsonConvert{}

	for _, user := range users {
		userJson := mysqlUserToJsonConvert{}
		if err := json.Unmarshal([]byte(user.String()), &userJson); err != nil {
			return nil, err
		}
		userToConvert = append(userToConvert, userJson)
	}

	return userToConvert, nil
}

func flattenMysqlUsers(list []*vmysql.CloudMysqlUser) []*mysqlUser {
	var outputs []*mysqlUser

	for _, v := range list {
		var output mysqlUser
		output.refreshFromOutput(v)

		outputs = append(outputs, &output)
	}
	return outputs
}

func (d *mysqlUsersDataSourceModel) refreshFromOutput(ctx context.Context, output []*mysqlUser, instance string) error {
	d.ID = types.StringValue(instance)
	d.MysqlInstanceNo = types.StringValue(instance)
	userListValue, err := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: mysqlUser{}.attrTypes()}, output)
	if err != nil {
		return fmt.Errorf("error creating ListValue for userList: %s", err)
	}
	d.MysqlUserList = userListValue
	return nil
}

func (d *mysqlUser) refreshFromOutput(output *vmysql.CloudMysqlUser) {
	d.UserName = types.StringPointerValue(output.UserName)
	d.HostIp = types.StringPointerValue(output.HostIp)
	d.Authority = types.StringPointerValue(output.Authority)
}

func reverseAndExcludeFirst(users []*vmysql.CloudMysqlUser) []*vmysql.CloudMysqlUser {
	if len(users) <= 1 {
		return users
	}

	for i, j := 0, len(users)-1; i < j; i, j = i+1, j-1 {
		users[i], users[j] = users[j], users[i]
	}

	return users[1:]
}
