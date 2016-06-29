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

	"github.com/alienth/go-fastly"
	"github.com/imdario/mergo"
)

var pendingVersions map[string]fastly.Version
var siteConfigs map[string]SiteConfig

type SiteConfig struct {
	Backends      []*fastly.Backend
	Conditions    []*fastly.Condition
	CacheSettings []*fastly.CacheSetting
	Headers       []*fastly.Header
	Domains       []*fastly.Domain
	S3s           []*fastly.S3
	Settings      *fastly.Settings
	SSLHostname   string
}

func readConfig() error {
	body, _ := ioutil.ReadFile("config.json")
	err := json.Unmarshal(body, &siteConfigs)
	if err != nil {
		return err
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
		for _, s3 := range config.S3s {
			s3.Path = strings.Replace(s3.Path, "_servicename_", name, -1)
		}
		for _, domain := range config.Domains {
			domain.Name = strings.Replace(domain.Name, "_servicename_", name, -1)
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

func syncSettings(client *fastly.Client, s *fastly.Service, newSettings *fastly.Settings) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	var i fastly.UpdateSettingsInput
	i.Service = s.ID
	i.Version = newversion.Number
	i.DefaultTTL = newSettings.DefaultTTL
	i.DefaultHost = newSettings.DefaultHost
	if _, err = client.UpdateSettings(&i); err != nil {
		return err
	}

	return nil
}

func syncDomains(client *fastly.Client, s *fastly.Service, newDomains []*fastly.Domain) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingDomains, err := client.ListDomains(&fastly.ListDomainsInput{Service: s.ID, Version: newversion.Number})
	for _, domain := range existingDomains {
		err := client.DeleteDomain(&fastly.DeleteDomainInput{Service: s.ID, Name: domain.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, domain := range newDomains {
		var i fastly.CreateDomainInput

		i.Name = domain.Name
		i.Service = s.ID
		i.Comment = domain.Comment
		i.Version = newversion.Number

		if _, err = client.CreateDomain(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncS3s(client *fastly.Client, s *fastly.Service, newS3s []*fastly.S3) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingS3s, err := client.ListS3s(&fastly.ListS3sInput{Service: s.ID, Version: newversion.Number})
	for _, s3 := range existingS3s {
		err := client.DeleteS3(&fastly.DeleteS3Input{Service: s.ID, Name: s3.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, s3 := range newS3s {
		var i fastly.CreateS3Input

		i.Name = s3.Name
		i.Service = s.ID
		i.Version = newversion.Number
		i.Path = s3.Path
		i.Format = s3.Format
		i.Period = s3.Period
		i.TimestampFormat = s3.TimestampFormat
		i.BucketName = s3.BucketName
		i.AccessKey = s3.AccessKey
		i.GzipLevel = s3.GzipLevel
		i.SecretKey = s3.SecretKey
		i.Domain = s3.Domain
		i.ResponseCondition = s3.ResponseCondition
		i.Redundancy = s3.Redundancy

		if _, err = client.CreateS3(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncHeaders(client *fastly.Client, s *fastly.Service, newHeaders []*fastly.Header) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingHeaders, err := client.ListHeaders(&fastly.ListHeadersInput{Service: s.ID, Version: newversion.Number})
	for _, setting := range existingHeaders {
		err := client.DeleteHeader(&fastly.DeleteHeaderInput{Service: s.ID, Name: setting.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, header := range newHeaders {
		var i fastly.CreateHeaderInput
		i.Name = header.Name
		i.Type = header.Type
		i.Regex = header.Regex
		i.Destination = header.Destination
		i.Source = header.Source
		i.Action = header.Action
		i.Version = newversion.Number
		i.Service = s.ID
		i.Priority = header.Priority
		i.IgnoreIfSet = header.IgnoreIfSet
		i.Substitution = header.Substitution
		i.RequestCondition = header.RequestCondition
		i.ResponseCondition = header.ResponseCondition
		i.CacheCondition = header.CacheCondition

		if _, err = client.CreateHeader(&i); err != nil {
			return err
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

	remoteHeaders, _ := client.ListHeaders(&fastly.ListHeadersInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Headers, remoteHeaders) {
		if err := syncHeaders(client, s, config.Headers); err != nil {
			return err
		}
	}

	remoteS3s, _ := client.ListS3s(&fastly.ListS3sInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.S3s, remoteS3s) {
		if err := syncS3s(client, s, config.S3s); err != nil {
			return err
		}
	}

	remoteDomains, _ := client.ListDomains(&fastly.ListDomainsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Domains, remoteDomains) {
		if err := syncDomains(client, s, config.Domains); err != nil {
			return err
		}
	}

	remoteSettings, _ := client.GetSettings(&fastly.GetSettingsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Settings, remoteSettings) {
		if err := syncSettings(client, s, config.Settings); err != nil {
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
		log.Fatalf("Error reading config file: %s", err)
	}
	pendingVersions = make(map[string]fastly.Version)

	services, err := client.ListServices(&fastly.ListServicesInput{})
	if err != nil {
		log.Fatalf("Error listing services: %s", err)
	}
	for _, s := range services {
		if err = syncVcls(client, s); err != nil {
			log.Fatalf("Error syncing VCLs: %s", err)
		}
		if err = syncConfig(client, s); err != nil {
			log.Fatalf("Error syncing service config: %s", err)
		}
	}
}
