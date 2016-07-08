package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"

	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
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

func countChanges(diff *string) (int, int) {
	removals := regexp.MustCompile(`(^|\n)\-`)
	additions := regexp.MustCompile(`(^|\n)\+`)
	return len(additions.FindAllString(*diff, -1)), len(removals.FindAllString(*diff, -1))
}

func activateVersion(c *cli.Context, client *fastly.Client, s *fastly.Service, v *fastly.Version) error {
	activeVersion, err := getActiveVersion(s)
	if err != nil {
		return err
	}
	assumeYes := c.GlobalBool("assume-yes")
	diff, err := client.GetDiff(&fastly.GetDiffInput{Service: s.ID, Format: "text", From: activeVersion, To: v.Number})
	if err != nil {
		return err
	}

	interactive := terminal.IsTerminal(syscall.Stdin)
	pager := getPager()

	additions, removals := countChanges(&diff.Diff)
	var proceed bool
	if !assumeYes {
		if proceed, err = prompt(fmt.Sprintf("%d additions and %d removals in diff. View?", additions, removals)); err != nil {
			return err
		}
	}

	if proceed || assumeYes {
		if pager != nil && interactive && !assumeYes {
			r, stdin := io.Pipe()
			pager.Stdin = r
			pager.Stdout = os.Stdout
			pager.Stderr = os.Stderr

			c := make(chan struct{})
			go func() {
				defer close(c)
				pager.Run()
			}()

			fmt.Fprintf(stdin, diff.Diff)
			stdin.Close()
			<-c
		} else {
			fmt.Printf("Diff for %s:\n\n", s.Name)
			fmt.Println(diff.Diff)
		}
	}

	if !assumeYes {
		if proceed, err = prompt("Activate version " + v.Number + " for service " + s.Name + "?"); err != nil {
			return err
		}
	}
	if proceed || assumeYes {
		if _, err = client.ActivateVersion(&fastly.ActivateVersionInput{Service: s.ID, Version: v.Number}); err != nil {
			return err
		}
		fmt.Printf("Activated version %s for %s. Old version: %s\n", v.Number, s.Name, activeVersion)
	}
	return nil
}

// validateVersion takes in a service and version number and returns an
// error if the version is invalid.
func validateVersion(client *fastly.Client, service *fastly.Service, version string) error {
	result, msg, err := client.ValidateVersion(&fastly.ValidateVersionInput{Service: service.ID, Version: version})
	if err != nil {
		return fmt.Errorf("Error validating version: %s", err)
	}
	if result {
		fmt.Printf("Version %s on service %s successfully validated!\n", version, service.Name)
	} else {
		return fmt.Errorf("Version %s on service %s is invalid:\n\n%s", version, service.Name, msg)
	}
	return nil
}

// Returns true if two versions of a given service are identical.  Generated
// VCL is not suitable as the ordering output of GeneratedVCL will vary if a
// no-op change has been made to a config (for example, removing and re-adding
// all domains). As such, this function generates a known-noop diff by
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

func getPager() *exec.Cmd {
	for _, pager := range [3]string{os.Getenv("PAGER"), "pager", "less"} {
		// we expect some NotFounds, so ignore errors
		path, _ := exec.LookPath(pager)
		if path != "" {
			return exec.Command(path)
		}
	}
	return nil
}

func debugPrint(message string) {
	if debug {
		fmt.Print(message)
	}
}
