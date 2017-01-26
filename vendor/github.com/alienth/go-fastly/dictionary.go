package fastly

import (
	"fmt"
	"net/http"
	"sort"
)

type DictionaryConfig config

type Dictionary struct {
	ServiceID string `json:"service_id"`
	Version   uint   `json:"version"`
	ID        string `json:"id"`

	Name string `json:"name" url:"name,omitempty"`
}

// dictionariesByName is a sortable list of dictionaries.
type dictionariesByName []*Dictionary

// Len, Swap, and Less implement the sortable interface.
func (s dictionariesByName) Len() int      { return len(s) }
func (s dictionariesByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s dictionariesByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// List dictionaries for a specific service and version.
func (c *DictionaryConfig) List(serviceID string, version uint) ([]*Dictionary, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/dictionary", serviceID, version)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	dictionaries := new([]*Dictionary)
	resp, err := c.client.Do(req, dictionaries)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(dictionariesByName(*dictionaries))

	return *dictionaries, resp, nil
}

// Get fetches a specific dictionary by name.
func (c *DictionaryConfig) Get(serviceID string, version uint, name string) (*Dictionary, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/dictionary/%s", serviceID, version, name)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	dictionary := new(Dictionary)
	resp, err := c.client.Do(req, dictionary)
	if err != nil {
		return nil, resp, err
	}
	return dictionary, resp, nil
}

// Create a new dictionary.
func (c *DictionaryConfig) Create(serviceID string, version uint, dictionary *Dictionary) (*Dictionary, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/dictionary", serviceID, version)

	req, err := c.client.NewJSONRequest("POST", u, dictionary)
	if err != nil {
		return nil, nil, err
	}

	b := new(Dictionary)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a dictionary
func (c *DictionaryConfig) Update(serviceID string, version uint, name string, dictionary *Dictionary) (*Dictionary, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/dictionary/%s", serviceID, version, name)

	req, err := c.client.NewJSONRequest("PUT", u, dictionary)
	if err != nil {
		return nil, nil, err
	}

	b := new(Dictionary)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a dictionary
func (c *DictionaryConfig) Delete(serviceID string, version uint, name string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/version/%d/dictionary/%s", serviceID, version, name)

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
