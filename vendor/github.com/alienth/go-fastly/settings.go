package fastly

import (
	"fmt"
	"net/http"
)

type SettingsConfig config

type Settings struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,omitempty"`

	DefaultTTL  uint   `json:"general.default_ttl,omitempty"`
	DefaultHost string `json:"general.default_host"`
}

// Get settings
func (c *SettingsConfig) Get(serviceID string, version uint) (*Settings, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/settings", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	settings := new(Settings)
	resp, err := c.client.Do(req, settings)
	if err != nil {
		return nil, resp, err
	}

	return settings, resp, nil
}

// Update settings
func (c *SettingsConfig) Update(serviceID string, version uint, settings *Settings) (*Settings, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/settings", serviceID, version)

	req, err := c.client.NewJSONRequest("PUT", u, settings)
	if err != nil {
		return nil, nil, err
	}

	b := new(Settings)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}
