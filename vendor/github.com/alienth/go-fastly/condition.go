package fastly

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type ConditionType int

const (
	// Don't use the zero-value so that json's omitempty won't ignore a
	// real value.
	_                                  = iota
	ConditionTypeRequest ConditionType = iota
	ConditionTypeResponse
	ConditionTypeCache
)

func (s *ConditionType) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "request":
		*s = ConditionTypeRequest
	case "response":
		*s = ConditionTypeResponse
	case "cache":
		*s = ConditionTypeCache
	}
	return nil
}

func (s *ConditionType) MarshalText() ([]byte, error) {
	switch *s {
	case ConditionTypeRequest:
		return []byte("REQUEST"), nil
	case ConditionTypeResponse:
		return []byte("RESPONSE"), nil
	case ConditionTypeCache:
		return []byte("CACHE"), nil
	}
	return nil, nil
}

type ConditionConfig config

type Condition struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,string,omitempty"`

	Name      string        `json:"name,omitempty"`
	Statement string        `json:"statement,omitempty"`
	Type      ConditionType `json:"type,omitempty"`
	Comment   string        `json:"comment,omitempty"`

	// When you create a Condition, you get an int priority. When you list
	// Conditions, you get string Priorities (quoted)
	Priority uint `json:"priority,string,omitempty"`
}

// conditionsByName is a sortable list of conditions.
type conditionsByName []*Condition

// Len, Swap, and Less implement the sortable interface.
func (s conditionsByName) Len() int      { return len(s) }
func (s conditionsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s conditionsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List conditions for a specific service and version.
func (c *ConditionConfig) List(serviceID string, version uint) ([]*Condition, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/condition", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	conditions := new([]*Condition)
	resp, err := c.client.Do(req, conditions)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(conditionsByName(*conditions))

	return *conditions, resp, nil
}

// Get fetches a specific condition by name.
func (c *ConditionConfig) Get(serviceID string, version uint, name string) (*Condition, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/condition/%s", serviceID, version, url.PathEscape(name))

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	condition := new(Condition)
	resp, err := c.client.Do(req, condition)
	if err != nil {
		return nil, resp, err
	}
	return condition, resp, nil
}

// Create a new cache condition.
func (c *ConditionConfig) Create(serviceID string, version uint, condition *Condition) (*Condition, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/condition", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, condition)
	if err != nil {
		return nil, nil, err
	}

	b := new(Condition)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a cache condition
func (c *ConditionConfig) Update(serviceID string, version uint, name string, condition *Condition) (*Condition, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/condition/%s", serviceID, version, url.PathEscape(name))

	req, err := c.client.NewJSONRequest("PUT", u, condition)
	if err != nil {
		return nil, nil, err
	}

	b := new(Condition)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a cache condition
func (c *ConditionConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/condition/%s", serviceID, version, url.PathEscape(name))

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
