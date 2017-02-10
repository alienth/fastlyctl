package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type RequestSettingConfig config

type RequestSetting struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,string,omitempty"`

	Name             string      `json:"name,omitempty"`
	BypassBusyWait   Compatibool `json:"bypass_busy_wait,omitempty"`
	DefaultHost      string      `json:"default_host,omitempty"`
	ForceMiss        Compatibool `json:"force_miss,omitempty"`
	ForceSSL         Compatibool `json:"force_ssl,omitempty"`
	GeoHeaders       Compatibool `json:"geo_headers,omitempty"`
	HashKeys         string      `json:"hash_keys,omitempty"`
	MaxStaleAge      int         `json:"max_stale_age,string,omitempty"`
	RequestCondition string      `json:"request_condition,omitempty"`
	TimerSupport     Compatibool `json:"timer_support,omitempty"`

	// Takes specific string values
	XFF    string `json:"xff,omitempty"`
	Action string `json:"action,omitempty"`
}

// requestSettingsByName is a sortable list of requestSettings.
type requestSettingsByName []*RequestSetting

// Len, Swap, and Less implement the sortable interface.
func (s requestSettingsByName) Len() int      { return len(s) }
func (s requestSettingsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s requestSettingsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List requestSettings for a specific service and version.
func (c *RequestSettingConfig) List(serviceID string, version uint) ([]*RequestSetting, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/request_settings", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	requestSettings := new([]*RequestSetting)
	resp, err := c.client.Do(req, requestSettings)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(requestSettingsByName(*requestSettings))

	return *requestSettings, resp, nil
}

// Get fetches a specific request setting by name.
func (c *RequestSettingConfig) Get(serviceID string, version uint, name string) (*RequestSetting, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/request_settings/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	requestSetting := new(RequestSetting)
	resp, err := c.client.Do(req, requestSetting)
	if err != nil {
		return nil, resp, err
	}
	return requestSetting, resp, nil
}

// Create a new request setting.
func (c *RequestSettingConfig) Create(serviceID string, version uint, requestSetting *RequestSetting) (*RequestSetting, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/request_settings", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, requestSetting)
	if err != nil {
		return nil, nil, err
	}

	b := new(RequestSetting)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a request setting
func (c *RequestSettingConfig) Update(serviceID string, version uint, name string, requestSetting *RequestSetting) (*RequestSetting, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/request_settings/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, requestSetting)
	if err != nil {
		return nil, nil, err
	}

	b := new(RequestSetting)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a request setting
func (c *RequestSettingConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/request_settings/%s", serviceID, version, name)

	req, err := c.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}
