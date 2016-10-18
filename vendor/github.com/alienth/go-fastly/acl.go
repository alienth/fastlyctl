package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type ACLConfig config

type ACL struct {
	ServiceID string `json:"service_id"`
	Version   uint   `json:"version,string"`
	ID        string `json:"id"`

	Name string `json:"name" url:"name,omitempty"`
}

// aclsByName is a sortable list of acls.
type aclsByName []*ACL

// Len, Swap, and Less implement the sortable interface.
func (s aclsByName) Len() int      { return len(s) }
func (s aclsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s aclsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List acls for a specific service and version.
func (c *ACLConfig) List(serviceID string, version uint) ([]*ACL, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/acl", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	acls := new([]*ACL)
	resp, err := c.client.Do(req, acls)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(aclsByName(*acls))

	return *acls, resp, nil
}

// Get fetches a specific acl by name.
func (c *ACLConfig) Get(serviceID string, version uint, name string) (*ACL, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/acl/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	acl := new(ACL)
	resp, err := c.client.Do(req, acl)
	if err != nil {
		return nil, resp, err
	}
	return acl, resp, nil
}

// Create a new acl.
func (c *ACLConfig) Create(serviceID string, version uint, acl *ACL) (*ACL, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/acl", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, acl)
	if err != nil {
		return nil, nil, err
	}

	b := new(ACL)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a acl
func (c *ACLConfig) Update(serviceID string, version uint, name string, acl *ACL) (*ACL, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/acl/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, acl)
	if err != nil {
		return nil, nil, err
	}

	b := new(ACL)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a acl
func (c *ACLConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/acl/%s", serviceID, version, name)

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
