package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type DomainConfig config

type Domain struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,omitempty"`

	Name    string `json:"name"`
	Comment string `json:"comment"`
}

// domainsByName is a sortable list of domains.
type domainsByName []*Domain

// Len, Swap, and Less implement the sortable interface.
func (s domainsByName) Len() int      { return len(s) }
func (s domainsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s domainsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List domains for a specific service and version.
func (c *DomainConfig) List(serviceID string, version uint) ([]*Domain, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/domain", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	domains := new([]*Domain)
	resp, err := c.client.Do(req, domains)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(domainsByName(*domains))

	return *domains, resp, nil
}

// Get fetches a specific domain by name.
func (c *DomainConfig) Get(serviceID string, version uint, name string) (*Domain, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/domain/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	domain := new(Domain)
	resp, err := c.client.Do(req, domain)
	if err != nil {
		return nil, resp, err
	}
	return domain, resp, nil
}

// Create a new domain.
func (c *DomainConfig) Create(serviceID string, version uint, domain *Domain) (*Domain, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/domain", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, domain)
	if err != nil {
		return nil, nil, err
	}

	b := new(Domain)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a domain
func (c *DomainConfig) Update(serviceID string, version uint, name string, domain *Domain) (*Domain, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/domain/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, domain)
	if err != nil {
		return nil, nil, err
	}

	b := new(Domain)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a domain
func (c *DomainConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/domain/%s", serviceID, version, name)

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
