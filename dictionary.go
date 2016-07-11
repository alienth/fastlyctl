package main

import (
	"fmt"

	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
)

func dictionaryList(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	serviceParam := c.Args().Get(0)
	var service *fastly.Service
	if service, err = getServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	activeVersion, err := getActiveVersion(service)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	dictionaries, err := client.ListDictionaries(&fastly.ListDictionariesInput{Service: service.ID, Version: activeVersion})
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
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	serviceParam := c.Args().Get(0)
	dictParam := c.Args().Get(1)
	keyParam := c.Args().Get(2)
	valueParam := c.Args().Get(3)
	var service *fastly.Service
	if service, err = getServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	activeVersion, err := getActiveVersion(service)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	dictionary, err := client.GetDictionary(&fastly.GetDictionaryInput{Service: service.ID, Version: activeVersion, Name: dictParam})
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	var i fastly.CreateDictionaryItemInput
	i.Service = service.ID
	i.Dictionary = dictionary.ID
	i.ItemKey = keyParam
	i.ItemValue = valueParam
	if _, err = client.CreateDictionaryItem(&i); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func dictionaryRemoveItem(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	serviceParam := c.Args().Get(0)
	dictParam := c.Args().Get(1)
	keyParam := c.Args().Get(2)
	var service *fastly.Service
	if service, err = getServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	activeVersion, err := getActiveVersion(service)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	dictionary, err := client.GetDictionary(&fastly.GetDictionaryInput{Service: service.ID, Version: activeVersion, Name: dictParam})
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	var i fastly.DeleteDictionaryItemInput
	i.Service = service.ID
	i.Dictionary = dictionary.ID
	i.ItemKey = keyParam
	if err = client.DeleteDictionaryItem(&i); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func dictionaryListItems(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	serviceParam := c.Args().Get(0)
	dictParam := c.Args().Get(1)
	var service *fastly.Service
	if service, err = getServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	activeVersion, err := getActiveVersion(service)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	dictionary, err := client.GetDictionary(&fastly.GetDictionaryInput{Service: service.ID, Version: activeVersion, Name: dictParam})
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	items, err := client.ListDictionaryItems(&fastly.ListDictionaryItemsInput{Service: service.ID, Dictionary: dictionary.ID})
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	fmt.Printf("Items in dictionary %s for service %s:\n\n", dictionary.Name, service.Name)
	for _, item := range items {
		fmt.Println(item.ItemKey, item.ItemValue)
	}

	return nil
}
