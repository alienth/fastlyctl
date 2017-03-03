package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type BackendConfig config

type Backend struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,omitempty"`

	Name                string `json:"name,omitempty"`
	Port                uint   `json:"port,omitempty"`
	ConnectTimeout      uint   `json:"connect_timeout,omitempty"`
	MaxConn             uint   `json:"max_conn,omitempty"`
	ErrorThreshold      uint   `json:"error_threshold"`
	FirstByteTimeout    uint   `json:"first_byte_timeout,omitempty"`
	BetweenBytesTimeout uint   `json:"between_bytes_timeout,omitempty"`
	AutoLoadbalance     bool   `json:"auto_loadbalance"`
	Weight              uint   `json:"weight,omitempty"`
	RequestCondition    string `json:"request_condition"`
	HealthCheck         string `json:"healthcheck"`
	UseSSL              bool   `json:"use_ssl"`
	SSLCheckCert        bool   `json:"ssl_check_cert"`
	SSLCertHostname     string `json:"ssl_cert_hostname"`
	SSLSNIHostname      string `json:"ssl_sni_hostname"`

	// These attributes are all related. Do not zero them out.
	Address  string `json:"address,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	IPV4     string `json:"ipv4,omitempty"` // TODO net.IP type these.
	IPV6     string `json:"ipv6,omitempty"`

	// These cannot be set to ''
	SSLHostname   string `json:"ssl_hostname,omitempty"`
	SSLCiphers    string `json:"ssl_ciphers,omitempty"`
	MinTLSVersion string `json:"min_tls_version,omitempty"`
	MaxTLSVersion string `json:"max_tls_version,omitempty"`

	// Somehow different?
	Shield string `json:"shield"`
}

// backendsByName is a sortable list of backends.
type backendsByName []*Backend

// Len, Swap, and Less implement the sortable interface.
func (s backendsByName) Len() int      { return len(s) }
func (s backendsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s backendsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List backends for a specific service and version.
func (c *BackendConfig) List(serviceID string, version uint) ([]*Backend, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/backend", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	backends := new([]*Backend)
	resp, err := c.client.Do(req, backends)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(backendsByName(*backends))

	return *backends, resp, nil
}

// Get fetches a specific backend by name.
func (c *BackendConfig) Get(serviceID string, version uint, name string) (*Backend, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/backend/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	backend := new(Backend)
	resp, err := c.client.Do(req, backend)
	if err != nil {
		return nil, resp, err
	}
	return backend, resp, nil
}

// Create a new backend.
func (c *BackendConfig) Create(serviceID string, version uint, backend *Backend) (*Backend, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/backend", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, backend)
	if err != nil {
		return nil, nil, err
	}

	b := new(Backend)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a backend
func (c *BackendConfig) Update(serviceID string, version uint, name string, backend *Backend) (*Backend, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/backend/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, backend)
	if err != nil {
		return nil, nil, err
	}

	b := new(Backend)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a backend
func (c *BackendConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/backend/%s", serviceID, version, name)

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
