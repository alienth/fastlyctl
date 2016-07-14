package main

import (
	"fmt"

	"github.com/alienth/fastlyctl/util"
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
	if service, err = util.GetServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	activeVersion, err := util.GetActiveVersion(service)
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

	item, err := util.NewDictionaryItem(client, serviceParam, dictParam, keyParam, valueParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	if err := item.Add(); err != nil {
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

	item, err := util.NewDictionaryItem(client, serviceParam, dictParam, keyParam, "")
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	if err := item.Remove(); err != nil {
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
	dictionary, err := util.NewDictionary(client, serviceParam, dictParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	items, err := dictionary.List()
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	fmt.Printf("Items in dictionary %s for service %s:\n\n", dictParam, serviceParam)
	for _, item := range items {
		fmt.Println(item.ItemKey, item.ItemValue)
	}

	return nil
}
