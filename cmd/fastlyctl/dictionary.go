package main

import (
	"fmt"

	"github.com/alienth/fastlyctl/util"
	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
)

func dictionaryList(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))

	var err error
	serviceParam := c.Args().Get(0)
	var service *fastly.Service
	if service, _, err = client.Service.Search(serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	activeVersion, err := util.GetActiveVersion(service)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	dictionaries, _, err := client.Dictionary.List(service.ID, activeVersion)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Unable to list dictionaries for service %s\n", service.Name), -1)
	}
	fmt.Printf("Dictionaries for %s:\n\n", service.Name)
	for _, d := range dictionaries {
		fmt.Println(d.Name)
	}
	return nil
}

func dictionaryAddItem(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))

	serviceParam := c.Args().Get(0)
	dictParam := c.Args().Get(1)
	keyParam := c.Args().Get(2)
	valueParam := c.Args().Get(3)

	dictionary, err := util.GetDictionaryByName(client, serviceParam, dictParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	item := new(fastly.DictionaryItem)
	item.Key = keyParam
	item.Value = valueParam

	if _, _, err = client.DictionaryItem.Create(dictionary.ServiceID, dictionary.ID, item); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func dictionaryRemoveItem(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))

	serviceParam := c.Args().Get(0)
	dictParam := c.Args().Get(1)
	keyParam := c.Args().Get(2)

	dictionary, err := util.GetDictionaryByName(client, serviceParam, dictParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	if _, err = client.DictionaryItem.Delete(dictionary.ServiceID, dictionary.ID, keyParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func dictionaryListItems(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))

	serviceParam := c.Args().Get(0)
	dictParam := c.Args().Get(1)

	dictionary, err := util.GetDictionaryByName(client, serviceParam, dictParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	items, _, err := client.DictionaryItem.List(dictionary.ServiceID, dictionary.ID)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	fmt.Printf("Items in dictionary %s for service %s:\n\n", dictParam, serviceParam)
	for _, item := range items {
		fmt.Println(item.Key, item.Value)
	}

	return nil
}
