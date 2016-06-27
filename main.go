package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/sethvargo/go-fastly"
)

var pendingVersions map[string]fastly.Version

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

func main() {
	client, err := fastly.NewClient(os.Getenv("FASTLY_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	hasher := sha256.New()

	services, err := client.ListServices(&fastly.ListServicesInput{})
	for _, s := range services {
		var activeVersion = strconv.Itoa(int(s.ActiveVersion))
		vcls, err := client.ListVCLs(&fastly.ListVCLsInput{Service: s.ID, Version: activeVersion})
		if err != nil {
			log.Fatal(err)
		}

		for _, v := range vcls {
			filename := v.Name + ".vcl"
			f, err := os.Open(filename)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			if _, err := io.Copy(hasher, f); err != nil {
				log.Fatal(err)
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
					log.Fatal(err)
				}

				fmt.Println(s.ID, activeVersion, v.Name)
				//newversion, err := client.CloneVersion(&fastly.CloneVersionInput{Service: s.ID, Version: activeVersion})
				newversion, err := prepareNewVersion(client, s)
				if err != nil {
					log.Fatal(err)
				}
				if _, err = client.UpdateVCL(&fastly.UpdateVCLInput{Name: v.Name, Service: s.ID, Version: newversion.Number, Content: string(content)}); err != nil {
					log.Fatal(err)
				}
			}

		}

	}
}
