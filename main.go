package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/alienth/go-fastly"
	"github.com/imdario/mergo"
	"github.com/urfave/cli"
)

var pendingVersions map[string]fastly.Version
var siteConfigs map[string]SiteConfig
var debug bool

type SiteConfig struct {
	Settings         fastly.UpdateSettingsInput
	Domains          []fastly.CreateDomainInput
	Backends         []fastly.CreateBackendInput
	Conditions       []fastly.CreateConditionInput
	CacheSettings    []fastly.CreateCacheSettingInput
	Headers          []fastly.CreateHeaderInput
	S3s              []fastly.CreateS3Input
	FTPs             []fastly.CreateFTPInput
	GCSs             []fastly.CreateGCSInput
	Papertrails      []fastly.CreatePapertrailInput
	Sumologics       []fastly.CreateSumologicInput
	Syslogs          []fastly.CreateSyslogInput
	Gzips            []fastly.CreateGzipInput
	Directors        []fastly.CreateDirectorInput
	DirectorBackends []fastly.CreateDirectorBackendInput
	HealthChecks     []fastly.CreateHealthCheckInput
	VCLs             []VCL

	// Override for backend SSLCertHostnames
	// Used in cases where _servicename_ is not sufficient for defining
	// an SSL hostname, such as when Fastly has a cert which we do not
	// have on the origin.
	SSLCertHostname string

	IPPrefix string
	IPSuffix string
}

type VCL struct {
	Name    string
	Content []byte
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

	outfile, _ := os.OpenFile("out.toml", os.O_CREATE|os.O_RDWR, 0644)
	encoder := toml.NewEncoder(outfile)
	encoder.Encode(&siteConfigs)
	outfile.Close()

	outfile, _ = os.OpenFile("out.json", os.O_CREATE|os.O_RDWR, 0644)
	jencoder := json.NewEncoder(outfile)
	jencoder.Encode(&siteConfigs)
	outfile.Close()

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

func syncVCLs(client *fastly.Client, s *fastly.Service, newVCLs []VCL) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}
	existingVCLs, err := client.ListVCLs(&fastly.ListVCLsInput{Service: s.ID, Version: newversion.Number})
	if err != nil {
		return err
	}
	for _, vcl := range existingVCLs {
		if err = client.DeleteVCL(&fastly.DeleteVCLInput{Service: s.ID, Name: vcl.Name, Version: newversion.Number}); err != nil {
			return err
		}
	}

	for _, vcl := range newVCLs {
		var content []byte
		if vcl.File != "" && vcl.Content != nil {
			return fmt.Errorf("Cannot specify both a File and Content for VCL %s", vcl.Name)
		}
		if vcl.File != "" {
			if content, err = ioutil.ReadFile(vcl.File); err != nil {
				return err
			}
		} else if vcl.Content != nil {
			content = vcl.Content
		} else {
			return fmt.Errorf("No Content or File specified for VCL %s", vcl.Name)
		}

		var i fastly.CreateVCLInput
		i.Name = vcl.Name
		i.Service = s.ID
		i.Version = newversion.Number
		i.Content = string(content)

		if _, err = client.CreateVCL(&i); err != nil {
			return err
		}
		if vcl.Main {
			// Activate actually toggles a VCL to be the 'main' one
			if _, err := client.ActivateVCL(&fastly.ActivateVCLInput{Name: vcl.Name, Service: s.ID, Version: newversion.Number}); err != nil {
				return err
			}
		}
	}
	return nil
}

