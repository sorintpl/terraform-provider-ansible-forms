package interfaces

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/mitchellh/mapstructure"

	"terraform-provider-ansible-forms/internal/restclient"
	"terraform-provider-ansible-forms/internal/utils"
)

// JobResourceModel describes the resource data model.
type JobResourceModel struct {
	ID            int64                  `mapstructure:"id"`
	Start         string                 `mapstructure:"start"`
	End           string                 `mapstructure:"end"`
	User          string                 `mapstructure:"user"`
	UserType      string                 `mapstructure:"user_type"`
	JobType       string                 `mapstructure:"job_type"`
	Extravars     map[string]interface{} `mapstructure:"extravars"`
	Credentials   map[string]interface{} `mapstructure:"credentials"`
	Form          string                 `mapstructure:"formName"`
	Status        string                 `mapstructure:"status"`
	Message       string                 `mapstructure:"message"`
	Target        string                 `mapstructure:"target"`
	NoOfRecords   int64                  `mapstructure:"no_of_records"`
	Counter       int64                  `mapstructure:"counter"`
	Output        string                 `mapstructure:"output"`
	Data          string                 `mapstructure:"data"`
	LastUpdated   string                 `mapstructure:"last_updated"`
	Approval      string                 `mapstructure:"approval"`
	State         string                 `mapstructure:"state"`
	CxProfileName string                 `mapstructure:"cx_profile_name"`
}

// ExtravarsBodyDataModel describes the data source of Protocols
type ExtravarsBodyDataModel struct {
	Accountid          string `mapstructure:"accountid"`
	ConfigStandard     string `mapstructure:"config_standard"`
	Dataclass          string `mapstructure:"dataclass"`
	Env                string `mapstructure:"env"`
	Exposure           string `mapstructure:"exposure"`
	Opco               string `mapstructure:"opco"`
	ProtectionRequired string `mapstructure:"protection_required"`
	Region             string `mapstructure:"region"`
	ShareName          string `mapstructure:"share_name"`
	Size               string `mapstructure:"size"`
	SvmName            string `mapstructure:"svm_name"`
	ExportPolicy       string `mapstructure:"export_policy"`
	VolumeComment      string `mapstructure:"volume_comment"`
}

// CredentialsDataModel describes data model
type CredentialsDataModel struct {
	CifsCred  string `mapstructure:"cifs_cred"`
	OntapCred string `mapstructure:"ontap_cred"`
}

// JobGetDataSourceModel ...
type JobGetDataSourceModel struct {
	ID          int64                  `mapstructure:"id"`
	Start       string                 `mapstructure:"start"`
	End         string                 `mapstructure:"end"`
	User        string                 `mapstructure:"user"`
	UserType    string                 `mapstructure:"user_type"`
	JobType     string                 `mapstructure:"job_type"`
	Extravars   map[string]interface{} `mapstructure:"extravars"`
	Credentials CredentialsDataModel   `mapstructure:"credentials"`
	Form        string                 `mapstructure:"formName"`
	Status      string                 `mapstructure:"status"`
	Target      string                 `mapstructure:"target"`
	Output      string                 `mapstructure:"output"`
	Data        string                 `mapstructure:"data"`
	Approval    map[string]interface{} `mapstructure:"approval"`
	State       string                 `mapstructure:"state"`
	Error       string                 `mapstructure:"error"`
}

// GetJobResponse describes GET job response.
type GetJobResponse struct {
	Status  string                `mapstructure:"status"`
	Message string                `mapstructure:"message"`
	Data    JobGetDataSourceModel `mapstructure:"data"`
}

// CreateJobResponse ...
type CreateJobResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Output struct {
			ID int64 `json:"id"`
		} `json:"output"`
		Error string `json:"error"`
	} `json:"data"`
}

// GetJobByID gets job info by id.
func GetJobByID(errorHandler *utils.ErrorHandler, r restclient.RestClient, id int64) (*JobGetDataSourceModel, error) {
	statusCode, response, err := r.GetNilOrOneRecord(fmt.Sprintf("job/%d", id), nil, nil)
	if err == nil && response["message"] == "failed to find job" {
		err = fmt.Errorf("no response for GET Job by ID")
	}
	if err != nil {
		return nil, errorHandler.MakeAndReportError("error reading job info", fmt.Sprintf("error on GET job/: %s, statusCode %d", err, statusCode))
	}

	var apiResp *GetJobResponse
	if err = mapstructure.Decode(response, &apiResp); err != nil {
		return nil, errorHandler.MakeAndReportError("failed to decode response from GET job", fmt.Sprintf("error: %s, statusCode %d, response %#v", err, statusCode, response))
	}
	tflog.Debug(errorHandler.Ctx, fmt.Sprintf("read job info: %#v", apiResp.Data))

	apiResp.Data.Status = apiResp.Status

	return &apiResp.Data, nil
}

// CreateJob creates a job.
func CreateJob(errorHandler *utils.ErrorHandler, r restclient.RestClient, data JobResourceModel) (*GetJobResponse, error) {
	var body map[string]interface{}
	if err := mapstructure.Decode(data, &body); err != nil {
		return nil, errorHandler.MakeAndReportError("error encoding job body", fmt.Sprintf("error on encoding POST job/ body: %s, body: %#v", err, data))
	}

	extravarsMap := make(map[string]string)
	for key, value := range data.Extravars {
		value = strings.Replace(fmt.Sprintf("%s", value), "\"", "", -1)
		extravarsMap[key] = fmt.Sprintf("%s", value)
	}

	body["extravars"] = extravarsMap

	credentialsMap := make(map[string]string)
	for key, value := range data.Credentials {
		value = strings.Replace(fmt.Sprintf("%s", value), "\"", "", -1)
		credentialsMap[key] = fmt.Sprintf("%s", value)
	}

	body["credentials"] = credentialsMap

	status, response, err := r.CallCreateMethod("job/", nil, body) // Ansible Forms API does not allow querying.
	if err != nil {
		return nil, errorHandler.MakeAndReportError("error creating job", fmt.Sprintf("error on POST job/: %s, status %v", err, status))
	}

	var resp *CreateJobResponse
	if err = mapstructure.Decode(response.Records[0], &resp); err != nil {
		return nil, errorHandler.MakeAndReportError("failed to decode response from POST job/", fmt.Sprintf("error: %s, status %s, response %#v", err, status, response))
	}
	jobData, err := GetJobByID(errorHandler, r, resp.Data.Output.ID)
	if err != nil {
		return nil, errorHandler.MakeAndReportError("failed to retrieve response from GET job/", fmt.Sprintf("error: %s, status %s, response %#v", err, status, response))
	}

	tflog.Debug(errorHandler.Ctx, fmt.Sprintf("Create svm source - udata: %#v", jobData))

	return &GetJobResponse{
		Status:  resp.Status,
		Message: resp.Message,
		Data:    *jobData,
	}, nil
}

// DeleteJobByID deletes a job by ID.
func DeleteJobByID(errorHandler *utils.ErrorHandler, r restclient.RestClient, id int64) error {
	statusCode, _, err := r.CallDeleteMethod(fmt.Sprintf("job/%d", id), nil, nil)
	if err != nil {
		return errorHandler.MakeAndReportError("error deleting job info", fmt.Sprintf("error on DELETE job/: %s, statusCode %d", err, statusCode))
	}

	return nil
}
