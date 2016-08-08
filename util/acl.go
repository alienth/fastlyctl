package util

import (
	"errors"
	"strings"

	"github.com/alienth/go-fastly"
)

var ErrGetACL = errors.New("Unable to fetch acl")

type ACLEntry struct {
	ServiceID string
	ACLID     string

	ID      string
	IP      string
	Subnet  uint8
	Comment string
	Negated bool

	Client *fastly.Client
}

type ACL struct {
	ServiceID string
	ID        string
	Client    *fastly.Client
}

func NewACLEntry(client *fastly.Client, serviceName, aclName, ip string, subnet uint8, comment string, negated bool) (*ACLEntry, error) {
	service, err := GetServiceByName(client, serviceName)
	if err != nil {
		return nil, err
	}

	acl, err := NewACL(client, service.Name, aclName)
	if err != nil {
		return nil, ErrGetACL
	}

	var entry ACLEntry
	entry.ServiceID = service.ID
	entry.ACLID = acl.ID
	entry.IP = ip
	entry.Subnet = subnet
	entry.Comment = comment
	entry.Negated = negated
	entry.Client = client

	entries, err := acl.ListEntries()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IP == entry.IP {
			entry.ID = e.ID
		}
	}

	return &entry, nil
}

func (e *ACLEntry) Add() error {
	var input fastly.CreateACLEntryInput
	input.Service = e.ServiceID
	input.ACL = e.ACLID
	input.IP = e.IP
	input.Subnet = e.Subnet
	input.Comment = e.Comment
	input.IP = e.IP
	input.Negated = fastly.Compatibool(e.Negated)
	entry, err := e.Client.CreateACLEntry(&input)
	if err != nil {
		return err
	}
	e.ID = entry.ID
	return nil
}

func (e *ACLEntry) Remove() error {
	var input fastly.DeleteACLEntryInput
	input.Service = e.ServiceID
	input.ACL = e.ACLID
	input.ID = e.ID
	if err := e.Client.DeleteACLEntry(&input); err != nil {
		if strings.Contains(err.Error(), "Record not found") {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func NewACL(client *fastly.Client, serviceName, aclName string) (*ACL, error) {
	service, err := GetServiceByName(client, serviceName)
	if err != nil {
		return nil, err
	}
	activeVersion, err := GetActiveVersion(service)
	if err != nil {
		return nil, err
	}

	acl, err := client.GetACL(&fastly.GetACLInput{Service: service.ID, Version: activeVersion, Name: aclName})
	if err != nil {
		return nil, ErrGetACL
	}

	return &ACL{ServiceID: service.ID, ID: acl.ID, Client: client}, nil
}

func (a *ACL) ListEntries() ([]*fastly.ACLEntry, error) {
	entries, err := a.Client.ListACLEntries(&fastly.ListACLEntriesInput{Service: a.ServiceID, ACL: a.ID})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
