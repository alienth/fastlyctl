package fastly

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

type DictionaryItemConfig config

type DictionaryItem struct {
	// Non-writable
	ServiceID    string `json:"service_id"`
	Version      uint   `json:"version,string"`
	DictionaryID string `json:"dictionary_id"`

	// writable
	Key   string `json:"item_key,omitempty"`
	Value string `json:"item_value,omitempty"`
}

// dictionaryItemsByName is a sortable list of dictionaryItems.
type dictionaryItemsByKey []*DictionaryItem

// Len, Swap, and Less implement the sortable interface.
func (s dictionaryItemsByKey) Len() int      { return len(s) }
func (s dictionaryItemsByKey) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s dictionaryItemsByKey) Less(i, j int) bool {
	return s[i].Key < s[j].Key
}

// List dictionaryItems for a specific Dictionary and service.
func (c *DictionaryItemConfig) List(serviceID, dictionaryID string) ([]*DictionaryItem, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/dictionary/%s/items", serviceID, dictionaryID)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	dictionaryItems := new([]*DictionaryItem)
	resp, err := c.client.Do(req, dictionaryItems)
	if err != nil {
		return nil, resp, err
	}

	sort.Stable(dictionaryItemsByKey(*dictionaryItems))

	return *dictionaryItems, resp, nil
}

// Get fetches a specific dictionary item by key.
func (c *DictionaryItemConfig) Get(serviceID, dictionaryID, key string) (*DictionaryItem, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/dictionary/%s/item/%s", serviceID, dictionaryID, key)

	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	item := new(DictionaryItem)
	resp, err := c.client.Do(req, item)
	if err != nil {
		return nil, resp, err
	}
	return item, resp, nil
}

// Create a new dictionary item.
func (c *DictionaryItemConfig) Create(serviceID, dictionaryID string, item *DictionaryItem) (*DictionaryItem, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/dictionary/%s/item", serviceID, dictionaryID)

	req, err := c.client.NewJSONRequest("POST", u, item)
	if err != nil {
		return nil, nil, err
	}

	b := new(DictionaryItem)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Update a dictionary item
func (c *DictionaryItemConfig) Update(serviceID, dictionaryID, key string, item *DictionaryItem) (*DictionaryItem, *http.Response, error) {
	u := fmt.Sprintf("/service/%s/dictionary/%s/item/%s", serviceID, dictionaryID, key)

	req, err := c.client.NewJSONRequest("PATCH", u, item)
	if err != nil {
		return nil, nil, err
	}

	b := new(DictionaryItem)
	resp, err := c.client.Do(req, b)
	if err != nil {
		return nil, resp, err
	}

	return b, resp, nil
}

// Delete a dictionary item
func (c *DictionaryItemConfig) Delete(serviceID, dictionaryID, key string) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/dictionary/%s/item/%s", serviceID, dictionaryID, key)

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

type DictionaryItemBatchUpdate struct {
	Items []DictionaryItemUpdate `json:"items"`
}

type DictionaryItemUpdate struct {
	Operation BatchOperation `json:"op,omitempty"`
	Key       string         `json:"item_key,omitempty"`
	Value     string         `json:"item_value,omitempty"`
}

func (c *DictionaryItemConfig) BatchUpdate(serviceID, dictionaryID string, items []DictionaryItemUpdate) (*http.Response, error) {
	u := fmt.Sprintf("/service/%s/dictionary/%s/items", serviceID, dictionaryID)

	var update DictionaryItemBatchUpdate
	update.Items = items
	data, _ := json.Marshal(update)
	fmt.Println(string(data))
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
