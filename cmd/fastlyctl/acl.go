package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alienth/fastlyctl/util"
	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
)

func aclList(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	serviceParam := c.Args().Get(0)
	var service *fastly.Service
	if service, err = util.GetServiceByName(client, serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	activeVersion, err := util.GetActiveVersion(service)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	acls, err := client.ListACLs(&fastly.ListACLsInput{Service: service.ID, Version: activeVersion})
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Unable to list ACLs for service %s\n", service.Name), -1)
	}
	fmt.Printf("ACLs for %s:\n\n", service.Name)
	for _, d := range acls {
		fmt.Println(d.Name)
	}
	return nil
}

func ipMaskSplit(ipParam string) (string, uint8, error) {
	var subnet uint8
	ipSplit := strings.Split(ipParam, "/")
	if len(ipSplit) == 2 {
		s, err := strconv.Atoi(ipSplit[1])
		if err != nil {
			return "", 0, fmt.Errorf("Invalid subnet mask specified: %s", err)
		}
		subnet = uint8(s)
	}
	return ipSplit[0], subnet, nil
}

func aclAddEntry(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	serviceParam := c.Args().Get(0)
	aclParam := c.Args().Get(1)
	ip, subnet, err := ipMaskSplit(c.Args().Get(2))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Invalid subnet mask specified: %s", err), -1)
	}

	negate := c.Bool("negate")
	comment := c.String("comment")

	item, err := util.NewACLEntry(client, serviceParam, aclParam, ip, subnet, comment, negate)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	if err := item.Add(); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func aclRemoveEntry(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	serviceParam := c.Args().Get(0)
	aclParam := c.Args().Get(1)
	ip, subnet, err := ipMaskSplit(c.Args().Get(2))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Invalid subnet mask specified: %s", err), -1)
	}

	item, err := util.NewACLEntry(client, serviceParam, aclParam, ip, subnet, "", false)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	if err := item.Remove(); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func aclListEntries(c *cli.Context) error {
	client, err := fastly.NewClient(c.GlobalString("fastly-key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	serviceParam := c.Args().Get(0)
	dictParam := c.Args().Get(1)
	acl, err := util.NewACL(client, serviceParam, dictParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	entries, err := acl.ListEntries()
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	fmt.Printf("Entries in acl %s for service %s:\n\n", dictParam, serviceParam)
	for _, entry := range entries {
		fmt.Println(entry.IP, entry.Subnet, entry.Negated, entry.Comment)
	}

	return nil
}
