package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type SyslogConfig config

type Syslog struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,string,omitempty"`

	Name              string      `json:"name,omitempty"`
	Address           string      `json:"address,omitempty"`
	Port              uint        `json:"port,string,omitempty"`
	UseTLS            Compatibool `json:"use_tls"`
	TLSCACert         string      `json:"tls_ca_cert"`
	TLSHostname       string      `json:"tls_hostname"`
	Token             string      `json:"token"`
	Format            string      `json:"format"`
	ResponseCondition string      `json:"response_condition"`
}

// syslogsByName is a sortable list of syslogs.
type syslogsByName []*Syslog

// Len, Swap, and Less implement the sortable interface.
func (s syslogsByName) Len() int      { return len(s) }
func (s syslogsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s syslogsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List syslogs for a specific service and version.
func (c *SyslogConfig) List(serviceID string, version uint) ([]*Syslog, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/syslog", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	syslogs := new([]*Syslog)
	resp, err := c.client.Do(req, syslogs)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(syslogsByName(*syslogs))

	return *syslogs, resp, nil
}

// Get fetches a specific syslog by name.
func (c *SyslogConfig) Get(serviceID string, version uint, name string) (*Syslog, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/syslog/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	syslog := new(Syslog)
	resp, err := c.client.Do(req, syslog)
	if err != nil {
		return nil, resp, err
	}
	return syslog, resp, nil
}

// Create a new syslog.
func (c *SyslogConfig) Create(serviceID string, version uint, syslog *Syslog) (*Syslog, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/syslog", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, syslog)
	if err != nil {
		return nil, nil, err
	}

	b := new(Syslog)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a syslog
func (c *SyslogConfig) Update(serviceID string, version uint, name string, syslog *Syslog) (*Syslog, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/syslog/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, syslog)
	if err != nil {
		return nil, nil, err
	}

	b := new(Syslog)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a syslog
func (c *SyslogConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/logging/syslog/%s", serviceID, version, name)

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
