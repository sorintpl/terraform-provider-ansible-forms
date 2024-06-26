package provider

import (
	"context"
	"fmt"
	"html"
	"terraform-provider-ansible-forms/internal/interfaces"
	"terraform-provider-ansible-forms/internal/utils"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/microcosm-cc/bluemonday"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &JobResource{}
	_ resource.ResourceWithConfigure = &JobResource{}
)

// NewJobResource is a helper function to simplify the provider implementation.
func NewJobResource() resource.Resource {
	return &JobResource{
		config: resourceOrDataSourceConfig{
			name: "job_resource",
		},
	}
}

// JobResource is the resource implementation.
type JobResource struct {
	config resourceOrDataSourceConfig
}

// JobResourceModel maps the resource schema data.
type JobResourceModel struct {
	CxProfileName types.String `tfsdk:"cx_profile_name"`
	ID            types.Int64  `tfsdk:"id"`
	LastUpdated   types.String `tfsdk:"last_updated"`
	FormName      types.String `tfsdk:"form_name"`
	Status        types.String `tfsdk:"status"`
	Extravars     types.Map    `tfsdk:"extravars"`
	Credentials   types.Map    `tfsdk:"credentials"`
	Target        types.String `tfsdk:"target"`
	Output        types.String `tfsdk:"output"`
	Start         types.String `tfsdk:"start"`
	End           types.String `tfsdk:"end"`
	Approval      types.String `tfsdk:"approval"`
	State         types.String `tfsdk:"state"`
	Message       types.String `tfsdk:"message"`
	Error         types.String `tfsdk:"error"`
}

// Metadata returns the resource type name.
func (r *JobResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.config.name
}

// Schema defines the schema for the resource.
func (r *JobResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Job resource",

		Attributes: map[string]schema.Attribute{
			"cx_profile_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Connection profile name.",
			},
			"form_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Form name of a job.",
			},
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "ID of a job.",
			},
			"extravars": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Extra vars of a job.",
			},
			"credentials": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Credentials of a job.",
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Last update time of a job.",
			},
			"status": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Status of a job.",
			},
			"target": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Target form of a job.",
			},
			"output": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Output of a job.",
			},
			"start": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Start time of a job.",
			},
			"end": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "End time of a job.",
			},
			"approval": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Approval of a job.",
			},
			"state": schema.StringAttribute{
				Description: "State.",
				Computed:    true,
				Optional:    true,
				Default:     stringdefault.StaticString("present"),
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"present", "absent"}...),
				},
			},
			"message": schema.StringAttribute{
				Description: "Message of a job.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed: true,
			},
			"error": schema.StringAttribute{
				Description: "Error of a job.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *JobResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}
	config, ok := req.ProviderData.(Config)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected Config, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
	}
	r.config.providerConfig = config
}

