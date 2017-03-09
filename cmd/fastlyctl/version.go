package main

import (
	"fmt"
	"strconv"

	"github.com/alienth/fastlyctl/util"
	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
)

func versionList(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))
	serviceParam := c.Args().Get(0)
	service, err := util.GetServiceByName(client, serviceParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	fmt.Printf("Versions for %s:\n\n", service.Name)
	fmt.Printf("%5s %-27s %-27s %s\n", "ID", "Created", "Updated", "Comment")
	for _, version := range service.Versions {
		active := ""
		if version.Active {
			active = "*"
		}
		fmt.Printf("%2s %4d %-27s %-27s %s\n", active, version.Number, version.Created, version.Updated, version.Comment)
	}

	return nil
}

func versionValidate(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))
	serviceParam := c.Args().Get(0)
	version, err := strconv.Atoi(c.Args().Get(1))
	if err != nil {
		return cli.NewExitError("Invalid version number.\n", -1)
	}

	var service *fastly.Service
	if service, err = util.GetServiceByName(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	if err := util.ValidateVersion(client, service, uint(version)); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func versionActivate(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))
	serviceParam := c.Args().Get(0)
	version, err := strconv.Atoi(c.Args().Get(1))
	if err != nil {
		return cli.NewExitError("Invalid version number.\n", -1)
	}

	var service *fastly.Service
	if service, err = util.GetServiceByName(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	if _, _, err = client.Version.Activate(service.ID, uint(version)); err != nil {
		return cli.NewExitError(fmt.Sprintf("Error activating version: %s", err), -1)
	} else {
		fmt.Printf("Version %d on service %s successfully activated!\n", version, serviceParam)
	}

	return nil
}
