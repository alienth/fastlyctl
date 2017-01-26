package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type GzipConfig config

type Gzip struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,string,omitempty"`

	CacheCondition string `json:"cache_condition,omitempty"`
	ContentTypes   string `json:"content_types,omitempty"`
	Extensions     string `json:"extensions,omitempty"`
	Name           string `json:"name,omitempty"`
}

// gzipsByName is a sortable list of gzips.
type gzipsByName []*Gzip

// Len, Swap, and Less implement the sortable interface.
func (s gzipsByName) Len() int      { return len(s) }
func (s gzipsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s gzipsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List gzips for a specific service and version.
func (c *GzipConfig) List(serviceID string, version uint) ([]*Gzip, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/gzip", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	gzips := new([]*Gzip)
	resp, err := c.client.Do(req, gzips)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(gzipsByName(*gzips))

	return *gzips, resp, nil
}

// Get fetches a specific gzip by name.
func (c *GzipConfig) Get(serviceID string, version uint, name string) (*Gzip, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/gzip/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	gzip := new(Gzip)
	resp, err := c.client.Do(req, gzip)
	if err != nil {
		return nil, resp, err
	}
	return gzip, resp, nil
}

// Create a new gzip.
func (c *GzipConfig) Create(serviceID string, version uint, gzip *Gzip) (*Gzip, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/gzip", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, gzip)
	if err != nil {
		return nil, nil, err
	}

	b := new(Gzip)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a gzip
func (c *GzipConfig) Update(serviceID string, version uint, name string, gzip *Gzip) (*Gzip, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/gzip/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, gzip)
	if err != nil {
		return nil, nil, err
	}

	b := new(Gzip)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a gzip
func (c *GzipConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/gzip/%s", serviceID, version, name)

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
