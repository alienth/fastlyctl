package main

import (
	"fmt"

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
	if service, err = getServiceByNameOrID(client, serviceParam); err != nil {
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
	if service, err = getServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	result, msg, err := client.ValidateVersion(&fastly.ValidateVersionInput{Service: service.ID, Version: version})
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error validating version: %s", err), -1)
	}
	if result {
		fmt.Printf("Version %s on service %s successfully validated!\n", version, service.Name)
	} else {
		return cli.NewExitError(fmt.Sprintf("Version %s on service %s is invalid:\n\n%s", version, service.Name, msg), -1)
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
	if service, err = getServiceByNameOrID(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	var version *fastly.Version
	if version, err = client.GetVersion(&fastly.GetVersionInput{Service: service.ID, Version: versionNumber}); err != nil {
		return cli.NewExitError(fmt.Sprintf("Error fetching version: %s", err), -1)
	}
	if err = activateVersion(c, client, service, version); err != nil {
		return cli.NewExitError(fmt.Sprintf("Error activating version: %s", err), -1)
	}

	return nil
}
