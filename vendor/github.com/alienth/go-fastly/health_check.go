package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type HealthCheckConfig config

type HealthCheck struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,omitempty"`

	Name             string `json:"name,omitempty"`
	CheckInterval    uint   `json:"check_interval,omitempty"`
	Initial          uint   `json:"initial,omitempty"`
	Threshold        uint   `json:"threshold,omitempty"`
	Timeout          uint   `json:"timeout,omitempty"`
	Window           uint   `json:"window,omitempty"`
	Comment          string `json:"comment,omitempty"`
	ExpectedResponse uint   `json:"expected_response,omitempty"`
	Host             string `json:"host,omitempty"`
	HTTPVersion      string `json:"http_version,omitempty"`
	Method           string `json:"method,omitempty"`
	Path             string `json:"path,omitempty"`
}

// healthChecksByName is a sortable list of healthChecks.
type healthChecksByName []*HealthCheck

// Len, Swap, and Less implement the sortable interface.
func (s healthChecksByName) Len() int      { return len(s) }
func (s healthChecksByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s healthChecksByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List healthChecks for a specific service and version.
func (c *HealthCheckConfig) List(serviceID string, version uint) ([]*HealthCheck, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/healthcheck", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	healthChecks := new([]*HealthCheck)
	resp, err := c.client.Do(req, healthChecks)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(healthChecksByName(*healthChecks))

	return *healthChecks, resp, nil
}

// Get fetches a specific health check by name.
func (c *HealthCheckConfig) Get(serviceID string, version uint, name string) (*HealthCheck, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/healthcheck/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	healthCheck := new(HealthCheck)
	resp, err := c.client.Do(req, healthCheck)
	if err != nil {
		return nil, resp, err
	}
	return healthCheck, resp, nil
}

// Create a new health check.
func (c *HealthCheckConfig) Create(serviceID string, version uint, healthCheck *HealthCheck) (*HealthCheck, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/healthcheck", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, healthCheck)
	if err != nil {
		return nil, nil, err
	}

	b := new(HealthCheck)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a health check
func (c *HealthCheckConfig) Update(serviceID string, version uint, name string, healthCheck *HealthCheck) (*HealthCheck, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/healthcheck/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, healthCheck)
	if err != nil {
		return nil, nil, err
	}

	b := new(HealthCheck)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a health check
func (c *HealthCheckConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/healthcheck/%s", serviceID, version, name)

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