func syncHealthChecks(client *fastly.Client, s *fastly.Service, newHealthChecks []fastly.CreateHealthCheckInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingHealthChecks, err := client.ListHealthChecks(&fastly.ListHealthChecksInput{Service: s.ID, Version: newversion.Number})
	for _, healthCheck := range existingHealthChecks {
		err := client.DeleteHealthCheck(&fastly.DeleteHealthCheckInput{Service: s.ID, Name: healthCheck.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, healthCheck := range newHealthChecks {
		healthCheck.Version = newversion.Number
		healthCheck.Service = s.ID
		if _, err = client.CreateHealthCheck(&healthCheck); err != nil {
			return err
		}

	}
	return nil
}

func syncDirectorBackends(client *fastly.Client, s *fastly.Service, newMappings []fastly.CreateDirectorBackendInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	for _, mapping := range newMappings {
		mapping.Service = s.ID
		mapping.Version = newversion.Number
		if _, err = client.CreateDirectorBackend(&mapping); err != nil {
			return err
		}

	}
	return nil
}

func syncDirectors(client *fastly.Client, s *fastly.Service, newDirectors []fastly.CreateDirectorInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingDirectors, err := client.ListDirectors(&fastly.ListDirectorsInput{Service: s.ID, Version: newversion.Number})
	for _, director := range existingDirectors {
		err := client.DeleteDirector(&fastly.DeleteDirectorInput{Service: s.ID, Name: director.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, director := range newDirectors {
		director.Version = newversion.Number
		director.Service = s.ID
		if _, err = client.CreateDirector(&director); err != nil {
			return err
		}

	}
	return nil
}

func syncGzips(client *fastly.Client, s *fastly.Service, newGzips []fastly.CreateGzipInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingGzips, err := client.ListGzips(&fastly.ListGzipsInput{Service: s.ID, Version: newversion.Number})
	for _, gzip := range existingGzips {
		err := client.DeleteGzip(&fastly.DeleteGzipInput{Service: s.ID, Name: gzip.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, gzip := range newGzips {
		gzip.Version = newversion.Number
		gzip.Service = s.ID
		if _, err = client.CreateGzip(&gzip); err != nil {
			return err
		}
	}
	return nil
}

func syncSettings(client *fastly.Client, s *fastly.Service, newSettings fastly.UpdateSettingsInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}
	newSettings.Service = s.ID
	newSettings.Version = newversion.Number
	if _, err = client.UpdateSettings(&newSettings); err != nil {
		return err
	}

	return nil
}

func syncDomains(client *fastly.Client, s *fastly.Service, newDomains []fastly.CreateDomainInput) error {
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
	r := strings.NewReplacer("_servicename_", s.Name)
	for _, domain := range newDomains {
		domain.Name = r.Replace(domain.Name)
		domain.Service = s.ID
		domain.Version = newversion.Number
		if _, err = client.CreateDomain(&domain); err != nil {
			return err
		}
	}
	return nil
}

func syncSyslogs(client *fastly.Client, s *fastly.Service, newSyslogs []fastly.CreateSyslogInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingSyslogs, err := client.ListSyslogs(&fastly.ListSyslogsInput{Service: s.ID, Version: newversion.Number})
	for _, syslog := range existingSyslogs {
		err := client.DeleteSyslog(&fastly.DeleteSyslogInput{Service: s.ID, Name: syslog.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, syslog := range newSyslogs {
		syslog.Service = s.ID
		syslog.Version = newversion.Number
		if _, err = client.CreateSyslog(&syslog); err != nil {
			return err
		}
	}
	return nil
}

func syncPapertrails(client *fastly.Client, s *fastly.Service, newPapertrails []fastly.CreatePapertrailInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingPapertrails, err := client.ListPapertrails(&fastly.ListPapertrailsInput{Service: s.ID, Version: newversion.Number})
	for _, papertrail := range existingPapertrails {
		err := client.DeletePapertrail(&fastly.DeletePapertrailInput{Service: s.ID, Name: papertrail.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, papertrail := range newPapertrails {
		papertrail.Name = papertrail.Name
		papertrail.Service = s.ID
		if _, err = client.CreatePapertrail(&papertrail); err != nil {
			return err
		}
	}
	return nil
}

func syncSumologics(client *fastly.Client, s *fastly.Service, newSumologics []fastly.CreateSumologicInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingSumologics, err := client.ListSumologics(&fastly.ListSumologicsInput{Service: s.ID, Version: newversion.Number})
	for _, sumologic := range existingSumologics {
		err := client.DeleteSumologic(&fastly.DeleteSumologicInput{Service: s.ID, Name: sumologic.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, sumologic := range newSumologics {
		sumologic.Service = s.ID
		sumologic.Version = newversion.Number
		if _, err = client.CreateSumologic(&sumologic); err != nil {
			return err
		}

	}
	return nil
}

func syncFTPs(client *fastly.Client, s *fastly.Service, newFTPs []fastly.CreateFTPInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingFTPs, err := client.ListFTPs(&fastly.ListFTPsInput{Service: s.ID, Version: newversion.Number})
	for _, ftp := range existingFTPs {
		err := client.DeleteFTP(&fastly.DeleteFTPInput{Service: s.ID, Name: ftp.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, ftp := range newFTPs {
		ftp.Service = s.ID
		ftp.Version = newversion.Number
		if _, err = client.CreateFTP(&ftp); err != nil {
			return err
		}
	}
	return nil
}

func syncGCSs(client *fastly.Client, s *fastly.Service, newGCSs []fastly.CreateGCSInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingGCSs, err := client.ListGCSs(&fastly.ListGCSsInput{Service: s.ID, Version: newversion.Number})
	for _, gcs := range existingGCSs {
		err := client.DeleteGCS(&fastly.DeleteGCSInput{Service: s.ID, Name: gcs.Name, Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, gcs := range newGCSs {
		gcs.Service = s.ID
		gcs.Version = newversion.Number
		if _, err = client.CreateGCS(&gcs); err != nil {
			return err
		}
	}
	return nil
}

func syncS3s(client *fastly.Client, s *fastly.Service, newS3s []fastly.CreateS3Input) error {
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
	r := strings.NewReplacer("_servicename_", s.Name)
	for _, s3 := range newS3s {
		s3.Service = s.ID
		s3.Version = newversion.Number
		s3.Path = r.Replace(s3.Path)
		s3.BucketName = r.Replace(s3.BucketName)
		if _, err = client.CreateS3(&s3); err != nil {
			return err
		}
	}
	return nil
}

func syncHeaders(client *fastly.Client, s *fastly.Service, newHeaders []fastly.CreateHeaderInput) error {
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
		if header == (fastly.CreateHeaderInput{}) {
			continue
		}
		header.Version = newversion.Number
		header.Service = s.ID
		if _, err = client.CreateHeader(&header); err != nil {
			return err
		}

	}
	return nil
}

func syncCacheSettings(client *fastly.Client, s *fastly.Service, newCacheSettings []fastly.CreateCacheSettingInput) error {
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
		if setting == (fastly.CreateCacheSettingInput{}) {
			continue
		}
		setting.Service = s.ID
		setting.Version = newversion.Number
		if _, err = client.CreateCacheSetting(&setting); err != nil {
			return err
		}

	}
	return nil
}

func syncConditions(client *fastly.Client, s *fastly.Service, newConditions []fastly.CreateConditionInput) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	existingConditions, err := client.ListConditions(&fastly.ListConditionsInput{Service: s.ID, Version: newversion.Number})
	if err != nil {
		return err
	}
	r := strings.NewReplacer("/", "%2f")
	for _, condition := range existingConditions {
		err := client.DeleteCondition(&fastly.DeleteConditionInput{Service: s.ID, Name: r.Replace(condition.Name), Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, condition := range newConditions {
		condition.Service = s.ID
		condition.Version = newversion.Number
		if _, err = client.CreateCondition(&condition); err != nil {
			return err
		}

	}
	return nil
}

func syncBackends(client *fastly.Client, s *fastly.Service, newBackends []fastly.CreateBackendInput) error {
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
	r := strings.NewReplacer("_servicename_", s.Name, "_prefix_", siteConfigs[s.Name].IPPrefix, "_suffix_", siteConfigs[s.Name].IPSuffix)
	for _, backend := range newBackends {
		backend.Address = r.Replace(backend.Address)
		backend.Service = newversion.ServiceID
		backend.Version = newversion.Number
		backend.SSLCertHostname = r.Replace(backend.SSLCertHostname)
		if _, err = client.CreateBackend(&backend); err != nil {
			return err
		}
	}

	return nil
}

func syncService(client *fastly.Client, s *fastly.Service) error {
	var activeVersion = strconv.Itoa(int(s.ActiveVersion))
	var config SiteConfig
	if _, ok := siteConfigs[s.Name]; ok {
		config = siteConfigs[s.Name]
	} else {
		config = siteConfigs["_default_"]
	}

	// Conditions, health checks, and cache settings must be sync'd first, as if they're
	// referenced in any other object the API will balk if they don't exist.
	remoteConditions, err := client.ListConditions(&fastly.ListConditionsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.Conditions) == 0 && len(remoteConditions) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "conditions", s.Name)
		}
		if err := syncConditions(client, s, config.Conditions); err != nil {
			return fmt.Errorf("Error syncing conditions: %s", err)
		}
	}

	remoteHealthChecks, _ := client.ListHealthChecks(&fastly.ListHealthChecksInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.HealthChecks) == 0 && len(remoteHealthChecks) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "health checks", s.Name)
		}
		if err := syncHealthChecks(client, s, config.HealthChecks); err != nil {
			return fmt.Errorf("Error syncing health checks: %s", err)
		}
	}

	remoteCacheSettings, _ := client.ListCacheSettings(&fastly.ListCacheSettingsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.CacheSettings) == 0 && len(remoteCacheSettings) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "cache settings", s.Name)
		}
		if err := syncCacheSettings(client, s, config.CacheSettings); err != nil {
			return fmt.Errorf("Error syncing cache settings: %s", err)
		}
	}

	remoteBackends, err := client.ListBackends(&fastly.ListBackendsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.Backends) == 0 && len(remoteBackends) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "backends", s.Name)
		}
		if err := syncBackends(client, s, config.Backends); err != nil {
			return fmt.Errorf("Error syncing backends: %s", err)
		}
	}

	remoteHeaders, _ := client.ListHeaders(&fastly.ListHeadersInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.Headers) == 0 && len(remoteHeaders) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "headers", s.Name)
		}
		if err := syncHeaders(client, s, config.Headers); err != nil {
			return fmt.Errorf("Error syncing headers: %s", err)
		}
	}

	remoteSyslogs, _ := client.ListSyslogs(&fastly.ListSyslogsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.Syslogs) == 0 && len(remoteSyslogs) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "syslogs", s.Name)
		}
		if err := syncSyslogs(client, s, config.Syslogs); err != nil {
			return fmt.Errorf("Error syncing syslogs: %s", err)
		}
	}

	remotePapertrails, _ := client.ListPapertrails(&fastly.ListPapertrailsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.Papertrails) == 0 && len(remotePapertrails) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "papertrails", s.Name)
		}
		if err := syncPapertrails(client, s, config.Papertrails); err != nil {
			return fmt.Errorf("Error syncing papertrail: %s", err)
		}
	}

	remoteSumologics, _ := client.ListSumologics(&fastly.ListSumologicsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.Sumologics) == 0 && len(remoteSumologics) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "sumologics", s.Name)
		}
		if err := syncSumologics(client, s, config.Sumologics); err != nil {
			return fmt.Errorf("Error syncing sumologics: %s", err)
		}
	}

	remoteFTPs, _ := client.ListFTPs(&fastly.ListFTPsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.FTPs) == 0 && len(remoteFTPs) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "ftps", s.Name)
		}
		if err := syncFTPs(client, s, config.FTPs); err != nil {
			return fmt.Errorf("Error syncing ftps: %s", err)
		}
	}

	remoteGCSs, _ := client.ListGCSs(&fastly.ListGCSsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.GCSs) == 0 && len(remoteGCSs) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "gcss", s.Name)
		}
		if err := syncGCSs(client, s, config.GCSs); err != nil {
			return err
		}
	}

	remoteS3s, _ := client.ListS3s(&fastly.ListS3sInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.S3s) == 0 && len(remoteS3s) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "s3s", s.Name)
		}
		if err := syncS3s(client, s, config.S3s); err != nil {
			return fmt.Errorf("Error syncing s3s: %s", err)
		}
	}

	remoteDomains, _ := client.ListDomains(&fastly.ListDomainsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.Domains) == 0 && len(remoteDomains) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "domains", s.Name)
		}
		if err := syncDomains(client, s, config.Domains); err != nil {
			return fmt.Errorf("Error syncing domains: %s", err)
		}
	}

	if debug {
		fmt.Printf("Syncing %s for %s\n", "settings", s.Name)
	}
	if err := syncSettings(client, s, config.Settings); err != nil {
		return fmt.Errorf("Error syncing settings: %s", err)
	}

	remoteGzips, _ := client.ListGzips(&fastly.ListGzipsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.Gzips) == 0 && len(remoteGzips) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "gzips", s.Name)
		}
		if err := syncGzips(client, s, config.Gzips); err != nil {
			return fmt.Errorf("Error syncing gzips: %s", err)
		}
	}

	remoteVCLs, _ := client.ListVCLs(&fastly.ListVCLsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !(len(config.VCLs) == 0 && len(remoteVCLs) == 0) {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "VCLs", s.Name)
		}
		if err := syncVCLs(client, s, config.VCLs); err != nil {
			return fmt.Errorf("Error syncing VCLs: %s", err)
		}
	}

	remoteDirectors, _ := client.ListDirectors(&fastly.ListDirectorsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	mappingsInSync := true
	for _, directorBackend := range config.DirectorBackends {
		// There is no way to list the DirectorBackend mappings, so we have to fetch
		// each and look for 404s.
		resp, err := client.Request("GET", fmt.Sprintf("/service/%s/version/%s/director/%s/backend/%s", s.ID, activeVersion, directorBackend.Director, directorBackend.Backend), nil)
		if err != nil && resp.StatusCode == 404 {
			mappingsInSync = false
		} else if err != nil {
			return err
		}
	}
	if !(len(config.Directors) == 0 && len(remoteDirectors) == 0) || !mappingsInSync {
		if debug {
			fmt.Printf("Syncing %s for %s\n", "directors", s.Name)
		}
		if err := syncDirectors(client, s, config.Directors); err != nil {
			return fmt.Errorf("Error syncing directors: %s", err)
		}
		// Syncing directors will initially delete all directors, which implicitly
		// deletes all of the directorbackend mappings. As such, we must recreate.
		if err := syncDirectorBackends(client, s, config.DirectorBackends); err != nil {
			return fmt.Errorf("Error syncing director backend mappings: %s", err)
		}
	}

	if version, ok := pendingVersions[s.ID]; ok {
		var i fastly.GetDiffInput
		i.From = activeVersion
		i.To = version.Number
		i.Service = s.ID
		i.Format = "text"
		diff, _ := client.GetDiff(&i)
		ioutil.WriteFile(s.Name+".diff", []byte(diff.Diff), 0644)
		fmt.Println("wrote diff for ", s.Name)
	}

	return nil
}

