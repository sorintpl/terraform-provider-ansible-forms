package restclient

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/mitchellh/mapstructure"

	"terraform-provider-ansible-forms/internal/restclient/httpclient"
)

const (
	CheckLoopInterval    = 15 * time.Second
	CheckLoopTimeout     = 3 * time.Minute
	AnsibleStatusSuccess = "success"
	AnsibleStatusFailure = "error" // failure was not returned but maybe because of testing
)

// ConnectionProfile describes out to reach a cluster or svm.
type ConnectionProfile struct {
	// TODO: add certs in addition to basic authentication
	// TODO: Add Timeout (currently hardcoded to 10 seconds)
	Hostname              string
	Username              string
	Password              string
	ValidateCerts         bool
	MaxConcurrentRequests int
}

// RestClient to interact with the Ansible Forms REST API.
type RestClient struct {
	connectionProfile     ConnectionProfile
	ctx                   context.Context
	maxConcurrentRequests int
	httpClient            httpclient.HTTPClient
	requestSlots          chan int
	mode                  string
	responses             []MockResponse
	jobCompletionTimeOut  int
	tag                   string
}

// NewClient creates a new REST client and a supporting HTTP client.
func NewClient(ctx context.Context, cxProfile ConnectionProfile, tag string, jobCompletionTimeOut int) (*RestClient, error) {
	var httpProfile httpclient.HTTPProfile
	err := mapstructure.Decode(cxProfile, &httpProfile)
	if err != nil {
		msg := fmt.Sprintf("decode error on ConnectionProfile %#v to HTTPProfile", cxProfile)
		tflog.Error(ctx, msg)
		return nil, errors.New(msg)
	}
	httpProfile.APIRoot = "api/v1"
	maxConcurrentRequests := cxProfile.MaxConcurrentRequests
	if maxConcurrentRequests == 0 {
		maxConcurrentRequests = 6
	}
	client := RestClient{
		connectionProfile:     cxProfile,
		ctx:                   ctx,
		httpClient:            httpclient.NewClient(ctx, httpProfile, tag),
		maxConcurrentRequests: maxConcurrentRequests,
		mode:                  "prod",
		requestSlots:          make(chan int, maxConcurrentRequests),
		jobCompletionTimeOut:  jobCompletionTimeOut,
		tag:                   tag,
	}

	return &client, nil
}

// CallCreateMethod returns response from POST results.  An error is reported if an error is received.
func (r *RestClient) CallCreateMethod(baseURL string, query *RestQuery, body map[string]any) (string, RestResponse, error) {
	if query == nil {
		query = r.NewQuery()
	}
	// TODO: make this a connection parameter ?
	query.Set("return_timeout", "60")
	statusCode, response, err := r.callAPIMethod("POST", baseURL, query, body)
	if err != nil {
		tflog.Debug(r.ctx, fmt.Sprintf("CallCreateMethod request failed %#v", statusCode))
		return "", RestResponse{}, err
	}

	status := "running"
	resp_id := response.Records[0]["data"].(map[string]any)["output"].(map[string]any)["id"]
	id, _ := big.NewFloat(resp_id.(float64)).Int64()
check:
	for {
		select {
		case <-time.After(CheckLoopInterval):
			statusCode, restInfo, err := r.GetNilOrOneRecord(fmt.Sprintf("job/%d", id), nil, nil)
			if err != nil {
				return "", RestResponse{}, fmt.Errorf("error on GET job/%d: %s, statusCode %d", id, err, statusCode)
			}
			status = restInfo["status"].(string)
			// are there other statuses possible?
			if status == AnsibleStatusSuccess {
				break check
			} else if status == AnsibleStatusFailure {
				return "", RestResponse{}, fmt.Errorf("when checking job status, Ansible returned failure")
			} else {
				return "", RestResponse{}, fmt.Errorf("status was different than expected - %+v", status)
			}
		case <-time.After(CheckLoopTimeout):
			tflog.Debug(r.ctx, "job status check timed-out")
			return "", RestResponse{}, fmt.Errorf("timed-out while checking job status")
		}
	}

	return status, response, err
}

// CallUpdateMethod returns response from PATCH results.  An error is reported if an error is received.
func (r *RestClient) CallUpdateMethod(baseURL string, query *RestQuery, body map[string]any) (int, RestResponse, error) {
	if query == nil {
		query = r.NewQuery()
	}
	// TODO: make this a connection parameter ?
	query.Set("return_timeout", "60")
	statusCode, response, err := r.callAPIMethod("PATCH", baseURL, query, body)
	if err != nil {
		tflog.Debug(r.ctx, fmt.Sprintf("CallUpdateMethod request failed %#v", statusCode))
		return statusCode, RestResponse{}, err
	}

	if response.Job != nil {
		statusCode, _, err = r.Wait(response.Job["uuid"].(string))
		if err != nil {
			return statusCode, RestResponse{}, err
		}
	} else if response.Jobs != nil {
		for _, v := range response.Jobs {
			statusCode, _, err = r.Wait(v["uuid"].(string))
			if err != nil {
				return statusCode, RestResponse{}, err
			}
		}
	}

	return statusCode, response, err
}

// CallDeleteMethod returns response from DELETE results.  An error is reported if an error is received.
func (r *RestClient) CallDeleteMethod(baseURL string, query *RestQuery, body map[string]any) (int, RestResponse, error) {
	if query == nil {
		query = r.NewQuery()
	}
	// TODO: make this a connection parameter ?
	query.Set("return_timeout", "60")
	statusCode, response, err := r.callAPIMethod("DELETE", baseURL, query, body)
	if err != nil {
		tflog.Debug(r.ctx, fmt.Sprintf("CallDeleteMethod request failed %#v", statusCode))
		return statusCode, RestResponse{}, err
	}

	// TODO: handle waitOnCompletion
	return statusCode, response, err
}