// Create a new resource.
func (r *JobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *JobResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	var request interfaces.JobResourceModel
	errorHandler := utils.NewErrorHandler(ctx, &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		tflog.Debug(ctx, "error getting req plan")
		return
	}

	client, err := getRestClient(errorHandler, r.config, data.CxProfileName)
	if err != nil {
		// error reporting done inside NewClient
		return
	}

	var extravars = make(map[string]interface{})
	for k, v := range data.Extravars.Elements() {
		extravars[k] = v
	}

	var credentials = make(map[string]interface{})
	for k, v := range data.Credentials.Elements() {
		credentials[k] = v
	}

	extravars["state"] = data.State.ValueString()

	request.Extravars = extravars
	request.Credentials = credentials

	if data.Credentials.IsNull() {
		request.Credentials = nil
	}

	request.CxProfileName = data.CxProfileName.ValueString()
	request.Form = data.FormName.ValueString()
	request.State = data.State.ValueString()

	job, err := interfaces.CreateJob(errorHandler, *client, request)
	if err != nil {
		tflog.Debug(ctx, "err creating a resource", map[string]interface{}{"err": err})
		return
	}

	data.ID = types.Int64Value(job.Data.ID)
	data.Start = types.StringValue(job.Data.Start)
	data.End = types.StringValue(job.Data.End)
	data.Status = types.StringValue(job.Data.Status)
	data.LastUpdated = types.StringValue(time.Now().UTC().Format(time.RFC3339))
	data.Target = types.StringValue(job.Data.Target)
	data.Output = types.StringValue(html.UnescapeString(bluemonday.StrictPolicy().Sanitize(job.Data.Output)))
	data.Approval = types.StringValue(fmt.Sprintf("%s", job.Data.Approval))
	data.Message = types.StringValue(job.Message)
	data.Error = types.StringValue(job.Data.Error)

	tflog.Debug(ctx, "JOB ID", map[string]interface{}{"ID": job.Data.ID, "DATA": data})

	tflog.Trace(ctx, "created a resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read resource information.
func (r *JobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *JobResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	errorHandler := utils.NewErrorHandler(ctx, &resp.Diagnostics)

	client, err := getRestClient(errorHandler, r.config, data.CxProfileName)
	if err != nil {
		// error reporting done inside NewClient
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("read a job resource: %#v", data))

	var job *interfaces.JobGetDataSourceModel
	if data.ID.ValueInt64() != 0 {
		job, err = interfaces.GetJobByID(errorHandler, *client, data.ID.ValueInt64())
	} else {
		return
	}
	if err != nil {
		return
	}

	if job == nil {
		return
	}

	data.ID = types.Int64Value(job.ID)

	if job.Form != "" {
		data.FormName = types.StringValue(job.Form)
	}
	if job.Status != "" {
		data.Status = types.StringValue(job.Status)
	}
	//data.Extravars = jsonStringToMapValue(ctx, &resp.Diagnostics, restInfo.JobGetDataSourceModel.Extravars)
	//data.Credentials = jsonStringToMapValue(ctx, &resp.Diagnostics, restInfo.JobGetDataSourceModel.Credentials)
	if job.Output != "" {
		data.Output = types.StringValue(html.UnescapeString(bluemonday.StrictPolicy().Sanitize(job.Output)))
	}
	if job.Target != "" {
		data.Target = types.StringValue(job.Target)
	}
	if job.Start != "" {
		data.Start = types.StringValue(job.Start)
	}
	if job.End != "" {
		data.End = types.StringValue(job.End)
	}
	if job.Approval != nil {
		data.Approval = types.StringValue(fmt.Sprintf("%s", job.Approval))
	}

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Debug(ctx, fmt.Sprintf("read a data source: %#v", data))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *JobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *JobResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	var request interfaces.JobResourceModel
	errorHandler := utils.NewErrorHandler(ctx, &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		tflog.Debug(ctx, "error getting req plan")
		return
	}

	client, err := getRestClient(errorHandler, r.config, data.CxProfileName)
	if err != nil {
		// error reporting done inside NewClient
		return
	}

	var extravars = make(map[string]interface{})
	for k, v := range data.Extravars.Elements() {
		extravars[k] = v
	}

	var credentials = make(map[string]interface{})
	for k, v := range data.Credentials.Elements() {
		credentials[k] = v
	}

	extravars["state"] = data.State.ValueString()

	request.Extravars = extravars
	request.Credentials = credentials

	if data.Credentials.IsNull() {
		request.Credentials = nil
	}

	request.Form = data.FormName.ValueString()
	request.State = data.State.ValueString()

	job, err := interfaces.CreateJob(errorHandler, *client, request)
	if err != nil {
		tflog.Debug(ctx, "err creating/updating a resource", map[string]interface{}{"err": err})
		return
	}
	data.ID = basetypes.NewInt64Value(job.Data.ID)

	elements := map[string]attr.Value{}

	for key, value := range job.Data.Extravars {
		elements[key] = types.StringValue(fmt.Sprintf("%s", value))
	}

	data.Error = types.StringValue(job.Data.Error)

	tflog.Debug(ctx, "JOB ID", map[string]interface{}{"ID": job.Data.ID, "DATA": data})

	tflog.Trace(ctx, "update/create a resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *JobResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *JobResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	errorHandler := utils.NewErrorHandler(ctx, &resp.Diagnostics)
	if data.ID.IsNull() {
		err := errorHandler.MakeAndReportError("ID is null", "job ID is null")
		if err != nil {
			tflog.Debug(ctx, fmt.Sprintf("failed reporting err: %v", err))
			return
		}
		return
	}

	client, err := getRestClient(errorHandler, r.config, data.CxProfileName)
	if err != nil {
		// error reporting done inside NewClient
		return
	}

	var extravars = make(map[string]interface{})
	for k, v := range data.Extravars.Elements() {
		extravars[k] = v
	}

	var credentials = make(map[string]interface{})
	for k, v := range data.Credentials.Elements() {
		credentials[k] = v
	}

	extravars["state"] = "absent"

	var request interfaces.JobResourceModel
	request.Extravars = extravars
	request.Credentials = credentials

	if data.Credentials.IsNull() {
		request.Credentials = nil
	}

	request.CxProfileName = data.CxProfileName.ValueString()
	request.Form = data.FormName.ValueString()
	request.State = "absent"

	_, err = interfaces.CreateJob(errorHandler, *client, request)
	if err != nil {
		tflog.Debug(ctx, "err delete a resource", map[string]interface{}{"err": err})
		return
	}
}
