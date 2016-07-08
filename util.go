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

func prompt(question string) (bool, error) {
	var input string
	for {
		fmt.Printf("%s (y/n): ", question)
		if _, err := fmt.Scanln(&input); err != nil {
			return false, err
		}
		if input == "y" {
			return true, nil
		} else if input == "n" {
			return false, nil
		} else {
			fmt.Printf("Invalid input: %s", input)
		}
	}
}

func activateVersion(client *fastly.Client, s *fastly.Service, v *fastly.Version) error {
	activeVersion, err := getActiveVersion(s)
	if err != nil {
		return err
	}
	diff, err := client.GetDiff(&fastly.GetDiffInput{Service: s.ID, Format: "text", From: activeVersion, To: v.Number})
	if err != nil {
		return err
	}
	fmt.Printf("Diff for %s:\n\n", s.Name)
	fmt.Println(diff.Diff)
	proceed, err := prompt("Activate version " + v.Number + " for service " + s.Name + "?")
	if err != nil {
		return err
	}
	if proceed {
		if _, err = client.ActivateVersion(&fastly.ActivateVersionInput{Service: s.ID, Version: v.Number}); err != nil {
			return err
		}
	}
	return nil
}

// Returns true if two versions of a given service are identical.  Generated
// VCL is not suitable as the ordering output of GeneratedVCL is
// non-deterministic.  As such, this function generates a known-noop diff by
// comparing a version with itself, and then generating a diff between the from
// and to versions.  If the two diffs are identical, then there is no
// difference between from and to.
func versionsEqual(c *fastly.Client, s *fastly.Service, from string, to string) (bool, error) {
	var i fastly.GetDiffInput
	i.Service = s.ID
	// Intentional
	i.To = from
	i.From = from
	noDiff, err := c.GetDiff(&i)
	if err != nil {
		return false, err
	}
	i.To = to
	diff, err := c.GetDiff(&i)
	if err != nil {
		return false, err
	}
	return noDiff.Diff == diff.Diff, nil
}

func stringInSlice(check string, slice []string) bool {
	for _, element := range slice {
		if element == check {
			return true
		}
	}
	return false
}
