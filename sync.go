package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/alienth/fastlyctl/util"
	"github.com/alienth/go-fastly"
	"github.com/imdario/mergo"
	"github.com/urfave/cli"
)

var pendingVersions map[string]fastly.Version
var siteConfigs map[string]SiteConfig

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
	Dictionaries     []fastly.CreateDictionaryInput
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

// syncDictionaries actually compares the remote side dictionaries, unlike most other sync functions.
// This is because dictionary contents are not tied to a config version. If we were to delete the
// dictionaries here, we'd lose whatever keys had been added since creation.
// Returns true if we made any changes, as that means we are activatable despite there being no diff.
func syncDictionaries(client *fastly.Client, s *fastly.Service, newDictionaries []fastly.CreateDictionaryInput) (bool, error) {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return false, err
	}
	var needsCreation []*fastly.CreateDictionaryInput
	var needsDeletion []*fastly.DeleteDictionaryInput
	existingDictionaries, err := client.ListDictionaries(&fastly.ListDictionariesInput{Service: s.ID, Version: newversion.Number})
	if err != nil {
		return false, err
	}
	for _, d := range existingDictionaries {
		found := false
		for _, n := range newDictionaries {
			if d.Name == n.Name {
				found = true
			}
		}
		if !found {
			var i fastly.DeleteDictionaryInput
			i.Name = d.Name
			i.Service = s.ID
			i.Version = newversion.Number
			needsDeletion = append(needsDeletion, &i)

		}
	}
	for _, d := range newDictionaries {
		if d == (fastly.CreateDictionaryInput{}) {
			continue
		}
		found := false
		for _, n := range existingDictionaries {
			if d.Name == n.Name {
				found = true
			}
		}
		if !found {
			var i fastly.CreateDictionaryInput
			i.Name = d.Name
			i.Service = s.ID
			i.Version = newversion.Number
			needsCreation = append(needsCreation, &i)
		}
	}

	var needsSync bool
	for _, d := range needsCreation {
		needsSync = true
		debugPrint(fmt.Sprintf("\t creating dictionary: %s\n", d.Name))
		if _, err = client.CreateDictionary(d); err != nil {
			return false, err
		}
	}
	for _, d := range needsDeletion {
		needsSync = true
		debugPrint(fmt.Sprintf("\t deleting dictionary: %s\n", d.Name))
		if err = client.DeleteDictionary(d); err != nil {
			return false, err
		}
	}
	return needsSync, nil
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

	// Dictionaries, Conditions, health checks, and cache settings must be
	// sync'd first, as if they're referenced in any other object the API
	// will balk if they don't exist.
	var mustSync bool
	debugPrint("Syncing Dictionaries\n")
	if mustSync, err = syncDictionaries(client, s, config.Dictionaries); err != nil {
		return fmt.Errorf("Error syncing Dictionaries: %s", err)
	}

	debugPrint("Syncing conditions\n")
	if err := syncConditions(client, s, config.Conditions); err != nil {
		return fmt.Errorf("Error syncing conditions: %s", err)
	}

	debugPrint("Syncing health checks\n")
	if err := syncHealthChecks(client, s, config.HealthChecks); err != nil {
		return fmt.Errorf("Error syncing health checks: %s", err)
	}

	debugPrint("Syncing cache settings\n")
	if err := syncCacheSettings(client, s, config.CacheSettings); err != nil {
		return fmt.Errorf("Error syncing cache settings: %s", err)
	}

	debugPrint("Syncing backends\n")
	if err := syncBackends(client, s, config.Backends); err != nil {
		return fmt.Errorf("Error syncing backends: %s", err)
	}

	debugPrint("Syncing headers\n")
	if err := syncHeaders(client, s, config.Headers); err != nil {
		return fmt.Errorf("Error syncing headers: %s", err)
	}

	debugPrint("Syncing syslogs\n")
	if err := syncSyslogs(client, s, config.Syslogs); err != nil {
		return fmt.Errorf("Error syncing syslogs: %s", err)
	}

	debugPrint("Syncing papertrails\n")
	if err := syncPapertrails(client, s, config.Papertrails); err != nil {
		return fmt.Errorf("Error syncing papertrail: %s", err)
	}

	debugPrint("Syncing sumologics\n")
	if err := syncSumologics(client, s, config.Sumologics); err != nil {
		return fmt.Errorf("Error syncing sumologics: %s", err)
	}

	debugPrint("Syncing ftps\n")
	if err := syncFTPs(client, s, config.FTPs); err != nil {
		return fmt.Errorf("Error syncing ftps: %s", err)
	}

	debugPrint("Syncing GCSs\n")
	if err := syncGCSs(client, s, config.GCSs); err != nil {
		return err
	}

	debugPrint("Syncing S3s\n")
	if err := syncS3s(client, s, config.S3s); err != nil {
		return fmt.Errorf("Error syncing s3s: %s", err)
	}

	debugPrint("Syncing domains\n")
	if err := syncDomains(client, s, config.Domains); err != nil {
		return fmt.Errorf("Error syncing domains: %s", err)
	}

	debugPrint("Syncing settings\n")
	if err := syncSettings(client, s, config.Settings); err != nil {
		return fmt.Errorf("Error syncing settings: %s", err)
	}

	debugPrint("Syncing gzips\n")
	if err := syncGzips(client, s, config.Gzips); err != nil {
		return fmt.Errorf("Error syncing gzips: %s", err)
	}

	debugPrint("Syncing VCLs\n")
	if err := syncVCLs(client, s, config.VCLs); err != nil {
		return fmt.Errorf("Error syncing VCLs: %s", err)
	}

	debugPrint("Syncing directors\n")
	if err := syncDirectors(client, s, config.Directors); err != nil {
		return fmt.Errorf("Error syncing directors: %s", err)
	}

	// Syncing directors will initially delete all directors, which implicitly
	// deletes all of the directorbackend mappings. As such, we must recreate.
	debugPrint("Syncing director backends\n")
	if err := syncDirectorBackends(client, s, config.DirectorBackends); err != nil {
		return fmt.Errorf("Error syncing director backend mappings: %s", err)
	}

	if version, ok := pendingVersions[s.ID]; ok {
		equal, err := util.VersionsEqual(client, s, activeVersion, version.Number)
		if err != nil {
			return err
		}
		if equal && !mustSync {
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

	noop := c.Bool("noop")
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
			if !noop {
				if err = util.ActivateVersion(c, client, s, &version); err != nil {
					return cli.NewExitError(fmt.Sprintf("Error activating pending version %s for service %s: %s", version.Number, s.Name, err), -1)
				}
			}
		}
	}
	if !foundService {
		return cli.NewExitError(fmt.Sprintf("No matching services could be found to be sync'd."), -1)
	}
	return nil
}
