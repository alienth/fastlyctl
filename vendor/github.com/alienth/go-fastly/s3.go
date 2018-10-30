package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type S3Config config

// https://docs.fastly.com/api/logging#logging_s3
type S3 struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,string,omitempty"`

	Name              string      `json:"name,omitempty"`
	BucketName        string      `json:"bucket_name,omitempty"`
	Domain            string      `json:"domain"`
	AccessKey         string      `json:"access_key,omitempty"`
	SecretKey         string      `json:"secret_key,omitempty"`
	Path              string      `json:"path"`
	Period            uint        `json:"period,string,omitempty"`
	GzipLevel         uint        `json:"gzip_level,string"`
	Format            string      `json:"format"`
	ResponseCondition string      `json:"response_condition"`
	TimestampFormat   string      `json:"timestamp_format"`
	Redundancy        string      `json:"redundancy"`
	MessageType       MessageType `json:"message_type"`
}

type MessageType int

const (
	_                              = iota
	MessageTypeClassic MessageType = iota
	MessageTypeLoggly
	MessageTypeLogplex
	MessageTypeBlank
)

func (s *MessageType) UnmarshalText(b []byte) error {
	switch string(b) {
	case "classic":
		*s = MessageTypeClassic
	case "loggly":
		*s = MessageTypeLoggly
	case "logplex":
		*s = MessageTypeLogplex
	case "blank":
		*s = MessageTypeBlank
	}
	return nil
}

func (s *MessageType) MarshalText() ([]byte, error) {
	switch *s {
	case MessageTypeClassic:
		return []byte("classic"), nil
	case MessageTypeLoggly:
		return []byte("loggly"), nil
	case MessageTypeLogplex:
		return []byte("logplex"), nil
	case MessageTypeBlank:
		return []byte("blank"), nil
	}
	return nil, nil
}

// s3sByName is a sortable list of s3s.
type s3sByName []*S3

// Len, Swap, and Less implement the sortable interface.
func (s s3sByName) Len() int      { return len(s) }
func (s s3sByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s s3sByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List s3s for a specific service and version.
func (c *S3Config) List(serviceID string, version uint) ([]*S3, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/s3", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	s3s := new([]*S3)
	resp, err := c.client.Do(req, s3s)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(s3sByName(*s3s))

	return *s3s, resp, nil
}

// Get fetches a specific s3 by name.
func (c *S3Config) Get(serviceID string, version uint, name string) (*S3, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/s3/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	s3 := new(S3)
	resp, err := c.client.Do(req, s3)
	if err != nil {
		return nil, resp, err
	}
	return s3, resp, nil
}

// Create a new s3.
func (c *S3Config) Create(serviceID string, version uint, s3 *S3) (*S3, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/s3", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, s3)
	if err != nil {
		return nil, nil, err
	}

	b := new(S3)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a s3
func (c *S3Config) Update(serviceID string, version uint, name string, s3 *S3) (*S3, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/s3/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, s3)
	if err != nil {
		return nil, nil, err
	}

	b := new(S3)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a s3
func (c *S3Config) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/s3/%s", serviceID, version, name)

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
