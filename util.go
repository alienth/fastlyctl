package main

import (
	"fmt"
	"strconv"

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

// getActiveVersion takes in a *fastly.Service and spits out the config version
// that is currently active for that service.
func getActiveVersion(service *fastly.Service) (string, error) {
	// Depending on how the service was fetched, it may or may not
	// have a filled ActiveVersion field.
	if service.ActiveVersion != 0 {
		return strconv.Itoa(int(service.ActiveVersion)), nil
	} else {
		for _, version := range service.Versions {
			if version.Active {
				return version.Number, nil
			}
		}
	}
	return "", fmt.Errorf("Unable to find the active version for service %s", service.Name)
}
