package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type ServiceConfig config

type Service struct {
	ID string `json:"id,omitempty"`

	Version    uint       `json:"version,omitempty"`
	Name       string     `json:"name,omitempty"`
	Comment    string     `json:"comment"`
	CustomerID string     `json:"customer_id,omitempty"`
	Versions   []*Version `json:"versions,omitempty"`
}

// servicesByName is a sortable list of services.
type servicesByName []*Service

// Len, Swap, and Less implement the sortable interface.
func (s servicesByName) Len() int      { return len(s) }
func (s servicesByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s servicesByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List services.
func (c *ServiceConfig) List() ([]*Service, *http.Response, error) {
	u := "/service"

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	services := new([]*Service)
	resp, err := c.client.Do(req, services)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(servicesByName(*services))

	return *services, resp, nil
}

// Get fetches a specific service by ID.
func (c *ServiceConfig) Get(serviceID string) (*Service, *http.Response, error) {
	u := fmt.Sprintf("/service/%s", serviceID)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	service := new(Service)
	resp, err := c.client.Do(req, service)
	if err != nil {
		return nil, resp, err
	}
	return service, resp, nil
}

// Search fetches a specific service by name.
func (c *ServiceConfig) Search(name string) (*Service, *http.Response, error) {
	u := fmt.Sprintf("/service/search?name=%s", name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	service := new(Service)
	resp, err := c.client.Do(req, service)
	if err != nil {
		return nil, resp, err
	}
	return service, resp, nil
}

// Create a new service.
func (c *ServiceConfig) Create(service *Service) (*Service, *http.Response, error) {
	u := "/service"

	req, err := c.client.NewJSONRequest("POST", u, service)
	if err != nil {
		return nil, nil, err
	}

	b := new(Service)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a service
func (c *ServiceConfig) Update(serviceID string, service *Service) (*Service, *http.Response, error) {
	u := fmt.Sprintf("/service/%s", serviceID)

	req, err := c.client.NewJSONRequest("PUT", u, service)
	if err != nil {
		return nil, nil, err
	}

	b := new(Service)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a service
func (c *ServiceConfig) Delete(serviceID string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s", serviceID)

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
