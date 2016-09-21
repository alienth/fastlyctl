package fastly

import (
	"fmt"
	"sort"
)

// ACLEntry represents a acl entry response from the Fastly API.
type ACLEntry struct {
	ServiceID string `mapstructure:"service_id"`
	ACLID     string `mapstructure:"acl_id"`

	ID      string `mapstructure:"id"`
	IP      string `mapstructure:"ip"`
	Subnet  uint8  `mapstructure:"subnet"`
	Comment string `mapstructure:"comment"`
	Negated bool   `mapstructure:"negated"`
}

// aclEntriesByKey is a sortable list of acl entries.
type aclEntriesByKey []*ACLEntry

// Len, Swap, and Less implement the sortable interface.
func (s aclEntriesByKey) Len() int      { return len(s) }
func (s aclEntriesByKey) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s aclEntriesByKey) Less(i, j int) bool {
	return s[i].IP < s[j].IP
}

// ListACLEntriesInput is used as input to the ListACLEntries function.
type ListACLEntriesInput struct {
	// Service is the ID of the service (required).
	Service string

	// ACL is the ID of the acl to retrieve entries for (required).
	ACL string
}

// ListACLEntries returns the list of acl entries for the
// configuration version.
func (c *Client) ListACLEntries(i *ListACLEntriesInput) ([]*ACLEntry, error) {
	if i.Service == "" {
		return nil, ErrMissingService
	}

	if i.ACL == "" {
		return nil, ErrMissingACL
	}

	path := fmt.Sprintf("/service/%s/acl/%s/entries", i.Service, i.ACL)
	resp, err := c.Get(path, nil)
	if err != nil {
		return nil, err
	}

	var bs []*ACLEntry
	if err := decodeJSON(&bs, resp.Body); err != nil {
		return nil, err
	}
	sort.Stable(aclEntriesByKey(bs))
	return bs, nil
}

// CreateACLEntryInput is used as input to the CreateACLEntry function.
type CreateACLEntryInput struct {
	// Service is the ID of the service. ACL is the ID of the acl.
	// Both fields are required.
	Service string
	ACL     string

	IP      string      `form:"ip,omitempty"`
	Subnet  uint8       `form:"subnet,omitempty"`
	Comment string      `form:"comment,omitempty"`
	Negated Compatibool `form:"negated,omitempty"`
}

// CreateACLEntry creates a new Fastly acl entry.
func (c *Client) CreateACLEntry(i *CreateACLEntryInput) (*ACLEntry, error) {
	if i.Service == "" {
		return nil, ErrMissingService
	}

	if i.ACL == "" {
		return nil, ErrMissingACL
	}

	path := fmt.Sprintf("/service/%s/acl/%s/entry", i.Service, i.ACL)
	resp, err := c.PostForm(path, i, nil)
	if err != nil {
		return nil, err
	}

	var b *ACLEntry
	if err := decodeJSON(&b, resp.Body); err != nil {
		return nil, err
	}
	return b, nil
}

// GetACLEntryInput is used as input to the GetACLEntry function.
type GetACLEntryInput struct {
	// Service is the ID of the service. ACL is the ID of the acl.
	// Both fields are required.
	Service string
	ACL     string

	// ID is the ID of the entry
	ID string
}

// GetACLEntry gets the acl entry with the given parameters.
func (c *Client) GetACLEntry(i *GetACLEntryInput) (*ACLEntry, error) {
	if i.Service == "" {
		return nil, ErrMissingService
	}

	if i.ACL == "" {
		return nil, ErrMissingACL
	}

	if i.ID == "" {
		return nil, ErrMissingACLEntryID
	}

	path := fmt.Sprintf("/service/%s/acl/%s/entry/%s", i.Service, i.ACL, i.ID)
	resp, err := c.Get(path, nil)
	if err != nil {
		return nil, err
	}

	var b *ACLEntry
	if err := decodeJSON(&b, resp.Body); err != nil {
		return nil, err
	}
	return b, nil
}

// UpdateACLEntryInput is used as input to the UpdateACLEntry function.
type UpdateACLEntryInput struct {
	// Service is the ID of the service. ACL is the ID of the acl.
	// Both fields are required.
	Service string
	ACL     string

	// ID is the id of the acl entry to fetch.
	ID string

	IP      string      `form:"ip,omitempty"`
	Subnet  uint8       `form:"subnet,omitempty"`
	Comment string      `form:"comment,omitempty"`
	Negated Compatibool `form:"negated,omitempty"`
}

// UpdateACLEntry updates a specific acl entry.
func (c *Client) UpdateACLEntry(i *UpdateACLEntryInput) (*ACLEntry, error) {
	if i.Service == "" {
		return nil, ErrMissingService
	}

	if i.ACL == "" {
		return nil, ErrMissingACL
	}

	if i.ID == "" {
		return nil, ErrMissingACLEntryID
	}

	path := fmt.Sprintf("/service/%s/acl/%s/entry/%s", i.Service, i.ACL, i.ID)
	resp, err := c.PutForm(path, i, nil)
	if err != nil {
		return nil, err
	}

	var b *ACLEntry
	if err := decodeJSON(&b, resp.Body); err != nil {
		return nil, err
	}
	return b, nil
}

// DeleteACLEntryInput is the input parameter to DeleteACLEntry.
type DeleteACLEntryInput struct {
	// Service is the ID of the service. ACL is the ID of the acl.
	// Both fields are required.
	Service string
	ACL     string

	// ID is the id of the acl entry to delete.
	ID string
}

// DeleteACLEntry deletes the given acl entry.
func (c *Client) DeleteACLEntry(i *DeleteACLEntryInput) error {
	if i.Service == "" {
		return ErrMissingService
	}

	if i.ACL == "" {
		return ErrMissingACL
	}

	if i.ID == "" {
		return ErrMissingACLEntryID
	}

	path := fmt.Sprintf("/service/%s/acl/%s/entry/%s", i.Service, i.ACL, i.ID)
	resp, err := c.Delete(path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Unlike other endpoints, the acl endpoint does not return a status
	// response - it just returns a 200 OK.
	return nil
}
