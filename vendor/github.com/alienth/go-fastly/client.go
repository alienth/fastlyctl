package fastly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime"
)

// defaultBaseURL is the default endpoint for Fastly. Since Fastly does not
// support an on-premise solution, this is likely to always be the default.
const defaultBaseURL = "https://api.fastly.com/"

// ProjectURL is the url for this library.
var ProjectURL = "github.com/alienth/go-fastly"

// ProjectVersion is the version of this library.
var ProjectVersion = "0.1"

// UserAgent is the user agent for this particular client.
var userAgent = fmt.Sprintf("alienth/go-fastly/%s (+%s; %s)",
	ProjectVersion, ProjectURL, runtime.Version())

// Client is the main entrypoint to the Fastly golang API library.
type Client struct {
	client *http.Client

	// Base URL for API requests.
	BaseURL *url.URL

	UserAgent string

	common config // Reuse a single struct instead of allocating one for each service on the heap.

	// Configs used for interacting with different parts of the Fastly API
	ACL            *ACLConfig
	ACLEntry       *ACLEntryConfig
	Backend        *BackendConfig
	CacheSetting   *CacheSettingConfig
	Condition      *ConditionConfig
	Dictionary     *DictionaryConfig
	DictionaryItem *DictionaryItemConfig
	Diff           *DiffConfig
	Domain         *DomainConfig

	Gzip           *GzipConfig
	Header         *HeaderConfig
	HealthCheck    *HealthCheckConfig
	RequestSetting *RequestSettingConfig
	ResponseObject *ResponseObjectConfig
	S3             *S3Config
	Service        *ServiceConfig
	Settings       *SettingsConfig
	Syslog         *SyslogConfig
	Version        *VersionConfig
	VCL            *VCLConfig
	// apiKey is the Fastly API key to authenticate requests.
	apiKey string
}

type config struct {
	client *Client
}

// NewClient returns a new Fastly API client. If a nil httpClient is provided,
// http.DefaultClient will be used.
func NewClient(httpClient *http.Client, key string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	baseURL, _ := url.Parse(defaultBaseURL)

	c := &Client{client: httpClient, BaseURL: baseURL, UserAgent: userAgent}
	c.common.client = c
	c.ACL = (*ACLConfig)(&c.common)
	c.ACLEntry = (*ACLEntryConfig)(&c.common)
	c.Backend = (*BackendConfig)(&c.common)
	c.CacheSetting = (*CacheSettingConfig)(&c.common)
	c.Condition = (*ConditionConfig)(&c.common)
	c.Dictionary = (*DictionaryConfig)(&c.common)
	c.DictionaryItem = (*DictionaryItemConfig)(&c.common)
	c.Diff = (*DiffConfig)(&c.common)
	c.Domain = (*DomainConfig)(&c.common)

	c.Gzip = (*GzipConfig)(&c.common)
	c.Header = (*HeaderConfig)(&c.common)
	c.HealthCheck = (*HealthCheckConfig)(&c.common)
	c.RequestSetting = (*RequestSettingConfig)(&c.common)
	c.ResponseObject = (*ResponseObjectConfig)(&c.common)
	c.S3 = (*S3Config)(&c.common)
	c.Service = (*ServiceConfig)(&c.common)
	c.Settings = (*SettingsConfig)(&c.common)
	c.Syslog = (*SyslogConfig)(&c.common)
	c.Version = (*VersionConfig)(&c.common)
	c.VCL = (*VCLConfig)(&c.common)
	c.apiKey = key
	return c
}

// NewRequest creates an API request. A relative URL can be provided in urlStr,
// in which case it is resolved relative to the BaseURL of the Client.
func (c *Client) NewRequest(method, urlStr string, body io.Reader) (*http.Request, error) {
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.ResolveReference(rel)

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}

	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req.Header.Set("Fastly-Key", c.apiKey)
	return req, nil
}

// NewJSONRequest creates an http.Request with a JSON body for use with the
// fastly API. The item passed in `body` will be Marshalled into JSON.
func (c *Client) NewJSONRequest(method, urlStr string, body interface{}) (*http.Request, error) {
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := c.NewRequest(method, urlStr, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// Do sends an API request and returns the response. The response is JSON
// decoded and stored in the value pointed to by v, or returned as an error if
// an API error has occurred.
func (c *Client) Do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	err = CheckResponse(resp)
	if err != nil {
		// return response regardless for caller inspection
		return resp, err
	}

	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
		if err == io.EOF {
			err = nil // ignore EOF errors caused by empty response body
		}
	}

	return resp, err
}

// CheckResponse takes in an HTTP response containing a JSON-encoded error,
// unmarshals the error, and returns it. Assumes no error if status code is
// successful.
func CheckResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}
	errorResponse := &ErrorResponse{Response: r}
	data, err := ioutil.ReadAll(r.Body)
	if err == nil && data != nil {
		json.Unmarshal(data, errorResponse)
	}

	// 401 Unauthorized
	// {"msg":"Provided credentials are missing or invalid"}
	// 400 Bad Request
	// {"msg":{"error":"2fa.verify","error_description":"Invalid one-time password."}}
	// 403 Forbidden
	// {"msg":"You are not authorized to perform this action"}

	return errorResponse
}

// ErrorResponse represents the error message sent back from Fastly.
type ErrorResponse struct {
	Response *http.Response // The response that held this error
	Message  string         `json:"msg"`
	Detail   string         `json:"detail"`
	//	Message  *struct {
	//	    Error string  `json:"error,omitempty"`
	//	    ErrorDescription string  `json:"error_description,omitempty"`
	//	} `json:"msg"`
}

// Error generates an error message based on an ErrorResponse.
func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %v %v",
		r.Response.Request.Method, r.Response.Request.URL,
		r.Response.StatusCode, r.Message, r.Detail)
}