// GetNilOrOneRecord returns nil if no record is found or a single record.  An error is reported if multiple records are received.
func (r *RestClient) GetNilOrOneRecord(baseURL string, query *RestQuery, body map[string]any) (int, map[string]any, error) {
	statusCode, response, err := r.callAPIMethod("GET", baseURL, query, body)
	if err != nil {
		return statusCode, nil, err
	}
	if response.NumRecords > 1 {
		msg := fmt.Sprintf("received 2 or more records when only one is expected - statusCode %d, err=%#v, response=%#v", statusCode, err, response)
		tflog.Error(r.ctx, msg)
		return statusCode, nil, errors.New(msg)
	}
	if response.NumRecords == 1 {
		return statusCode, response.Records[0], err
	}

	return statusCode, nil, err
}

// GetZeroOrMoreRecords returns a list of records.
func (r *RestClient) GetZeroOrMoreRecords(baseURL string, query *RestQuery, body map[string]any) (int, []map[string]any, error) {
	statusCode, response, err := r.callAPIMethod("GET", baseURL, query, body)
	if err != nil {
		return statusCode, nil, err
	}

	return statusCode, response.Records, err
}

// Wait waits for job to finish.
func (r *RestClient) Wait(uuid string) (int, RestResponse, error) {
	timeRemaining := r.jobCompletionTimeOut
	errorRetries := 3
	for timeRemaining > 0 {
		statusCode, response, err := r.GetNilOrOneRecord("job/"+uuid, nil, nil)
		if err != nil {
			if errorRetries <= 0 {
				return statusCode, RestResponse{}, err
			}
			time.Sleep(10 * time.Second)
			errorRetries--
			continue
		}
		var job Job
		if err := mapstructure.Decode(response, &job); err != nil {
			tflog.Error(r.ctx, fmt.Sprintf("Read job data - decode error: %s, data: %#v", err, response))
			return statusCode, RestResponse{}, err
		}
		if job.State == "queued" || job.State == "running" || job.State == "paused" {
			timeRemaining = timeRemaining - 10
		} else if job.State == "success" {
			return statusCode, RestResponse{}, nil
		} else {
			// if job struct ifself contains message and code, jobError struct might be empty. Vice versa.
			if job.Error != (jobError{}) {
				if job.Error.Code != "" {
					errorMessage := fmt.Errorf("fail to get job status. Error code: %s. Message: %s, Target: %s", job.Error.Code, job.Error.Message, job.Error.Target)
					return statusCode, RestResponse{}, errorMessage
				}
				return statusCode, RestResponse{}, fmt.Errorf("fail to get job status. Unknown error")
			}
			if job.Code != 0 {
				return statusCode, RestResponse{}, fmt.Errorf("job UUID %s failed. Error code: %d. Message: %s", uuid, job.Code, job.Message)
			}
		}
		time.Sleep(10 * time.Second)
	}

	// TODO: clean up the resources in creation when errors out.
	return 0, RestResponse{}, fmt.Errorf("fail to wait for job to finish. Exit now")
}

// callAPIMethod can be used to make a request to any REST API method, receiving response as bytes.
func (r *RestClient) callAPIMethod(method string, baseURL string, query *RestQuery, body map[string]any) (int, RestResponse, error) {
	if r.mode == "mock" {
		return r.mockCallAPIMethod(method, baseURL, query, body)
	}
	r.waitForAvailableSlot()
	defer r.releaseSlot()

	values := url.Values{}
	if query != nil {
		values = query.Values
	}

	statusCode, response, httpClientErr := r.httpClient.Do(baseURL, &httpclient.Request{
		Method: method,
		Body:   body,
		Query:  values,
	})

	// TODO: error handling for HTTTP status code >=300
	// TODO: handle async calls (job in response)
	return r.unmarshalResponse(statusCode, response, httpClientErr)
}

func (r *RestClient) waitForAvailableSlot() {
	r.requestSlots <- 1
}

func (r *RestClient) releaseSlot() {
	<-r.requestSlots
}

// Equals is a test function for Unit Testing
func (r *RestClient) Equals(r2 *RestClient) (ok bool, firstDiff string) {
	if r.connectionProfile != r2.connectionProfile {
		return false, fmt.Sprintf("expected %#v, got %#v", r.connectionProfile, r2.connectionProfile)
	}
	if r.tag != r2.tag {
		return false, fmt.Sprintf("expected %#v, got %#v", r.tag, r2.tag)
	}

	return true, ""
}

// RestQuery is a wrapper around urlValues, and supports a Fields method in addition to Set, Add.
type RestQuery struct {
	url.Values
}

// NewQuery is used to provide query parameters.  Set and Add functions are inherited from url.Values.
func (r *RestClient) NewQuery() *RestQuery {
	query := new(RestQuery)
	query.Values = url.Values{}

	return query
}

// Fields adds a list of fields to query.
func (q *RestQuery) Fields(fields []string) {
	q.Set("fields", strings.Join(fields, ","))
}

// SetValues adds a set of key, value
func (q *RestQuery) SetValues(keyValues map[string]any) {
	for k, v := range keyValues {
		// TODO: add some type validation
		value := fmt.Sprintf("%v", v)
		if value != "" {
			q.Set(k, value)
		}
	}
}

// Job is Ansible Forms API job data structure
type Job struct {
	State   string
	Error   jobError
	Code    int
	Message string
}

type jobError struct {
	Message string `tfsdk:"state"`
	Code    string `tfsdk:"code"`
	Target  string `tfsdk:"target"`
}
