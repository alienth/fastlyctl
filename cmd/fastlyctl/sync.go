package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/alienth/fastlyctl/log"
	"github.com/alienth/fastlyctl/util"
	"github.com/alienth/go-fastly"
	"github.com/imdario/mergo"
	"github.com/urfave/cli"
)

var pendingVersions map[string]fastly.Version
var siteConfigs map[string]SiteConfig

const (
	defaultHealthCheckHTTPVersion = "1.1"
	defaultS3TimestampFormat      = "%Y-%m-%dT%H:%M:%S.000"
)

type SiteConfig struct {
	Settings      fastly.Settings
	Domains       []fastly.Domain
	Backends      []fastly.Backend
	Conditions    []fastly.Condition
	CacheSettings []fastly.CacheSetting
	Headers       []fastly.Header
	S3s           []fastly.S3
	//	FTPs             []fastly.CreateFTPInput
	//	GCSs             []fastly.CreateGCSInput
	//	Papertrails      []fastly.CreatePapertrailInput
	//	Sumologics       []fastly.CreateSumologicInput
	Syslogs         []fastly.Syslog
	Gzips           []fastly.Gzip
	HealthChecks    []fastly.HealthCheck
	Dictionaries    []fastly.Dictionary
	ACLs            []fastly.ACL
	VCLs            []VCL
	RequestSettings []fastly.RequestSetting
	ResponseObject  []fastly.ResponseObject

	IPPrefix string
	IPSuffix string

	S3AccessKey string
	S3SecretKey string
}

type VCL struct {
	Name    string
	Content string
	File    string
	Main    bool
}

