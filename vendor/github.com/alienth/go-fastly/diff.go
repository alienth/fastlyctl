package fastly

import (
	"fmt"
	"net/http"
)

type DiffConfig config

type Diff struct {
	// Read only
	ServiceID string `json:"service_id,omitempty"`
	Diff      string

	// Read/Write
	FromVersion uint       `json:"version,omitempty"`
	ToVersion   uint       `json:"version,omitempty"`
	Format      DiffFormat `json:"format,omitempty"`
}

type DiffFormat string

const (
	DiffFormatText       = "text"
	DiffFormatHTML       = "html"
	DiffFormatHTMLSimple = "html_simple"
)

// Get fetches a specific backend by name.
func (c *DiffConfig) Get(serviceID string, from, to uint, format DiffFormat) (*Diff, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/diff/from/%d/to/%d", serviceID, from, to)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	diff := new(Diff)
	resp, err := c.client.Do(req, diff)
	if err != nil {
		return nil, resp, err
	}
	return diff, resp, nil
}
