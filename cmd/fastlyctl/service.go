package main

import (
	"fmt"

	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
)

func serviceList(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	services, err := client.ListServices(&fastly.ListServicesInput{})
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error listing services: %s", err), -1)
	}
	fmt.Printf("%25s %8s  %s\n", "ID", "Version", "Name")
	for _, s := range services {
		fmt.Printf("%25s %8d  %s\n", s.ID, s.ActiveVersion, s.Name)
	}

	return nil
}
