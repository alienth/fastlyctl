package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type CacheSettingAction int

const (
	_                                          = iota
	CacheSettingActionCache CacheSettingAction = iota
	CacheSettingActionPass
	CacheSettingActionRestart
)

func (s *CacheSettingAction) UnmarshalText(b []byte) error {
	switch string(b) {
	case "pass":
		*s = CacheSettingActionPass
	case "cache":
		*s = CacheSettingActionCache
	case "restart":
		*s = CacheSettingActionRestart
	}
	return nil
}

func (s *CacheSettingAction) MarshalText() ([]byte, error) {
	switch *s {
	case CacheSettingActionPass:
		return []byte("pass"), nil
	case CacheSettingActionCache:
		return []byte("cache"), nil
	case CacheSettingActionRestart:
		return []byte("restart"), nil
	}
	return nil, nil
}

type CacheSettingConfig config

type CacheSetting struct {
	ServiceID string `json:"service_id,omitempty"`
	Version   uint   `json:"version,string,omitempty"`

	Name           string             `json:"name,omitempty"`
	Action         CacheSettingAction `json:"action,omitempty"`
	CacheCondition string             `json:"cache_condition"`
	StaleTTL       uint               `json:"stale_ttl,string"`
	TTL            uint               `json:"ttl,string"`
}

// cacheSettingsByName is a sortable list of cacheSettings.
type cacheSettingsByName []*CacheSetting

// Len, Swap, and Less implement the sortable interface.
func (s cacheSettingsByName) Len() int      { return len(s) }
func (s cacheSettingsByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s cacheSettingsByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List cacheSettings for a specific service and version.
func (c *CacheSettingConfig) List(serviceID string, version uint) ([]*CacheSetting, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/cache_settings", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	cacheSettings := new([]*CacheSetting)
	resp, err := c.client.Do(req, cacheSettings)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(cacheSettingsByName(*cacheSettings))

	return *cacheSettings, resp, nil
}

// Get fetches a specific cache setting by name.
func (c *CacheSettingConfig) Get(serviceID string, version uint, name string) (*CacheSetting, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/cache_settings/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	setting := new(CacheSetting)
	resp, err := c.client.Do(req, setting)
	if err != nil {
		return nil, resp, err
	}
	return setting, resp, nil
}

// Create a new cache setting.
func (c *CacheSettingConfig) Create(serviceID string, version uint, setting *CacheSetting) (*CacheSetting, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/cache_settings", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, setting)
	if err != nil {
		return nil, nil, err
	}

	b := new(CacheSetting)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a cache setting
func (c *CacheSettingConfig) Update(serviceID string, version uint, name string, setting *CacheSetting) (*CacheSetting, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/cache_settings/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, setting)
	if err != nil {
		return nil, nil, err
	}

	b := new(CacheSetting)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a cache setting
func (c *CacheSettingConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/cache_settings/%s", serviceID, version, name)

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
