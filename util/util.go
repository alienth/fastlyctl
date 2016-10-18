package util

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/alienth/go-fastly"
	"github.com/urfave/cli"
)

var ErrNonInteractive = errors.New("In non-interactive shell and --assume-yes not used.")

func GetServiceByName(client *fastly.Client, name string) (*fastly.Service, error) {
	var service *fastly.Service
	service, _, err := client.Service.Search(name)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func GetDictionaryByName(client *fastly.Client, serviceName, dictName string) (*fastly.Dictionary, error) {
	var err error
	service, err := GetServiceByName(client, serviceName)
	if err != nil {
		return nil, err
	}
	activeVersion, err := GetActiveVersion(service)
	if err != nil {
		return nil, err
	}
	_ = activeVersion

	dictionary, _, err := client.Dictionary.Get(service.ID, activeVersion, dictName)
	if err != nil {
		return nil, err
	}

	return dictionary, err
}

// getActiveVersion takes in a *fastly.Service and spits out the config version
// that is currently active for that service.
func GetActiveVersion(service *fastly.Service) (uint, error) {
	// Depending on how the service was fetched, it may or may not
	// have a filled ActiveVersion field.
	// TODO verify this is still the case
	if service.Version != 0 {
		return service.Version, nil
	} else {
		for _, version := range service.Versions {
			if version.Active {
				return version.Number, nil
			}
		}
	}
	return 0, fmt.Errorf("Unable to find the active version for service %s", service.Name)
}

func Prompt(question string) (bool, error) {
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

func CountChanges(diff *string) (int, int) {
	removals := regexp.MustCompile(`(^|\n)\-`)
	additions := regexp.MustCompile(`(^|\n)\+`)
	return len(additions.FindAllString(*diff, -1)), len(removals.FindAllString(*diff, -1))
}

func ActivateVersion(c *cli.Context, client *fastly.Client, s *fastly.Service, v *fastly.Version) error {
	activeVersion, err := GetActiveVersion(s)
	if err != nil {
		return err
	}
	assumeYes := c.GlobalBool("assume-yes")
	diff, _, err := client.Diff.Get(s.ID, activeVersion, v.Number, "text")
	if err != nil {
		return err
	}

	interactive := IsInteractive()
	if !interactive && !assumeYes {
		return cli.NewExitError(ErrNonInteractive.Error(), -1)
	}
	pager := GetPager()

	additions, removals := CountChanges(&diff.Diff)
	var proceed bool
	if !assumeYes {
		if proceed, err = Prompt(fmt.Sprintf("%d additions and %d removals in diff. View?", additions, removals)); err != nil {
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
		if proceed, err = Prompt("Activate version " + strconv.Itoa(int(v.Number)) + " for service " + s.Name + "?"); err != nil {
			return err
		}
	}
	if proceed || assumeYes {
		if _, _, err = client.Version.Activate(s.ID, v.Number); err != nil {
			return err
		}
		fmt.Printf("Activated version %d for %s. Old version: %d\n", v.Number, s.Name, activeVersion)
	}
	return nil
}

// validateVersion takes in a service and version number and returns an
// error if the version is invalid.
func ValidateVersion(client *fastly.Client, service *fastly.Service, version uint) error {
	// TODO verify this logic
	_, err := client.Version.Validate(service.ID, version)
	if err != nil {
		return fmt.Errorf("Error validating version: %s", err)
	}
	fmt.Printf("Version %d on service %s successfully validated!\n", version, service.Name)
	return nil
}

// Returns true if two versions of a given service are identical.  Generated
// VCL is not suitable as the ordering output of GeneratedVCL will vary if a
// no-op change has been made to a config (for example, removing and re-adding
// all domains). As such, this function generates a known-noop diff by
// comparing a version with itself, and then generating a diff between the from
// and to versions.  If the two diffs are identical, then there is no
// difference between from and to.
func VersionsEqual(c *fastly.Client, s *fastly.Service, from, to uint) (bool, error) {
	noDiff, _, err := c.Diff.Get(s.ID, from, from, "text")
	if err != nil {
		return false, err
	}
	diff, _, err := c.Diff.Get(s.ID, from, to, "text")
	if err != nil {
		return false, err
	}
	return noDiff.Diff == diff.Diff, nil
}

func StringInSlice(check string, slice []string) bool {
	for _, element := range slice {
		if element == check {
			return true
		}
	}
	return false
}

func GetPager() *exec.Cmd {
	for _, pager := range [3]string{os.Getenv("PAGER"), "pager", "less"} {
		// we expect some NotFounds, so ignore errors
		path, _ := exec.LookPath(pager)
		if path != "" {
			return exec.Command(path)
		}
	}
	return nil
}

func CheckFastlyKey(c *cli.Context) *cli.ExitError {
	if c.GlobalString("fastly-key") == "" {
		return cli.NewExitError("Error: Fastly API key must be set.", -1)
	}
	return nil
}

func GetFastlyKey() string {
	file := "fastly_key"
	if _, err := os.Stat(file); err == nil {
		contents, _ := ioutil.ReadFile(file)
		if contents[len(contents)-1] == []byte("\n")[0] {
			contents = contents[:len(contents)-1]
		}
		return string(contents)
	}
	return ""
}
