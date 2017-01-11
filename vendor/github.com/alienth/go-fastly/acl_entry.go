package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type ACLEntryConfig config

type ACLEntry struct {
	// Non-writable
	ServiceID string `json:"service_id"`
	ID        string `json:"id"`
	ACLID     string `json:"acl_id"`

	// writable
	IP      string      `json:"ip"`
	Subnet  uint8       `json:"subnet"`
	Comment string      `json:"comment"`
	Negated Compatibool `json:"negated"`
}

// aclEntriesByName is a sortable list of aclEntries.
type aclEntriesByIP []*ACLEntry

// Len, Swap, and Less implement the sortable interface.
func (s aclEntriesByIP) Len() int      { return len(s) }
func (s aclEntriesByIP) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s aclEntriesByIP) Less(i, j int) bool {
	return s[i].IP < s[j].IP
}

// List aclEntries for a specific ACL and service.
func (c *ACLEntryConfig) List(serviceID, aclID string) ([]*ACLEntry, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/acl/%s/entries", serviceID, aclID)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	aclEntries := new([]*ACLEntry)
	resp, err := c.client.Do(req, aclEntries)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(aclEntriesByIP(*aclEntries))

	return *aclEntries, resp, nil
}

// Get fetches a specific aclEntry by entryID.
func (c *ACLEntryConfig) Get(serviceID, aclID, entryID string) (*ACLEntry, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/acl/%s/entry/%s", serviceID, aclID, entryID)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	aclEntry := new(ACLEntry)
	resp, err := c.client.Do(req, aclEntry)
	if err != nil {
		return nil, resp, err
	}
	return aclEntry, resp, nil
}

// Create a new aclEntry.
func (c *ACLEntryConfig) Create(serviceID, aclID string, aclEntry *ACLEntry) (*ACLEntry, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/acl/%s/entry", serviceID, aclID)

	req, err := c.client.NewJSONRequest("POST", u, aclEntry)
	if err != nil {
		return nil, nil, err
	}

	b := new(ACLEntry)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a aclEntry
func (c *ACLEntryConfig) Update(serviceID, aclID, entryID string, aclEntry *ACLEntry) (*ACLEntry, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/acl/%s/entry/%s", serviceID, aclID, entryID)

	req, err := c.client.NewJSONRequest("PATCH", u, aclEntry)
	if err != nil {
		return nil, nil, err
	}

	b := new(ACLEntry)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a aclEntry
func (c *ACLEntryConfig) Delete(serviceID, aclID, entryID string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/acl/%s/entry/%s", serviceID, aclID, entryID)

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

type ACLEntryBatchUpdate struct {
	Entries []ACLEntryUpdate `json:"entries"`
}

type ACLEntryUpdate struct {
	Operation BatchOperation `json:"op,omitempty"`
	ID        string         `json:"id,omitempty"`
	IP        string         `json:"ip,omitempty"`
	Subnet    string         `json:"subnet,omitempty"`
	Comment   string         `json:"comment,omitempty"`
}

func (c *ACLEntryConfig) BatchUpdate(serviceID, aclID string, entries []ACLEntryUpdate) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/acl/%s/entries", serviceID, aclID)

	var update ACLEntryBatchUpdate
	update.Entries = entries
	req, err := c.client.NewJSONRequest("PATCH", u, update)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}
