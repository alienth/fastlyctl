package main

import (
	"fmt"

	"github.com/alienth/fastlyctl/util"
	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
)

func versionList(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}
	serviceParam := c.Args().Get(0)
	var service *fastly.Service
	if service, err = util.GetServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	fmt.Printf("Versions for %s:\n\n", service.Name)
	fmt.Printf("%5s %-27s %-27s %s\n", "ID", "Created", "Updated", "Comment")
	for _, version := range service.Versions {
		active := ""
		if version.Active {
			active = "* "
		}
		fmt.Printf("%5s %-27s %-27s %s\n", active+version.Number, version.Created, version.Updated, version.Comment)
	}

	return nil
}

func versionValidate(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}
	serviceParam := c.Args().Get(0)
	version := c.Args().Get(1)
	var service *fastly.Service
	if service, err = util.GetServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	if err := util.ValidateVersion(client, service, version); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func versionActivate(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}
	serviceParam := c.Args().Get(0)
	versionNumber := c.Args().Get(1)
	var service *fastly.Service
	if service, err = util.GetServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	var version *fastly.Version
	if version, err = client.GetVersion(&fastly.GetVersionInput{Service: service.ID, Version: versionNumber}); err != nil {
		return cli.NewExitError(fmt.Sprintf("Error fetching version: %s", err), -1)
	}
	if err = util.ActivateVersion(c, client, service, version); err != nil {
		return cli.NewExitError(fmt.Sprintf("Error activating version: %s", err), -1)
	}

	return nil
}
