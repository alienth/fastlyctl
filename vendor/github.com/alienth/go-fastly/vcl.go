package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type VCLConfig config

type VCL struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,omitempty"`

	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
	Main    bool   `json:"main,omitempty"`
}

// vclsByName is a sortable list of vcls.
type vclsByName []*VCL

// Len, Swap, and Less implement the sortable interface.
func (s vclsByName) Len() int      { return len(s) }
func (s vclsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s vclsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List vcls for a specific service and version.
func (c *VCLConfig) List(serviceID string, version uint) ([]*VCL, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/vcl", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	vcls := new([]*VCL)
	resp, err := c.client.Do(req, vcls)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(vclsByName(*vcls))

	return *vcls, resp, nil
}

// Get fetches a specific vcl by name.
func (c *VCLConfig) Get(serviceID string, version uint, name string) (*VCL, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/vcl/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	vcl := new(VCL)
	resp, err := c.client.Do(req, vcl)
	if err != nil {
		return nil, resp, err
	}
	return vcl, resp, nil
}

// Create a new vcl.
func (c *VCLConfig) Create(serviceID string, version uint, vcl *VCL) (*VCL, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/vcl", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, vcl)
	if err != nil {
		return nil, nil, err
	}

	b := new(VCL)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a vcl
func (c *VCLConfig) Update(serviceID string, version uint, name string, vcl *VCL) (*VCL, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/vcl/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, vcl)
	if err != nil {
		return nil, nil, err
	}

	b := new(VCL)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a vcl
func (c *VCLConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/vcl/%s", serviceID, version, name)

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
