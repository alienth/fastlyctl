package main

import (
	"fmt"
	"net"
	"os"

	"github.com/alienth/fastlyctl/util"
	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
)

var services []*fastly.Service
var client *fastly.Client

func main() {
	app := cli.NewApp()
	app.Name = "ban_ip"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "fastly-key, K",
			Usage:  "Fastly API Key. Can be read from 'fastly_key' file in CWD.",
			EnvVar: "FASTLY_KEY",
			Value:  util.GetFastlyKey(),
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "Print more detailed info for debugging.",
		},
		cli.BoolFlag{
			Name:  "assume-yes, y",
			Usage: "Assume 'yes' to all prompts. USE ONLY IF YOU ARE CERTAIN YOUR COMMANDS WON'T BREAK ANYTHING!",
		},
		cli.StringFlag{
			Name:  "dictionary, D",
			Usage: "The dictionary which we add the IP to.",
			Value: "banned_ips",
		},
		cli.StringSliceFlag{
			Name:  "service, s",
			Usage: "The service name which we're going to ban on. Can be specified multiple times. (default: all services which have the specified dictionary)",
		},
	}

	app.Before = func(c *cli.Context) error {
		if err := util.CheckFastlyKey(c); err != nil {
			return err
		}
		client = fastly.NewClient(nil, c.GlobalString("fastly-key"))

		serviceNames := c.GlobalStringSlice("service")

		services = make([]*fastly.Service, len(serviceNames))

		if len(serviceNames) == 0 {
			results, _, err := client.Service.List()
			if err != nil {
				return fmt.Errorf("Error fetching service list.")
			}
			services = results
		} else {
			for i, name := range serviceNames {
				service, err := util.GetServiceByName(client, name)
				if err != nil {
					return fmt.Errorf("Error fetching service %s: %s.", name, err)
				}
				services[i] = service
			}
		}
		return nil
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:   "ls",
			Usage:  "List banned addresses for each service",
			Action: banList,
		},
		cli.Command{
			Name:  "add",
			Usage: "Add one or more ADDRESSes to the ban list",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "comment, c",
					Usage: "Optional comment. Placed in the currently unused dictionary value.",
				},
			},
			ArgsUsage: "<ADDRESS>...",
			Action:    banAdd,
			Before:    validateAddresses,
		},
		cli.Command{
			Name:      "rm",
			ArgsUsage: "<ADDRESS>...",
			Usage:     "Remove one or more `ADDRESS`es from the ban list",
			Action:    banRemove,
			Before:    validateAddresses,
		},
	}

	app.Run(os.Args)
}

func validateAddresses(c *cli.Context) error {
	if c.NArg() == 0 {
		return cli.NewExitError("Specify at least one address.", -1)
	}

	for _, a := range c.Args() {
		if result := net.ParseIP(a); result == nil {
			return cli.NewExitError(fmt.Sprintf("%s is not a valid IP address.", a), -1)
		}
	}

	return nil
}

func banAdd(c *cli.Context) error {
	value := "1"
	if c.String("comment") != "" {
		value = c.String("comment")
	}

	for _, service := range services {
		activeVersion, err := util.GetActiveVersion(service)
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Error finding active version for service %s: %s\n", service.Name, err), -1)
		}
		dictionary, _, err := client.Dictionary.Get(service.ID, activeVersion, c.GlobalString("dictionary"))
		if err != nil {
			fmt.Printf("Unable to fetch dictionary %s on service %s. Skipping\n", c.GlobalString("dictionary"), service.Name)
			continue
		}

		for _, address := range c.Args() {
			item := new(fastly.DictionaryItem)
			item.Key = address
			item.Value = value
			_, _, err := client.DictionaryItem.Create(service.ID, dictionary.ID, item)
			if err != nil {
				return cli.NewExitError(fmt.Sprintf("Error adding item: %s\n", err), -1)
			}
			fmt.Printf("Added address %s to dictionary %s on service %s\n", address, c.GlobalString("dictionary"), service.Name)
		}
	}

	return nil
}

func banRemove(c *cli.Context) error {
	for _, service := range services {
		activeVersion, err := util.GetActiveVersion(service)
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Error finding active version for service %s: %s\n", service.Name, err), -1)
		}
		dictionary, _, err := client.Dictionary.Get(service.ID, activeVersion, c.GlobalString("dictionary"))
		if err != nil {
			fmt.Printf("Unable to fetch dictionary %s on service %s. Skipping\n", c.GlobalString("dictionary"), service.Name)
			continue
		}

		for _, address := range c.Args() {
			resp, err := client.DictionaryItem.Delete(service.ID, dictionary.ID, address)
			if err != nil {
				if resp.StatusCode == 404 {
					fmt.Printf("IP %s not found in dictionary %s on service %s. Skipping\n", address, c.GlobalString("dictionary"), service.Name)
					continue
				}
				return cli.NewExitError(fmt.Sprintf("Error removing item: %s\n", err), -1)
			}
			fmt.Printf("Removed address %s from dictionary %s on service %s\n", address, c.GlobalString("dictionary"), service.Name)
		}
	}

	return nil
}
func banList(c *cli.Context) error {
	for _, service := range services {
		activeVersion, err := util.GetActiveVersion(service)
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Error finding active version for service %s: %s\n", service.Name, err), -1)
		}
		dictionary, _, err := client.Dictionary.Get(service.ID, activeVersion, c.GlobalString("dictionary"))
		if err != nil {
			fmt.Printf("Unable to fetch dictionary %s on service %s. Skipping\n", c.GlobalString("dictionary"), service.Name)
			continue
		}
		items, _, err := client.DictionaryItem.List(service.ID, dictionary.ID)
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Error listing items: %s\n", err), -1)
		}
		fmt.Printf("Banned IP addresses for service %s:\n\n", service.Name)
		for _, i := range items {
			fmt.Println(i.Key, i.Value)
		}
		fmt.Println("")
	}
	return nil
}
