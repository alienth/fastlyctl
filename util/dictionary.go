package util

import "github.com/alienth/go-fastly"

type DictionaryItem struct {
	ServiceID    string
	DictionaryID string
	Key          string
	Value        string
	Client       *fastly.Client
}

type Dictionary struct {
	ServiceID    string
	DictionaryID string
	Client       *fastly.Client
}

func NewDictionaryItem(client *fastly.Client, serviceID, dictionaryName, key, value string) (*DictionaryItem, error) {
	service, err := GetServiceByNameOrID(client, serviceID)
	if err != nil {
		return nil, err
	}
	activeVersion, err := GetActiveVersion(service)
	if err != nil {
		return nil, err
	}

	dictionary, err := client.GetDictionary(&fastly.GetDictionaryInput{Service: service.ID, Version: activeVersion, Name: dictionaryName})
	if err != nil {
		return nil, err
	}

	return &DictionaryItem{ServiceID: service.ID, DictionaryID: dictionary.ID, Key: key, Value: value, Client: client}, nil
}

func (i *DictionaryItem) Add() error {
	var input fastly.CreateDictionaryItemInput
	input.Service = i.ServiceID
	input.Dictionary = i.DictionaryID
	input.ItemKey = i.Key
	input.ItemValue = i.Value
	if _, err := i.Client.CreateDictionaryItem(&input); err != nil {
		return err
	}
	return nil
}

func (i *DictionaryItem) Remove() error {
	var input fastly.DeleteDictionaryItemInput
	input.Service = i.ServiceID
	input.Dictionary = i.DictionaryID
	input.ItemKey = i.Key
	if err := i.Client.DeleteDictionaryItem(&input); err != nil {
		return err
	}
	return nil
}

func NewDictionary(client *fastly.Client, serviceID, dictionaryName string) (*Dictionary, error) {
	service, err := GetServiceByNameOrID(client, serviceID)
	if err != nil {
		return nil, err
	}
	activeVersion, err := GetActiveVersion(service)
	if err != nil {
		return nil, err
	}

	dictionary, err := client.GetDictionary(&fastly.GetDictionaryInput{Service: service.ID, Version: activeVersion, Name: dictionaryName})
	if err != nil {
		return nil, err
	}

	return &Dictionary{ServiceID: service.ID, DictionaryID: dictionary.ID, Client: client}, nil
}

func (d *Dictionary) List() ([]*fastly.DictionaryItem, error) {
	items, err := d.Client.ListDictionaryItems(&fastly.ListDictionaryItemsInput{Service: d.ServiceID, Dictionary: d.DictionaryID})
	if err != nil {
		return nil, err
	}
	return items, nil
}
