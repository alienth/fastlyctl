package main

import (
	"fmt"

	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
)

func serviceList(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))

	services, _, err := client.Service.List()
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error listing services: %s", err), -1)
	}
	fmt.Printf("%25s %8s  %s\n", "ID", "Version", "Name")
	for _, s := range services {
		fmt.Printf("%25s %8d  %s\n", s.ID, s.Version, s.Name)
	}

	return nil
}
