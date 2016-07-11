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