func readConfig(file string) error {
	body, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	if strings.HasSuffix(file, ".toml") {
		if err := toml.Unmarshal(body, &siteConfigs); err != nil {
			return fmt.Errorf("toml parsing error: %s\n", err)
		}
	} else if strings.HasSuffix(file, ".json") {
		if err := json.Unmarshal(body, &siteConfigs); err != nil {
			return fmt.Errorf("json parsing error: %s\n", err)
		}
	} else {
		return fmt.Errorf("Unknown config file type for file %s\n", file)
	}

	//outfile, _ := os.OpenFile("out.toml", os.O_CREATE|os.O_RDWR, 0644)
	//encoder := toml.NewEncoder(outfile)
	//encoder.Encode(&siteConfigs)
	//outfile.Close()
	//outfile, _ = os.OpenFile("out.json", os.O_CREATE|os.O_RDWR, 0644)
	//jencoder := json.NewEncoder(outfile)
	//jencoder.Encode(&siteConfigs)
	//outfile.Close()

	for name, config := range siteConfigs {
		if name == "_default_" {
			continue
		}

		if err := mergo.Merge(&config, siteConfigs["_default_"]); err != nil {
			return err
		}
		siteConfigs[name] = config
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
	versions, _, err := client.Version.List(s.ID)
	if err != nil {
		return fastly.Version{}, err
	}
	for _, v := range versions {
		if v.Number > s.Version && v.Comment == versionComment && !v.Active && !v.Locked {
			pendingVersions[s.ID] = *v
			return *v, nil
		}
	}

	// Otherwise, create a new version
	newversion, _, err := client.Version.Clone(s.ID, s.Version)
	if err != nil {
		return *newversion, err
	}
	newversion.Comment = versionComment
	// Zero out unwritable fields
	newversion.Updated = ""
	newversion.Created = ""
	if _, _, err := client.Version.Update(s.ID, newversion.Number, newversion); err != nil {
		return *newversion, err
	}
	pendingVersions[s.ID] = *newversion
	return *newversion, nil
}

func syncVCLs(client *fastly.Client, s *fastly.Service, vcls []VCL) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	var newVCLs []fastly.VCL

	for _, vcl := range vcls {
		if vcl == (VCL{}) {
			continue
		}
		var newVCL fastly.VCL
		if vcl.File != "" && vcl.Content != "" {
			return fmt.Errorf("Cannot specify both a File and Content for VCL %s", vcl.Name)
		}
		if vcl.File != "" {
			var content []byte
			if content, err = ioutil.ReadFile(vcl.File); err != nil {
				return err
			}
			newVCL.Content = string(content)
		} else if vcl.Content != "" {
			newVCL.Content = vcl.Content
		} else {
			return fmt.Errorf("No Content or File specified for VCL %s", vcl.Name)
		}
		newVCL.Main = vcl.Main
		newVCL.Name = vcl.Name
		newVCLs = append(newVCLs, newVCL)
	}

	existingVCLs, _, err := client.VCL.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, vcl := range existingVCLs {
		var match bool
		// Zero out read-only fields that we don't want to compare
		vcl.ServiceID = ""
		vcl.Version = 0
		for i, newVCL := range newVCLs {
			if *vcl == newVCL {
				log.Debug(fmt.Sprintf("Found matching vcl %s. Not creating.\n", vcl.Name))
				newVCLs = append(newVCLs[:i], newVCLs[i+1:]...)
				match = true
				break
			} else if vcl.Name == newVCL.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing vcl %s. Updating.\n", vcl.Name))
				if _, _, err := client.VCL.Update(s.ID, newversion.Number, vcl.Name, &newVCL); err != nil {
					return err
				}
				newVCLs = append(newVCLs[:i], newVCLs[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching vcl %s. Deleting.\n", vcl.Name))
			_, err := client.VCL.Delete(s.ID, newversion.Number, vcl.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, vcl := range newVCLs {
		log.Debug(fmt.Sprintf("Creating missing vcl %s.\n", vcl.Name))
		_, _, err := client.VCL.Create(s.ID, newversion.Number, &vcl)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncHealthChecks(client *fastly.Client, s *fastly.Service, newHealthChecks []fastly.HealthCheck) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	for i := range newHealthChecks {
		if newHealthChecks[i].HTTPVersion == "" {
			newHealthChecks[i].HTTPVersion = defaultHealthCheckHTTPVersion
		}
	}

	existingHealthChecks, _, err := client.HealthCheck.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, healthCheck := range existingHealthChecks {
		var match bool
		// Zero out read-only fields that we don't want to compare
		healthCheck.ServiceID = ""
		healthCheck.Version = 0
		for i, newHealthCheck := range newHealthChecks {
			if *healthCheck == newHealthCheck {
				log.Debug(fmt.Sprintf("Found matching healthCheck %s. Not creating.\n", healthCheck.Name))
				newHealthChecks = append(newHealthChecks[:i], newHealthChecks[i+1:]...)
				match = true
				break
			} else if healthCheck.Name == newHealthCheck.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing healthCheck %s. Updating.\n", healthCheck.Name))
				if _, _, err := client.HealthCheck.Update(s.ID, newversion.Number, healthCheck.Name, &newHealthCheck); err != nil {
					return err
				}
				newHealthChecks = append(newHealthChecks[:i], newHealthChecks[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching healthCheck %s. Deleting.\n", healthCheck.Name))
			_, err := client.HealthCheck.Delete(s.ID, newversion.Number, healthCheck.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, healthCheck := range newHealthChecks {
		if healthCheck == (fastly.HealthCheck{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing healthCheck %s.\n", healthCheck.Name))
		_, _, err := client.HealthCheck.Create(s.ID, newversion.Number, &healthCheck)
		if err != nil {
			return err
		}
	}
	return nil
}

// Caveat: contentTypes is autogenerated by fastly
func syncGzips(client *fastly.Client, s *fastly.Service, newGzips []fastly.Gzip) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingGzips, _, err := client.Gzip.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, gzip := range existingGzips {
		var match bool
		// Zero out read-only fields that we don't want to compare
		gzip.ServiceID = ""
		gzip.Version = 0
		for i, newGzip := range newGzips {
			if *gzip == newGzip {
				log.Debug(fmt.Sprintf("Found matching gzip %s. Not creating.\n", gzip.Name))
				newGzips = append(newGzips[:i], newGzips[i+1:]...)
				match = true
				break
			} else if gzip.Name == newGzip.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing gzip %s. Updating.\n", gzip.Name))
				if _, _, err := client.Gzip.Update(s.ID, newversion.Number, gzip.Name, &newGzip); err != nil {
					return err
				}
				newGzips = append(newGzips[:i], newGzips[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching gzip %s. Deleting.\n", gzip.Name))
			_, err := client.Gzip.Delete(s.ID, newversion.Number, gzip.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, gzip := range newGzips {
		if gzip == (fastly.Gzip{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing gzip %s.\n", gzip.Name))
		_, _, err := client.Gzip.Create(s.ID, newversion.Number, &gzip)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncSettings(client *fastly.Client, s *fastly.Service, newSettings fastly.Settings) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingSettings, _, err := client.Settings.Get(s.ID, newversion.Number)
	if err != nil {
		return err
	}

	// Zero out read-only fields that we don't want to compare
	existingSettings.ServiceID = ""
	existingSettings.Version = 0
	if newSettings != *existingSettings {
		log.Debug("Mismatched settings. Updating.\n")
		if _, _, err = client.Settings.Update(s.ID, newversion.Number, &newSettings); err != nil {
			return err
		}
	}

	return nil
}

func syncDomains(client *fastly.Client, s *fastly.Service, newDomains []fastly.Domain) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	r := strings.NewReplacer("_servicename_", s.Name)
	for i := range newDomains {
		newDomains[i].Name = r.Replace(newDomains[i].Name)
	}

	existingDomains, _, err := client.Domain.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, domain := range existingDomains {
		var match bool
		// Zero out read-only fields that we don't want to compare
		domain.ServiceID = ""
		domain.Version = 0
		for i, newDomain := range newDomains {
			if *domain == newDomain {
				log.Debug(fmt.Sprintf("Found matching domain %s. Not creating.\n", domain.Name))
				newDomains = append(newDomains[:i], newDomains[i+1:]...)
				match = true
				break
			} else if domain.Name == newDomain.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing domain %s. Updating.\n", domain.Name))
				if _, _, err := client.Domain.Update(s.ID, newversion.Number, domain.Name, &newDomain); err != nil {
					return err
				}
				newDomains = append(newDomains[:i], newDomains[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching domain %s. Deleting.\n", domain.Name))
			_, err := client.Domain.Delete(s.ID, newversion.Number, domain.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, domain := range newDomains {
		if domain == (fastly.Domain{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing domain %s.\n", domain.Name))
		_, _, err := client.Domain.Create(s.ID, newversion.Number, &domain)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncSyslogs(client *fastly.Client, s *fastly.Service, newSyslogs []fastly.Syslog) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	r := strings.NewReplacer("_servicename_", s.Name, "_prefix_", siteConfigs[s.Name].IPPrefix, "_suffix_", siteConfigs[s.Name].IPSuffix)
	for i := range newSyslogs {
		newSyslogs[i].TLSHostname = r.Replace(newSyslogs[i].TLSHostname)
		newSyslogs[i].Address = r.Replace(newSyslogs[i].Address)
	}

	existingSyslogs, _, err := client.Syslog.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, syslog := range existingSyslogs {
		var match bool
		// Zero out read-only fields that we don't want to compare
		syslog.ServiceID = ""
		syslog.Version = 0
		for i, newSyslog := range newSyslogs {
			if *syslog == newSyslog {
				log.Debug(fmt.Sprintf("Found matching syslog %s. Not creating.\n", syslog.Name))
				newSyslogs = append(newSyslogs[:i], newSyslogs[i+1:]...)
				match = true
				break
			} else if syslog.Name == newSyslog.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing syslog %s. Updating.\n", syslog.Name))
				if _, _, err := client.Syslog.Update(s.ID, newversion.Number, syslog.Name, &newSyslog); err != nil {
					return err
				}
				newSyslogs = append(newSyslogs[:i], newSyslogs[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching syslog %s. Deleting.\n", syslog.Name))
			_, err := client.Syslog.Delete(s.ID, newversion.Number, syslog.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, syslog := range newSyslogs {
		if syslog == (fastly.Syslog{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing syslog %s.\n", syslog.Name))
		_, _, err := client.Syslog.Create(s.ID, newversion.Number, &syslog)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncS3s(client *fastly.Client, s *fastly.Service, newS3s []fastly.S3) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	accessKey := os.Getenv("FASTLY_S3_ACCESS_KEY")
	secretKey := os.Getenv("FASTLY_S3_SECRET_KEY")
	if accessKey == "" {
		accessKey = siteConfigs[s.Name].S3AccessKey
	}
	if secretKey == "" {
		secretKey = siteConfigs[s.Name].S3SecretKey
	}

	r := strings.NewReplacer("_servicename_", s.Name, "_s3accesskey_", accessKey, "_s3secretkey_", secretKey)
	for i := range newS3s {
		if newS3s[i].TimestampFormat == "" {
			newS3s[i].TimestampFormat = defaultS3TimestampFormat
		}
		newS3s[i].Path = r.Replace(newS3s[i].Path)
		newS3s[i].BucketName = r.Replace(newS3s[i].BucketName)
	}

	existingS3s, _, err := client.S3.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, s3 := range existingS3s {
		var match bool
		// Zero out read-only fields that we don't want to compare
		s3.ServiceID = ""
		s3.Version = 0
		for i, newS3 := range newS3s {
			if *s3 == newS3 {
				log.Debug(fmt.Sprintf("Found matching s3 %s. Not creating.\n", s3.Name))
				newS3s = append(newS3s[:i], newS3s[i+1:]...)
				match = true
				break
			} else if s3.Name == newS3.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing s3 %s. Updating.\n", s3.Name))
				if _, _, err := client.S3.Update(s.ID, newversion.Number, s3.Name, &newS3); err != nil {
					return err
				}
				newS3s = append(newS3s[:i], newS3s[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching s3 %s. Deleting.\n", s3.Name))
			_, err := client.S3.Delete(s.ID, newversion.Number, s3.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, s3 := range newS3s {
		if s3 == (fastly.S3{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing s3 %s.\n", s3.Name))
		_, _, err := client.S3.Create(s.ID, newversion.Number, &s3)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncHeaders(client *fastly.Client, s *fastly.Service, newHeaders []fastly.Header) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingHeaders, _, err := client.Header.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, header := range existingHeaders {
		var match bool
		// Zero out read-only fields that we don't want to compare
		header.ServiceID = ""
		header.Version = 0
		for i, newHeader := range newHeaders {
			if *header == newHeader {
				log.Debug(fmt.Sprintf("Found matching header %s. Not creating.\n", header.Name))
				newHeaders = append(newHeaders[:i], newHeaders[i+1:]...)
				match = true
				break
			} else if header.Name == newHeader.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing header %s. Updating.\n", header.Name))
				if _, _, err := client.Header.Update(s.ID, newversion.Number, header.Name, &newHeader); err != nil {
					return err
				}
				newHeaders = append(newHeaders[:i], newHeaders[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching header %s. Deleting.\n", header.Name))
			_, err := client.Header.Delete(s.ID, newversion.Number, header.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, header := range newHeaders {
		if header == (fastly.Header{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing header %s.\n", header.Name))
		_, _, err := client.Header.Create(s.ID, newversion.Number, &header)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncCacheSettings(client *fastly.Client, s *fastly.Service, newCacheSettings []fastly.CacheSetting) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingCacheSettings, _, err := client.CacheSetting.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, cacheSetting := range existingCacheSettings {
		var match bool
		// Zero out read-only fields that we don't want to compare
		cacheSetting.ServiceID = ""
		cacheSetting.Version = 0
		for i, newCacheSetting := range newCacheSettings {
			if *cacheSetting == newCacheSetting {
				log.Debug(fmt.Sprintf("Found matching cache setting %s. Not creating.\n", cacheSetting.Name))
				newCacheSettings = append(newCacheSettings[:i], newCacheSettings[i+1:]...)
				match = true
				break
			} else if cacheSetting.Name == newCacheSetting.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing cache setting %s. Updating.\n", cacheSetting.Name))
				if _, _, err := client.CacheSetting.Update(s.ID, newversion.Number, cacheSetting.Name, &newCacheSetting); err != nil {
					return err
				}
				newCacheSettings = append(newCacheSettings[:i], newCacheSettings[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching cache setting %s. Deleting.\n", cacheSetting.Name))
			_, err := client.CacheSetting.Delete(s.ID, newversion.Number, cacheSetting.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, cacheSetting := range newCacheSettings {
		if cacheSetting == (fastly.CacheSetting{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing cache setting %s.\n", cacheSetting.Name))
		_, _, err := client.CacheSetting.Create(s.ID, newversion.Number, &cacheSetting)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncRequestSettings(client *fastly.Client, s *fastly.Service, newRequestSettings []fastly.RequestSetting) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingRequestSettings, _, err := client.RequestSetting.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, requestSetting := range existingRequestSettings {
		var match bool
		// Zero out read-only fields that we don't want to compare
		requestSetting.ServiceID = ""
		requestSetting.Version = 0
		for i, newRequestSetting := range newRequestSettings {
			if *requestSetting == newRequestSetting {
				log.Debug(fmt.Sprintf("Found matching request setting %s. Not creating.\n", requestSetting.Name))
				newRequestSettings = append(newRequestSettings[:i], newRequestSettings[i+1:]...)
				match = true
				break
			} else if requestSetting.Name == newRequestSetting.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing request setting %s. Updating.\n", requestSetting.Name))
				if _, _, err := client.RequestSetting.Update(s.ID, newversion.Number, requestSetting.Name, &newRequestSetting); err != nil {
					return err
				}
				newRequestSettings = append(newRequestSettings[:i], newRequestSettings[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching request setting %s. Deleting.\n", requestSetting.Name))
			_, err := client.RequestSetting.Delete(s.ID, newversion.Number, requestSetting.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, requestSetting := range newRequestSettings {
		if requestSetting == (fastly.RequestSetting{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing request setting %s.\n", requestSetting.Name))
		_, _, err := client.RequestSetting.Create(s.ID, newversion.Number, &requestSetting)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncResponseObjects(client *fastly.Client, s *fastly.Service, newResponseObjects []fastly.ResponseObject) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingResponseObjects, _, err := client.ResponseObject.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, responseObject := range existingResponseObjects {
		var match bool
		// Zero out read-only fields that we don't want to compare
		responseObject.ServiceID = ""
		responseObject.Version = 0
		for i, newResponseObject := range newResponseObjects {
			if *responseObject == newResponseObject {
				log.Debug(fmt.Sprintf("Found matching response object %s. Not creating.\n", responseObject.Name))
				newResponseObjects = append(newResponseObjects[:i], newResponseObjects[i+1:]...)
				match = true
				break
			} else if responseObject.Name == newResponseObject.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing response object %s. Updating.\n", responseObject.Name))
				if _, _, err := client.ResponseObject.Update(s.ID, newversion.Number, responseObject.Name, &newResponseObject); err != nil {
					return err
				}
				newResponseObjects = append(newResponseObjects[:i], newResponseObjects[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching response object %s. Deleting.\n", responseObject.Name))
			_, err := client.ResponseObject.Delete(s.ID, newversion.Number, responseObject.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, responseObject := range newResponseObjects {
		if responseObject == (fastly.ResponseObject{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing response object %s.\n", responseObject.Name))
		_, _, err := client.ResponseObject.Create(s.ID, newversion.Number, &responseObject)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncConditions(client *fastly.Client, s *fastly.Service, newConditions []fastly.Condition) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingConditions, _, err := client.Condition.List(s.ID, newversion.Number)
	if err != nil {
		return err
	}
	for _, condition := range existingConditions {
		var match bool
		// Zero out read-only fields that we don't want to compare
		condition.ServiceID = ""
		condition.Version = 0
		for i, newCondition := range newConditions {
			if *condition == newCondition {
				log.Debug(fmt.Sprintf("Found matching condition %s. Not creating.\n", condition.Name))
				newConditions = append(newConditions[:i], newConditions[i+1:]...)
				match = true
				break
			} else if condition.Name == newCondition.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing condition %s. Updating.\n", condition.Name))
				if _, _, err := client.Condition.Update(s.ID, newversion.Number, condition.Name, &newCondition); err != nil {
					return err
				}
				newConditions = append(newConditions[:i], newConditions[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching condition %s. Deleting.\n", condition.Name))
			_, err := client.Condition.Delete(s.ID, newversion.Number, condition.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, condition := range newConditions {
		if condition == (fastly.Condition{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing condition %s.\n", condition.Name))
		_, _, err := client.Condition.Create(s.ID, newversion.Number, &condition)
		if err != nil {
			return err
		}
	}
	return nil
}

// Returns true if we made any changes, as that means we are activatable
// despite there being no diff.
func syncDictionaries(client *fastly.Client, s *fastly.Service, newDictionaries []fastly.Dictionary) (bool, error) {
	var changesMade bool
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return changesMade, err
	}

	existingDictionaries, _, err := client.Dictionary.List(s.ID, newversion.Number)
	if err != nil {
		return changesMade, err
	}
	for _, dictionary := range existingDictionaries {
		var match bool
		// Zero out read-only fields that we don't want to compare
		dictionary.ServiceID = ""
		dictionary.Version = 0
		dictionary.ID = ""
		for i, newDictionary := range newDictionaries {
			if *dictionary == newDictionary {
				log.Debug(fmt.Sprintf("Found matching dictionary %s. Not creating.\n", dictionary.Name))
				newDictionaries = append(newDictionaries[:i], newDictionaries[i+1:]...)
				match = true
				break
			} else if dictionary.Name == newDictionary.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing dictionary %s. Updating.\n", dictionary.Name))
				if _, _, err := client.Dictionary.Update(s.ID, newversion.Number, dictionary.Name, &newDictionary); err != nil {
					return changesMade, err
				}
				changesMade = true
				newDictionaries = append(newDictionaries[:i], newDictionaries[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching dictionary %s. Deleting.\n", dictionary.Name))
			_, err := client.Dictionary.Delete(s.ID, newversion.Number, dictionary.Name)
			if err != nil {
				return changesMade, err
			}
			changesMade = true
		}
	}

	for _, dictionary := range newDictionaries {
		if dictionary == (fastly.Dictionary{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing dictionary %s.\n", dictionary.Name))
		_, _, err := client.Dictionary.Create(s.ID, newversion.Number, &dictionary)
		if err != nil {
			return changesMade, err
		}
		changesMade = true
	}
	return changesMade, nil
}

// Returns true if we made any changes, as that means we are activatable
// despite there being no diff.
func syncACLs(client *fastly.Client, s *fastly.Service, newACLs []fastly.ACL) (bool, error) {
	var changesMade bool
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return changesMade, err
	}

	existingACLs, _, err := client.ACL.List(s.ID, newversion.Number)
	if err != nil {
		return changesMade, err
	}
	for _, acl := range existingACLs {
		var match bool
		// Zero out read-only fields that we don't want to compare
		acl.ServiceID = ""
		acl.Version = 0
		acl.ID = ""
		for i, newACL := range newACLs {
			if *acl == newACL {
				log.Debug(fmt.Sprintf("Found matching acl %s. Not creating.\n", acl.Name))
				newACLs = append(newACLs[:i], newACLs[i+1:]...)
				match = true
				break
			} else if acl.Name == newACL.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing acl %s. Updating.\n", acl.Name))
				if _, _, err := client.ACL.Update(s.ID, newversion.Number, acl.Name, &newACL); err != nil {
					return changesMade, err
				}
				changesMade = true
				newACLs = append(newACLs[:i], newACLs[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching acl %s. Deleting.\n", acl.Name))
			_, err := client.ACL.Delete(s.ID, newversion.Number, acl.Name)
			if err != nil {
				return changesMade, err
			}
			changesMade = true
		}
	}

	for _, acl := range newACLs {
		if acl == (fastly.ACL{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing acl %s.\n", acl.Name))
		_, _, err := client.ACL.Create(s.ID, newversion.Number, &acl)
		if err != nil {
			return changesMade, err
		}
		changesMade = true
	}
	return changesMade, nil
}

// Checks to ensure that only one of the given parameters has a non-zero value.
// Returns true if the parameters meet this requirement.
func checkMutuallyExclusive(a, b, c, d string) bool {
	count := 0
	if a != "" {
		count++
	}
	if b != "" {
		count++
	}
	if c != "" {
		count++
	}
	if d != "" {
		count++
	}
	if count > 1 {
		return false
	}
	return true
}

func syncBackends(client *fastly.Client, s *fastly.Service, newBackends []fastly.Backend) (bool, error) {
	var changesMade bool
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return changesMade, err
	}

	r := strings.NewReplacer("_servicename_", s.Name, "_prefix_", siteConfigs[s.Name].IPPrefix, "_suffix_", siteConfigs[s.Name].IPSuffix)
	for i, b := range newBackends {
		newBackends[i].Address = r.Replace(b.Address)
		newBackends[i].Hostname = r.Replace(b.Hostname)
		newBackends[i].IPV4 = r.Replace(b.IPV4)
		newBackends[i].IPV6 = r.Replace(b.IPV6)
		newBackends[i].SSLCertHostname = r.Replace(b.SSLCertHostname)
	}
	for i, b := range newBackends {
		if !checkMutuallyExclusive(b.Address, b.Hostname, b.IPV4, b.IPV6) {
			return changesMade, fmt.Errorf("Backend %s can only have one of Address, Hostname, IPV4, or IPV6 specified.", b.Name)
		}
		// The Address field is automatically filled by the API with
		// the Hostname, IPV4, or IPV6 value if one of those are
		// specified. Vice versa is also true. We must duplicate this
		// logic locally so the comparison works properly.
		if b.Address != "" {
			parsed := net.ParseIP(b.Address)
			if parsed != nil {
				if strings.Contains(":", parsed.String()) {
					newBackends[i].IPV6 = parsed.String()
				} else {
					newBackends[i].IPV4 = parsed.String()
				}
			} else {
				newBackends[i].Hostname = b.Address
			}
		} else if b.Hostname != "" {
			newBackends[i].Address = b.Hostname
		} else if b.IPV4 != "" {
			newBackends[i].Address = b.IPV4
		} else if b.IPV6 != "" {
			newBackends[i].Address = b.IPV6
		}
	}

	existingBackends, _, err := client.Backend.List(s.ID, newversion.Number)
	if err != nil {
		return changesMade, err
	}
	for _, backend := range existingBackends {
		var match bool
		// Zero out read-only fields that we don't want to compare
		backend.ServiceID = ""
		backend.Version = 0
		for i, newBackend := range newBackends {
			if *backend == newBackend {
				log.Debug(fmt.Sprintf("Found matching backend %s. Not creating.\n", backend.Name))
				newBackends = append(newBackends[:i], newBackends[i+1:]...)
				match = true
				break
			} else if backend.Name == newBackend.Name {
				log.Debug(fmt.Sprintf("Found mismatched existing backend %s. Updating.\n", backend.Name))
				if _, _, err := client.Backend.Update(s.ID, newversion.Number, backend.Name, &newBackend); err != nil {
					return changesMade, err
				}
				changesMade = true
				newBackends = append(newBackends[:i], newBackends[i+1:]...)
				match = true
				break
			}
		}
		if !match {
			log.Debug(fmt.Sprintf("Found non-matching backend %s. Deleting.\n", backend.Name))
			_, err := client.Backend.Delete(s.ID, newversion.Number, backend.Name)
			if err != nil {
				return changesMade, err
			}
			changesMade = true
		}
	}

	for _, backend := range newBackends {
		if backend == (fastly.Backend{}) {
			continue
		}
		log.Debug(fmt.Sprintf("Creating missing backend %s.\n", backend.Name))
		_, _, err := client.Backend.Create(s.ID, newversion.Number, &backend)
		if err != nil {
			return changesMade, err
		}
		changesMade = true
	}
	return changesMade, nil
}

func syncService(client *fastly.Client, s *fastly.Service) error {
	activeVersion, err := util.GetActiveVersion(s)
	if err != nil {
		return err
	}
	var config SiteConfig
	if _, ok := siteConfigs[s.Name]; ok {
		config = siteConfigs[s.Name]
	} else {
		config = siteConfigs["_default_"]
	}

	// If this var is set to true, then we must prompt for an activation
	// regardless of diff results. Some changes, such as ACL and Dict
	// creation, have no affect on the diff.
	var changesMade bool
	// Dictionaries, Conditions, health checks, and cache settings must be
	// sync'd first, as if they're referenced in any other object the API
	// will balk if they don't exist.
	log.Debug("Syncing Dictionaries\n")
	dictionaries := make([]fastly.Dictionary, len(config.Dictionaries))
	copy(dictionaries, config.Dictionaries)
	if changesMade, err = syncDictionaries(client, s, dictionaries); err != nil {
		return fmt.Errorf("Error syncing Dictionaries: %s", err)
	}

	log.Debug("Syncing ACLs\n")
	acls := make([]fastly.ACL, len(config.ACLs))
	copy(acls, config.ACLs)
	if changesMade, err = syncACLs(client, s, acls); err != nil {
		return fmt.Errorf("Error syncing ACLs: %s", err)
	}

	log.Debug("Syncing conditions\n")
	conditions := make([]fastly.Condition, len(config.Conditions))
	copy(conditions, config.Conditions)
	if err := syncConditions(client, s, conditions); err != nil {
		return fmt.Errorf("Error syncing conditions: %s", err)
	}

	log.Debug("Syncing health checks\n")
	healthChecks := make([]fastly.HealthCheck, len(config.HealthChecks))
	copy(healthChecks, config.HealthChecks)
	if err := syncHealthChecks(client, s, healthChecks); err != nil {
		return fmt.Errorf("Error syncing health checks: %s", err)
	}

	log.Debug("Syncing cache settings\n")
	cacheSettings := make([]fastly.CacheSetting, len(config.CacheSettings))
	copy(cacheSettings, config.CacheSettings)
	if err := syncCacheSettings(client, s, cacheSettings); err != nil {
		return fmt.Errorf("Error syncing cache settings: %s", err)
	}

	log.Debug("Syncing response objects\n")
	responseObjects := make([]fastly.ResponseObject, len(config.ResponseObject))
	copy(responseObjects, config.ResponseObject)
	if err = syncResponseObjects(client, s, responseObjects); err != nil {
		return fmt.Errorf("Error syncing response objects: %s", err)
	}

	log.Debug("Syncing request settings\n")
	requestSettings := make([]fastly.RequestSetting, len(config.RequestSettings))
	copy(requestSettings, config.RequestSettings)
	if err = syncRequestSettings(client, s, requestSettings); err != nil {
		return fmt.Errorf("Error syncing request settings: %s", err)
	}

	log.Debug("Syncing backends\n")
	backends := make([]fastly.Backend, len(config.Backends))
	copy(backends, config.Backends)
	if changesMade, err = syncBackends(client, s, backends); err != nil {
		return fmt.Errorf("Error syncing backends: %s", err)
	}

	log.Debug("Syncing headers\n")
	headers := make([]fastly.Header, len(config.Headers))
	copy(headers, config.Headers)
	if err := syncHeaders(client, s, headers); err != nil {
		return fmt.Errorf("Error syncing headers: %s", err)
	}

	log.Debug("Syncing syslogs\n")
	syslogs := make([]fastly.Syslog, len(config.Syslogs))
	copy(syslogs, config.Syslogs)
	if err := syncSyslogs(client, s, syslogs); err != nil {
		return fmt.Errorf("Error syncing syslogs: %s", err)
	}

	log.Debug("Syncing S3s\n")
	s3s := make([]fastly.S3, len(config.S3s))
	copy(s3s, config.S3s)
	if err := syncS3s(client, s, s3s); err != nil {
		return fmt.Errorf("Error syncing s3s: %s", err)
	}

	log.Debug("Syncing domains\n")
	domains := make([]fastly.Domain, len(config.Domains))
	copy(domains, config.Domains)
	if err := syncDomains(client, s, domains); err != nil {
		return fmt.Errorf("Error syncing domains: %s", err)
	}

	log.Debug("Syncing settings\n")
	if err := syncSettings(client, s, config.Settings); err != nil {
		return fmt.Errorf("Error syncing settings: %s", err)
	}

	log.Debug("Syncing gzips\n")
	gzips := make([]fastly.Gzip, len(config.Gzips))
	copy(gzips, config.Gzips)
	if err := syncGzips(client, s, gzips); err != nil {
		return fmt.Errorf("Error syncing gzips: %s", err)
	}

	log.Debug("Syncing VCLs\n")
	vcls := make([]VCL, len(config.VCLs))
	copy(vcls, config.VCLs)
	if err := syncVCLs(client, s, vcls); err != nil {
		return fmt.Errorf("Error syncing VCLs: %s", err)
	}

	if version, ok := pendingVersions[s.ID]; ok {
		equal, err := util.VersionsEqual(client, s, activeVersion, version.Number)
		if err != nil {
			return err
		}
		if equal && !changesMade {
			fmt.Printf("No changes for service %s\n", s.Name)
			delete(pendingVersions, s.ID)
			return nil
		}
	}

	return nil
}

func syncConfig(c *cli.Context) error {
	fastlyKey := c.GlobalString("fastly-key")
	configFile := c.GlobalString("config")

	client := fastly.NewClient(nil, fastlyKey)

	if err := readConfig(configFile); err != nil {
		return cli.NewExitError(fmt.Sprintf("Error reading config file: %s", err), -1)
	}
	pendingVersions = make(map[string]fastly.Version)

	services, _, err := client.Service.List()
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error listing services: %s", err), -1)
	}

	foundService := false
	// TODO Prompt if a version requested to be updated does not exist in Fastly, or
	// provide a way to autocreate it.
	for _, s := range services {
		// Only configure services for which configs have been specified
		if _, ok := siteConfigs[s.Name]; !ok {
			continue
		}
		if !c.Bool("all") && !util.StringInSlice(s.Name, c.Args()) {
			continue
		}
		foundService = true
		fmt.Println("Syncing ", s.Name)
		if err = syncService(client, s); err != nil {
			return cli.NewExitError(fmt.Sprintf("Error syncing service config for %s: %s", s.Name, err), -1)
		}
		if version, ok := pendingVersions[s.ID]; ok {
			if err = util.ValidateVersion(client, s, version.Number); err != nil {
				return cli.NewExitError(err.Error(), -1)
			}
			if err = util.ActivateVersion(c, client, s, &version); err != nil {
				return cli.NewExitError(fmt.Sprintf("Error activating pending version %s for service %s: %s", version.Number, s.Name, err), -1)
			}
		}
	}
	if !foundService {
		return cli.NewExitError(fmt.Sprintf("No matching services could be found to be sync'd."), -1)
	}
	return nil
}
