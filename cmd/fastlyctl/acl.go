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
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))

	var err error
	serviceParam := c.Args().Get(0)
	var service *fastly.Service
	if service, _, err = client.Service.Search(serviceParam); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}
	activeVersion, err := util.GetActiveVersion(service)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	acls, _, err := client.ACL.List(service.ID, activeVersion)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Unable to list ACLs for service %s\n", service.Name), -1)
	}
	fmt.Printf("ACLs for %s:\n\n", service.Name)
	for _, a := range acls {
		fmt.Println(a.Name)
	}
	return nil
}

func getACL(client *fastly.Client, serviceName, aclName string) (*fastly.ACL, error) {
	var err error
	var service *fastly.Service
	if service, _, err = client.Service.Search(serviceName); err != nil {
		return nil, err
	}
	activeVersion, err := util.GetActiveVersion(service)
	if err != nil {
		return nil, err
	}
	_ = activeVersion

	acl, _, err := client.ACL.Get(service.ID, activeVersion, aclName)
	if err != nil {
		return nil, err
	}

	return acl, err
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
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))

	serviceParam := c.Args().Get(0)
	aclParam := c.Args().Get(1)
	ip, subnet, err := ipMaskSplit(c.Args().Get(2))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Invalid subnet mask specified: %s", err), -1)
	}

	negate := fastly.Compatibool(c.Bool("negate"))
	comment := c.String("comment")

	acl, err := getACL(client, serviceParam, aclParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	entry := new(fastly.ACLEntry)
	entry.IP = ip
	entry.Subnet = subnet
	entry.Comment = comment
	entry.Negated = negate

	if _, _, err = client.ACLEntry.Create(acl.ServiceID, acl.ID, entry); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func aclRemoveEntry(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))

	serviceParam := c.Args().Get(0)
	aclParam := c.Args().Get(1)
	ip, subnet, err := ipMaskSplit(c.Args().Get(2))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Invalid subnet mask specified: %s", err), -1)
	}

	acl, err := getACL(client, serviceParam, aclParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	entries, _, err := client.ACLEntry.List(acl.ServiceID, acl.ID)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	var entry = new(fastly.ACLEntry)
	for _, e := range entries {
		if e.IP == ip && e.Subnet == subnet {
			entry = e
			break
		}
	}

	if entry == nil {
		return cli.NewExitError("Unable to find ACL entry\n", -1)
	}

	if _, err = client.ACLEntry.Delete(acl.ServiceID, acl.ID, entry.ID); err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	return nil
}

func aclListEntries(c *cli.Context) error {
	client := fastly.NewClient(nil, c.GlobalString("fastly-key"))

	serviceParam := c.Args().Get(0)
	aclParam := c.Args().Get(1)

	acl, err := getACL(client, serviceParam, aclParam)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	entries, _, err := client.ACLEntry.List(acl.ServiceID, acl.ID)
	if err != nil {
		return cli.NewExitError(err.Error(), -1)
	}

	fmt.Printf("Entries in acl %s for service %s:\n\n", aclParam, serviceParam)
	for _, entry := range entries {
		fmt.Println(entry.IP, entry.Subnet, entry.Negated, entry.Comment)
	}

	return nil
}
