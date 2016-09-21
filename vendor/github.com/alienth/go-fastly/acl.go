package fastly

import (
	"fmt"
	"sort"
)

// ACL represents a ACL response from the Fastly API.
type ACL struct {
	ServiceID string `mapstructure:"service_id"`
	Version   string `mapstructure:"version"`

	ID      string `mapstructure:"id"`
	Name    string `mapstructure:"name"`
	Created string `mapstructure:"created_at"`
	Updated string `mapstructure:"updated_at"`
}

// aclsByName is a sortable list of dictionaries.
type aclsByName []*ACL

// Len, Swap, and Less implement the sortable interface.
func (s aclsByName) Len() int      { return len(s) }
func (s aclsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s aclsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// ListACLsInput is used as input to the ListACLs function.
type ListACLsInput struct {
	// Service is the ID of the service (required).
	Service string

	// Version is the specific configuration version (required).
	Version string
}

// ListACLs returns the list of ACLs for the configuration version.
func (c *Client) ListACLs(i *ListACLsInput) ([]*ACL, error) {
	if i.Service == "" {
		return nil, ErrMissingService
	}

	if i.Version == "" {
		return nil, ErrMissingVersion
	}

	path := fmt.Sprintf("/service/%s/version/%s/acl", i.Service, i.Version)
	resp, err := c.Get(path, nil)
	if err != nil {
		return nil, err
	}

	var bs []*ACL
	if err := decodeJSON(&bs, resp.Body); err != nil {
		return nil, err
	}
	sort.Stable(aclsByName(bs))
	return bs, nil
}

// CreateACLInput is used as input to the CreateACL function.
type CreateACLInput struct {
	// Service is the ID of the service. Version is the specific configuration
	// version. Both fields are required.
	Service string
	Version string

	Name string `form:"name,omitempty"`
}

// CreateACL creates a new Fastly acl.
func (c *Client) CreateACL(i *CreateACLInput) (*ACL, error) {
	if i.Service == "" {
		return nil, ErrMissingService
	}

	if i.Version == "" {
		return nil, ErrMissingVersion
	}

	path := fmt.Sprintf("/service/%s/version/%s/acl", i.Service, i.Version)
	resp, err := c.PostForm(path, i, nil)
	if err != nil {
		return nil, err
	}

	var b *ACL
	if err := decodeJSON(&b, resp.Body); err != nil {
		return nil, err
	}
	return b, nil
}

// GetACLInput is used as input to the GetACL function.
type GetACLInput struct {
	// Service is the ID of the service. Version is the specific configuration
	// version. Both fields are required.
	Service string
	Version string

	// Name is the name of the acl to fetch.
	Name string
}

// GetACL gets the acl configuration with the given parameters.
func (c *Client) GetACL(i *GetACLInput) (*ACL, error) {
	if i.Service == "" {
		return nil, ErrMissingService
	}

	if i.Version == "" {
		return nil, ErrMissingVersion
	}

	if i.Name == "" {
		return nil, ErrMissingName
	}

	path := fmt.Sprintf("/service/%s/version/%s/acl/%s", i.Service, i.Version, i.Name)
	resp, err := c.Get(path, nil)
	if err != nil {
		return nil, err
	}

	var b *ACL
	if err := decodeJSON(&b, resp.Body); err != nil {
		return nil, err
	}
	return b, nil
}

// UpdateACLInput is used as input to the UpdateACL function.
type UpdateACLInput struct {
	// Service is the ID of the service. Version is the specific configuration
	// version. Both fields are required.
	Service string
	Version string

	// Name is the name of the acl to update.
	Name string

	NewName string `form:"name,omitempty"`
}

// UpdateACL updates a specific acl.
func (c *Client) UpdateACL(i *UpdateACLInput) (*ACL, error) {
	if i.Service == "" {
		return nil, ErrMissingService
	}

	if i.Version == "" {
		return nil, ErrMissingVersion
	}

	if i.Name == "" {
		return nil, ErrMissingName
	}

	path := fmt.Sprintf("/service/%s/version/%s/acl/%s", i.Service, i.Version, i.Name)
	resp, err := c.PutForm(path, i, nil)
	if err != nil {
		return nil, err
	}

	var b *ACL
	if err := decodeJSON(&b, resp.Body); err != nil {
		return nil, err
	}
	return b, nil
}

// DeleteACLInput is the input parameter to DeleteACL.
type DeleteACLInput struct {
	// Service is the ID of the service. Version is the specific configuration
	// version. Both fields are required.
	Service string
	Version string

	// Name is the name of the acl to delete (required).
	Name string
}

// DeleteACL deletes the given acl version.
func (c *Client) DeleteACL(i *DeleteACLInput) error {
	if i.Service == "" {
		return ErrMissingService
	}

	if i.Version == "" {
		return ErrMissingVersion
	}

	if i.Name == "" {
		return ErrMissingName
	}

	path := fmt.Sprintf("/service/%s/version/%s/acl/%s", i.Service, i.Version, i.Name)
	resp, err := c.Delete(path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Unlike other endpoints, the acl endpoint does not return a status
	// response - it just returns a 200 OK.
	return nil
}
