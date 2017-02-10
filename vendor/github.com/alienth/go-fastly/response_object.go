package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type ResponseObjectConfig config

type ResponseObject struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,string,omitempty"`

	Name             string `json:"name,omitempty"`
	CacheCondition   string `json:"cache_condition,omitempty"`
	Content          string `json:"content,omitempty"`
	ContentType      string `json:"content_type,omitempty"`
	Status           string `json:"status,omitempty"`
	Response         string `json:"response,omitempty"`
	RequestCondition string `json:"request_condition,omitempty"`
}

// responseObjectsByName is a sortable list of responseObjects.
type responseObjectsByName []*ResponseObject

// Len, Swap, and Less implement the sortable interface.
func (s responseObjectsByName) Len() int      { return len(s) }
func (s responseObjectsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s responseObjectsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List responseObjects for a specific service and version.
func (c *ResponseObjectConfig) List(serviceID string, version uint) ([]*ResponseObject, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/response_object", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	responseObjects := new([]*ResponseObject)
	resp, err := c.client.Do(req, responseObjects)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(responseObjectsByName(*responseObjects))

	return *responseObjects, resp, nil
}

// Get fetches a specific response object by name.
func (c *ResponseObjectConfig) Get(serviceID string, version uint, name string) (*ResponseObject, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/response_object/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	responseObject := new(ResponseObject)
	resp, err := c.client.Do(req, responseObject)
	if err != nil {
		return nil, resp, err
	}
	return responseObject, resp, nil
}

// Create a new response object.
func (c *ResponseObjectConfig) Create(serviceID string, version uint, responseObject *ResponseObject) (*ResponseObject, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/response_object", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, responseObject)
	if err != nil {
		return nil, nil, err
	}

	b := new(ResponseObject)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a response object
func (c *ResponseObjectConfig) Update(serviceID string, version uint, name string, responseObject *ResponseObject) (*ResponseObject, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/response_object/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, responseObject)
	if err != nil {
		return nil, nil, err
	}

	b := new(ResponseObject)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a response object
func (c *ResponseObjectConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/response_object/%s", serviceID, version, name)

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
