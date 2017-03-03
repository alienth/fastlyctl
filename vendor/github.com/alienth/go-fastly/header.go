package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type HeaderType int

const (
	_                            = iota
	HeaderTypeRequest HeaderType = iota
	HeaderTypeFetch
	HeaderTypeCache
	HeaderTypeResponse
)

func (s *HeaderType) UnmarshalText(b []byte) error {
	switch string(b) {
	case "request":
		*s = HeaderTypeRequest
	case "fetch":
		*s = HeaderTypeFetch
	case "cache":
		*s = HeaderTypeCache
	case "response":
		*s = HeaderTypeResponse
	}
	return nil
}

func (s *HeaderType) MarshalText() ([]byte, error) {
	switch *s {
	case HeaderTypeRequest:
		return []byte("request"), nil
	case HeaderTypeFetch:
		return []byte("fetch"), nil
	case HeaderTypeCache:
		return []byte("cache"), nil
	case HeaderTypeResponse:
		return []byte("response"), nil
	}
	return nil, nil
}

type HeaderAction int

const (
	_                            = iota
	HeaderActionSet HeaderAction = iota
	HeaderActionAppend
	HeaderActionDelete
	HeaderActionRegex
	HeaderActionRegexRepeat
)

func (s *HeaderAction) UnmarshalText(b []byte) error {
	switch string(b) {
	case "set":
		*s = HeaderActionSet
	case "append":
		*s = HeaderActionAppend
	case "delete":
		*s = HeaderActionDelete
	case "regex":
		*s = HeaderActionRegex
	case "regex_repeat":
		*s = HeaderActionRegexRepeat
	}
	return nil
}

func (s *HeaderAction) MarshalText() ([]byte, error) {
	switch *s {
	case HeaderActionSet:
		return []byte("set"), nil
	case HeaderActionAppend:
		return []byte("append"), nil
	case HeaderActionDelete:
		return []byte("delete"), nil
	case HeaderActionRegex:
		return []byte("regex"), nil
	case HeaderActionRegexRepeat:
		return []byte("regex_repeat"), nil
	}
	return nil, nil
}

type HeaderConfig config

type Header struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,string,omitempty"`

	Name              string       `json:"name,omitempty"`
	Action            HeaderAction `json:"action,omitempty"`
	CacheCondition    string       `json:"cache_condition"`
	IgnoreIfSet       Compatibool  `json:"ignore_if_set"`
	Priority          uint         `json:"priority,string,omitempty"`
	Regex             string       `json:"regex"`
	RequestCondition  string       `json:"request_condition"`
	ResponseCondition string       `json:"response_condition"`
	Source            string       `json:"src,omitempty"`
	Destination       string       `json:"dst,omitempty"`
	Substitution      string       `json:"substitution"`
	Type              HeaderType   `json:"type,omitempty"`
}

// headersByName is a sortable list of headers.
type headersByName []*Header

// Len, Swap, and Less implement the sortable interface.
func (s headersByName) Len() int      { return len(s) }
func (s headersByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s headersByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List headers for a specific service and version.
func (c *HeaderConfig) List(serviceID string, version uint) ([]*Header, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/header", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	headers := new([]*Header)
	resp, err := c.client.Do(req, headers)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(headersByName(*headers))

	return *headers, resp, nil
}

// Get fetches a specific header by name.
func (c *HeaderConfig) Get(serviceID string, version uint, name string) (*Header, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/header/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	header := new(Header)
	resp, err := c.client.Do(req, header)
	if err != nil {
		return nil, resp, err
	}
	return header, resp, nil
}

// Create a new header.
func (c *HeaderConfig) Create(serviceID string, version uint, header *Header) (*Header, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/header", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, header)
	if err != nil {
		return nil, nil, err
	}

	b := new(Header)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a header
func (c *HeaderConfig) Update(serviceID string, version uint, name string, header *Header) (*Header, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/header/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, header)
	if err != nil {
		return nil, nil, err
	}

	b := new(Header)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a header
func (c *HeaderConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/header/%s", serviceID, version, name)

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
