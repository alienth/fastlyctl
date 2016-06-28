package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"

	"github.com/imdario/mergo"
	"github.com/sethvargo/go-fastly"
)

var pendingVersions map[string]fastly.Version
var siteConfigs map[string]SiteConfig

type SiteConfig struct {
	Backends    []*fastly.Backend
	SSLHostname string
}

func readConfig() {
	//	var parsed interface{}
	//	f, _ := os.Open("config.json")
	//	dec := json.NewDecoder(f)
	//	if err := dec.Decode(&parsed); err != nil {
	//		log.Fatal(err)
	//	}
	//	fmt.Println(parsed)

	body, _ := ioutil.ReadFile("config.json")
	err := json.Unmarshal(body, &siteConfigs)
	if err != nil {
		log.Fatal(err)
	}
}

func prepareNewVersion(client *fastly.Client, s *fastly.Service) (fastly.Version, error) {
	if version, ok := pendingVersions[s.ID]; ok {
		return version, nil
	}

	// Otherwise, create a new version
	newversion, err := client.CloneVersion(&fastly.CloneVersionInput{Service: s.ID, Version: strconv.Itoa(int(s.ActiveVersion))})
	if err != nil {
		return *newversion, err
	}
	pendingVersions[s.ID] = *newversion
	return *newversion, nil
}

func syncVcls(client *fastly.Client, s *fastly.Service) error {
	hasher := sha256.New()
	var activeVersion = strconv.Itoa(int(s.ActiveVersion))
	vcls, err := client.ListVCLs(&fastly.ListVCLsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}

	for _, v := range vcls {
		filename := v.Name + ".vcl"
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(hasher, f); err != nil {
			return err
		}
		localsum := hasher.Sum(nil)
		hasher.Reset()

		hasher.Write([]byte(v.Content))
		remotesum := hasher.Sum(nil)
		hasher.Reset()

		if !bytes.Equal(localsum, remotesum) {
			fmt.Printf("VCL mismatch on service %s VCL %s. Updating.\n", s.Name, v.Name)
			content, err := ioutil.ReadFile(filename)
			if err != nil {
				return err
			}

			//newversion, err := client.CloneVersion(&fastly.CloneVersionInput{Service: s.ID, Version: activeVersion})
			newversion, err := prepareNewVersion(client, s)
			if err != nil {
				return err
			}
			if _, err = client.UpdateVCL(&fastly.UpdateVCLInput{Name: v.Name, Service: s.ID, Version: newversion.Number, Content: string(content)}); err != nil {
				return err
			}
		}

	}
	return nil
}

func syncBackends(client *fastly.Client, s *fastly.Service, currentBackends []*fastly.Backend, newBackends []*fastly.Backend) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	for _, backend := range currentBackends {
		err := client.DeleteBackend(&fastly.DeleteBackendInput{Service: s.ID, Name: backend.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, backend := range newBackends {
		var i fastly.CreateBackendInput
		i.Address = backend.Address
		i.Name = backend.Name
		i.Service = newversion.ServiceID
		i.Version = newversion.Number
		i.UseSSL = backend.UseSSL
		if i.UseSSL && backend.SSLHostname != "" {
			i.SSLHostname = backend.SSLHostname
		}
		if _, err = client.CreateBackend(&i); err != nil {
			return err
		}
	}

	return nil
}

func syncConfig(client *fastly.Client, s *fastly.Service) error {
	var activeVersion = strconv.Itoa(int(s.ActiveVersion))
	var config SiteConfig
	if _, ok := siteConfigs[s.Name]; ok {
		config = siteConfigs[s.Name]
		if err := mergo.Merge(&config, siteConfigs["_default_"]); err != nil {
			return err
		}
	} else {
		config = siteConfigs["_default_"]
	}
	remoteBackends, _ := client.ListBackends(&fastly.ListBackendsInput{Service: s.ID, Version: activeVersion})
	if !reflect.DeepEqual(config.Backends, remoteBackends) {
		if err := syncBackends(client, s, remoteBackends, config.Backends); err != nil {
			return err
		}
	}

	return nil

}

func main() {
	client, err := fastly.NewClient(os.Getenv("FASTLY_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	readConfig()
	pendingVersions = make(map[string]fastly.Version)

	services, err := client.ListServices(&fastly.ListServicesInput{})
	for _, s := range services {
		if err = syncVcls(client, s); err != nil {
			log.Fatal(err)
		}
		if err = syncConfig(client, s); err != nil {
			log.Fatal(err)
		}
	}
}
