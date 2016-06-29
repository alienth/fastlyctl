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
	"strings"

	"github.com/imdario/mergo"
	"github.com/sethvargo/go-fastly"
)

var pendingVersions map[string]fastly.Version
var siteConfigs map[string]SiteConfig

type SiteConfig struct {
	Backends      []*fastly.Backend
	Conditions    []*fastly.Condition
	CacheSettings []*fastly.CacheSetting
	SSLHostname   string
}

func readConfig() error {
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

	for name, config := range siteConfigs {
		if name == "_default_" {
			continue
		}

		if err := mergo.Merge(&config, siteConfigs["_default_"]); err != nil {
			return err
		}
		siteConfigs[name] = config
		for _, backend := range config.Backends {
			backend.SSLHostname = strings.Replace(backend.SSLHostname, "_servicename_", name, -1)
		}

	}
	return nil
}

const versionComment string = "fastly-ctl"

func prepareNewVersion(client *fastly.Client, s *fastly.Service) (fastly.Version, error) {
	// See if we've already prepared a version
	if version, ok := pendingVersions[s.ID]; ok {
		return version, nil
	}

	// Look for an inactive version higher than our current version
	versions, err := client.ListVersions(&fastly.ListVersionsInput{Service: s.ID})
	if err != nil {
		return fastly.Version{}, err
	}
	for _, v := range versions {
		versionNumber, err := strconv.Atoi(v.Number)
		if err != nil {
			return fastly.Version{}, fmt.Errorf("Invalid version number encountered: %s", err)
		}
		if uint(versionNumber) > s.ActiveVersion && v.Comment == versionComment && !v.Active && !v.Locked {
			pendingVersions[s.ID] = *v
			return *v, nil
		}
	}

	// Otherwise, create a new version
	newversion, err := client.CloneVersion(&fastly.CloneVersionInput{Service: s.ID, Version: strconv.Itoa(int(s.ActiveVersion))})
	if err != nil {
		return *newversion, err
	}
	if _, err := client.UpdateVersion(&fastly.UpdateVersionInput{Service: s.ID, Version: newversion.Number, Comment: versionComment}); err != nil {
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

func syncCacheSettings(client *fastly.Client, s *fastly.Service, newCacheSettings []*fastly.CacheSetting) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingCacheSettings, err := client.ListCacheSettings(&fastly.ListCacheSettingsInput{Service: s.ID, Version: newversion.Number})
	for _, setting := range existingCacheSettings {
		err := client.DeleteCacheSetting(&fastly.DeleteCacheSettingInput{Service: s.ID, Name: setting.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, setting := range newCacheSettings {
		var i fastly.CreateCacheSettingInput
		i.TTL = setting.TTL
		i.Name = setting.Name
		i.Action = setting.Action
		i.Service = s.ID
		i.Version = newversion.Number
		i.StaleTTL = setting.StaleTTL
		i.CacheCondition = setting.CacheCondition

		if _, err = client.CreateCacheSetting(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncConditions(client *fastly.Client, s *fastly.Service, newConditions []*fastly.Condition) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingConditions, err := client.ListConditions(&fastly.ListConditionsInput{Service: s.ID, Version: newversion.Number})
	if err != nil {
		return err
	}
	for _, condition := range existingConditions {
		err := client.DeleteCondition(&fastly.DeleteConditionInput{Service: s.ID, Name: condition.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, condition := range newConditions {
		var i fastly.CreateConditionInput
		i.Name = condition.Name
		i.Type = condition.Type
		i.Service = s.ID
		i.Version = newversion.Number
		i.Priority = condition.Priority
		i.Statement = condition.Statement
		if _, err = client.CreateCondition(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncBackends(client *fastly.Client, s *fastly.Service, newBackends []*fastly.Backend) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingBackends, err := client.ListBackends(&fastly.ListBackendsInput{Service: s.ID, Version: newversion.Number})
	if err != nil {
		return err
	}
	for _, backend := range existingBackends {
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
		i.SSLCheckCert = backend.SSLCheckCert
		i.SSLSNIHostname = backend.SSLSNIHostname
		i.SSLHostname = backend.SSLHostname
		i.AutoLoadbalance = backend.AutoLoadbalance
		i.Weight = backend.Weight
		i.MaxConn = backend.MaxConn
		i.ConnectTimeout = backend.ConnectTimeout
		i.FirstByteTimeout = backend.FirstByteTimeout
		i.BetweenBytesTimeout = backend.BetweenBytesTimeout
		i.HealthCheck = backend.HealthCheck
		i.RequestCondition = backend.RequestCondition
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
	} else {
		config = siteConfigs["_default_"]
	}

	remoteConditions, err := client.ListConditions(&fastly.ListConditionsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	// Conditions must be sync'd first, as if they're referenced in any other setup
	// the API will reject if they don't exist.
	if !reflect.DeepEqual(config.Conditions, remoteConditions) {
		if err := syncConditions(client, s, config.Conditions); err != nil {
			return fmt.Errorf("Error syncing conditions: %s", err)
		}
	}
	remoteBackends, err := client.ListBackends(&fastly.ListBackendsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Backends, remoteBackends) {
		if err := syncBackends(client, s, config.Backends); err != nil {
			return err
		}
	}

	remoteCacheSettings, _ := client.ListCacheSettings(&fastly.ListCacheSettingsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.CacheSettings, remoteCacheSettings) {
		if err := syncCacheSettings(client, s, config.CacheSettings); err != nil {
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

	if err := readConfig(); err != nil {
		log.Fatal(err)
	}
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