func syncConfig(c *cli.Context) error {
	fastlyKey := c.GlobalString("fastly-key")
	configFile := c.GlobalString("config")
	if fastlyKey == "" {
		cli.ShowAppHelp(c)
		return cli.NewExitError("Error: Fastly API key must be set.", -1)
	}

	if (!c.Bool("all") && !c.Args().Present()) || (c.Bool("all") && c.Args().Present()) {
		cli.ShowAppHelp(c)
		return cli.NewExitError("Error: either specify service names to be syncd, or sync all with -a", -1)
	}
	client, err := fastly.NewClient(fastlyKey)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error initializing fastly client: %s", err), -1)
	}

	if err := readConfig(configFile); err != nil {
		return cli.NewExitError(fmt.Sprintf("Error reading config file: %s", err), -1)
	}
	pendingVersions = make(map[string]fastly.Version)

	services, err := client.ListServices(&fastly.ListServicesInput{})
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error listing services: %s", err), -1)
	}
	foundService := false
	for _, s := range services {
		// Only configure services for which configs have been specified
		if _, ok := siteConfigs[s.Name]; !ok {
			continue
		}
		if !c.Bool("all") && !stringInSlice(s.Name, c.Args()) {
			continue
		}
		foundService = true
		fmt.Println("Syncing ", s.Name)
		if err = syncService(client, s); err != nil {
			return cli.NewExitError(fmt.Sprintf("Error syncing service config for %s: %s", s.Name, err), -1)
		}
	}
	if !foundService {
		return cli.NewExitError(fmt.Sprintf("No matching services could be found to be sync'd."), -1)
	}
	return nil
}

func stringInSlice(check string, slice []string) bool {
	for _, element := range slice {
		if element == check {
			return true
		}
	}
	return false
}

func main() {
	app := cli.NewApp()
	app.Name = "fastlyctl"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "config.toml",
			Usage: "Load Fastly configuration from `FILE`",
		},
		cli.StringFlag{
			Name:   "fastly-key, K",
			Usage:  "Fastly API Key.",
			EnvVar: "FASTLY_KEY",
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "Print more detailed info for debugging.",
		},
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:    "sync",
			Aliases: []string{"s"},
			Usage:   "Sync remote service configuration with local config file.",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "all, a",
					Usage: "Sync all services listed in config file",
				},
			},
			Before: func(c *cli.Context) error {
				debug = c.GlobalBool("debug")
				return nil
			},
			Action: syncConfig,
		},
	}

	app.Run(os.Args)
}
