package main

import (
	"fmt"

	"github.com/alienth/go-fastly"
)

func getServiceByNameOrID(client *fastly.Client, identifier string) (*fastly.Service, error) {
	var service *fastly.Service
	service, err := client.SearchService(&fastly.SearchServiceInput{Name: identifier})
	if err != nil {
		if service, err = client.GetService(&fastly.GetServiceInput{ID: identifier}); err != nil {
			return nil, fmt.Errorf("Error fetching service %s: %s", identifier, err)
		}
	}
	return service, nil
}
