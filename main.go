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

	"github.com/BurntSushi/toml"
	"github.com/alienth/go-fastly"
	"github.com/imdario/mergo"
)

var pendingVersions map[string]fastly.Version
var siteConfigs map[string]SiteConfig

type SiteConfig struct {
	Settings         *fastly.Settings
	Domains          []*fastly.Domain
	Backends         []*fastly.Backend
	Conditions       []*fastly.Condition
	CacheSettings    []*fastly.CacheSetting
	Headers          []*fastly.Header
	S3s              []*fastly.S3
	FTPs             []*fastly.FTP
	GCSs             []*fastly.GCS
	Papertrails      []*fastly.Papertrail
	Sumologics       []*fastly.Sumologic
	Syslogs          []*fastly.Syslog
	Gzips            []*fastly.Gzip
	Directors        []*fastly.Director
	DirectorBackends []*fastly.DirectorBackend
	HealthChecks     []*fastly.HealthCheck

	// Override for backend SSLCertHostnames
	// Used in cases where _servicename_ is not sufficient for defining
	// an SSL hostname, such as when Fastly has a cert which we do not
	// have on the origin.
	SSLCertHostname string
}

func readConfig(file string) error {
	body, _ := ioutil.ReadFile(file)
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

	for name, config := range siteConfigs {
		if name == "_default_" {
			continue
		}

		if err := mergo.Merge(&config, siteConfigs["_default_"]); err != nil {
			return err
		}
		siteConfigs[name] = config
		for _, backend := range config.Backends {
			if config.SSLCertHostname != "" {
				backend.SSLCertHostname = config.SSLCertHostname
			}
			backend.SSLCertHostname = strings.Replace(backend.SSLCertHostname, "_servicename_", name, -1)
		}
		for _, s3 := range config.S3s {
			s3.Path = strings.Replace(s3.Path, "_servicename_", name, -1)
		}
		for _, domain := range config.Domains {
			domain.Name = strings.Replace(domain.Name, "_servicename_", name, -1)
		}
		for _, s3 := range config.S3s {
			s3.BucketName = strings.Replace(s3.BucketName, "_servicename_", name, -1)
			s3.Path = strings.Replace(s3.Path, "_servicename_", name, -1)
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

func syncHealthChecks(client *fastly.Client, s *fastly.Service, newHealthChecks []*fastly.HealthCheck) error {
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
		var i fastly.CreateHealthCheckInput

		i.Name = healthCheck.Name
		i.Version = newversion.Number
		i.Service = s.ID
		i.Host = healthCheck.Host
		i.Path = healthCheck.Host
		i.ExpectedResponse = healthCheck.ExpectedResponse
		i.CheckInterval = healthCheck.CheckInterval
		i.HTTPVersion = healthCheck.HTTPVersion
		i.Threshold = healthCheck.Threshold
		i.Initial = healthCheck.Initial
		i.Timeout = healthCheck.Timeout
		i.Window = healthCheck.Window
		i.Method = healthCheck.Method

		if _, err = client.CreateHealthCheck(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncDirectorBackends(client *fastly.Client, s *fastly.Service, newMappings []*fastly.DirectorBackend) error {
	newversion, err := prepareNewVersion(client, s)
	if err != nil {
		return err
	}

	for _, mapping := range newMappings {
		var i fastly.CreateDirectorBackendInput

		i.Service = s.ID
		i.Version = newversion.Number
		i.Backend = mapping.Backend
		i.Director = mapping.Director

		if _, err = client.CreateDirectorBackend(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncDirectors(client *fastly.Client, s *fastly.Service, newDirectors []*fastly.Director) error {
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
		var i fastly.CreateDirectorInput

		i.Name = director.Name
		i.Version = newversion.Number
		i.Service = s.ID
		i.Type = director.Type
		i.Comment = director.Comment
		i.Quorum = director.Quorum
		i.Retries = director.Retries

		if _, err = client.CreateDirector(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncGzips(client *fastly.Client, s *fastly.Service, newGzips []*fastly.Gzip) error {
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
		var i fastly.CreateGzipInput

		i.Name = gzip.Name
		i.Version = newversion.Number
		i.Service = s.ID
		i.Extensions = gzip.Extensions
		i.ContentTypes = gzip.ContentTypes
		i.CacheCondition = gzip.CacheCondition

		if _, err = client.CreateGzip(&i); err != nil {
			return err
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

func syncSyslogs(client *fastly.Client, s *fastly.Service, newSyslogs []*fastly.Syslog) error {
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
		var i fastly.CreateSyslogInput

		i.Name = syslog.Name
		i.Service = s.ID
		i.Version = newversion.Number
		i.Address = syslog.Address
		i.Port = syslog.Port
		i.UseTLS = fastly.Compatibool(syslog.UseTLS)
		i.TLSCACert = syslog.TLSCACert
		i.Token = syslog.Token
		i.Format = syslog.Format
		i.ResponseCondition = syslog.ResponseCondition

		if _, err = client.CreateSyslog(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncPapertrails(client *fastly.Client, s *fastly.Service, newPapertrails []*fastly.Papertrail) error {
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
		var i fastly.CreatePapertrailInput

		i.Name = papertrail.Name
		i.Service = s.ID
		i.Version = newversion.Number
		i.Address = papertrail.Address
		i.Port = papertrail.Port
		i.Format = papertrail.Format
		i.ResponseCondition = papertrail.ResponseCondition

		if _, err = client.CreatePapertrail(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncSumologics(client *fastly.Client, s *fastly.Service, newSumologics []*fastly.Sumologic) error {
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
		var i fastly.CreateSumologicInput

		i.Name = sumologic.Name
		i.Service = s.ID
		i.Version = newversion.Number
		i.Address = sumologic.Address
		i.Format = sumologic.Format
		i.URL = sumologic.URL
		i.ResponseCondition = sumologic.ResponseCondition

		if _, err = client.CreateSumologic(&i); err != nil {
			return err
		}

	}
	return nil
}

func syncFTPs(client *fastly.Client, s *fastly.Service, newFTPs []*fastly.FTP) error {
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
		var i fastly.CreateFTPInput

		i.Name = ftp.Name
		i.Service = s.ID
		i.Version = newversion.Number
		i.Path = ftp.Path
		i.Format = ftp.Format
		i.Period = ftp.Period
		i.TimestampFormat = ftp.TimestampFormat
		i.Username = ftp.Username
		i.Password = ftp.Password
		i.Address = ftp.Address
		i.GzipLevel = ftp.GzipLevel
		i.ResponseCondition = ftp.ResponseCondition

		if _, err = client.CreateFTP(&i); err != nil {
			return err
		}
	}
	return nil
}

func syncGCSs(client *fastly.Client, s *fastly.Service, newGCSs []*fastly.GCS) error {
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
		var i fastly.CreateGCSInput

		i.Name = gcs.Name
		i.Service = s.ID
		i.Version = newversion.Number
		i.Path = gcs.Path
		i.Format = gcs.Format
		i.Period = gcs.Period
		i.TimestampFormat = gcs.TimestampFormat
		i.Bucket = gcs.Bucket
		i.GzipLevel = gcs.GzipLevel
		i.SecretKey = gcs.SecretKey
		i.ResponseCondition = gcs.ResponseCondition

		if _, err = client.CreateGCS(&i); err != nil {
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
		i.IgnoreIfSet = fastly.Compatibool(header.IgnoreIfSet)
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
	r := strings.NewReplacer("/", "%2f")
	for _, condition := range existingConditions {
		err := client.DeleteCondition(&fastly.DeleteConditionInput{Service: s.ID, Name: r.Replace(condition.Name), Version: newversion.Number})
		if err != nil {
			return err
		}
	}
	for _, condition := range newConditions {
		var i fastly.CreateConditionInput
		i.Name = r.Replace(condition.Name)
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
		i.Port = backend.Port
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

	// Conditions, health checks, and cache settings must be sync'd first, as if they're
	// referenced in any other object the API will balk if they don't exist.
	remoteConditions, err := client.ListConditions(&fastly.ListConditionsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Conditions, remoteConditions) {
		if err := syncConditions(client, s, config.Conditions); err != nil {
			return fmt.Errorf("Error syncing conditions: %s", err)
		}
	}

	remoteHealthChecks, _ := client.ListHealthChecks(&fastly.ListHealthChecksInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.HealthChecks, remoteHealthChecks) {
		if err := syncHealthChecks(client, s, config.HealthChecks); err != nil {
			return fmt.Errorf("Error syncing health checks: %s", err)
		}
	}

	remoteCacheSettings, _ := client.ListCacheSettings(&fastly.ListCacheSettingsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.CacheSettings, remoteCacheSettings) {
		if err := syncCacheSettings(client, s, config.CacheSettings); err != nil {
			return fmt.Errorf("Error syncing cache settings: %s", err)
		}
	}

	remoteBackends, err := client.ListBackends(&fastly.ListBackendsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Backends, remoteBackends) {
		if err := syncBackends(client, s, config.Backends); err != nil {
			return fmt.Errorf("Error syncing backends: %s", err)
		}
	}

	remoteHeaders, _ := client.ListHeaders(&fastly.ListHeadersInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Headers, remoteHeaders) {
		if err := syncHeaders(client, s, config.Headers); err != nil {
			return fmt.Errorf("Error syncing headers: %s", err)
		}
	}

	remoteSyslogs, _ := client.ListSyslogs(&fastly.ListSyslogsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Syslogs, remoteSyslogs) {
		if err := syncSyslogs(client, s, config.Syslogs); err != nil {
			return fmt.Errorf("Error syncing syslogs: %s", err)
		}
	}

	remotePapertrails, _ := client.ListPapertrails(&fastly.ListPapertrailsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Papertrails, remotePapertrails) {
		if err := syncPapertrails(client, s, config.Papertrails); err != nil {
			return fmt.Errorf("Error syncing papertrail: %s", err)
		}
	}

	remoteSumologics, _ := client.ListSumologics(&fastly.ListSumologicsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Sumologics, remoteSumologics) {
		if err := syncSumologics(client, s, config.Sumologics); err != nil {
			return fmt.Errorf("Error syncing sumologics: %s", err)
		}
	}

	remoteFTPs, _ := client.ListFTPs(&fastly.ListFTPsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.FTPs, remoteFTPs) {
		if err := syncFTPs(client, s, config.FTPs); err != nil {
			return fmt.Errorf("Error syncing ftps: %s", err)
		}
	}

	remoteGCSs, _ := client.ListGCSs(&fastly.ListGCSsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.GCSs, remoteGCSs) {
		if err := syncGCSs(client, s, config.GCSs); err != nil {
			return err
		}
	}

	remoteS3s, _ := client.ListS3s(&fastly.ListS3sInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.S3s, remoteS3s) {
		if err := syncS3s(client, s, config.S3s); err != nil {
			return fmt.Errorf("Error syncing s3s: %s", err)
		}
	}

	remoteDomains, _ := client.ListDomains(&fastly.ListDomainsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Domains, remoteDomains) {
		if err := syncDomains(client, s, config.Domains); err != nil {
			return fmt.Errorf("Error syncing domains: %s", err)
		}
	}

	remoteSettings, _ := client.GetSettings(&fastly.GetSettingsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Settings, remoteSettings) {
		if err := syncSettings(client, s, config.Settings); err != nil {
			return fmt.Errorf("Error syncing settings: %s", err)
		}
	}

	remoteGzips, _ := client.ListGzips(&fastly.ListGzipsInput{Service: s.ID, Version: activeVersion})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(config.Gzips, remoteGzips) {
		if err := syncGzips(client, s, config.Gzips); err != nil {
			return fmt.Errorf("Error syncing gzips: %s", err)
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
	if !reflect.DeepEqual(config.Directors, remoteDirectors) || !mappingsInSync {
		if err := syncDirectors(client, s, config.Directors); err != nil {
			return fmt.Errorf("Error syncing directors: %s", err)
		}
		// Syncing directors will initially delete all directors, which implicitly
		// deletes all of the directorbackend mappings. As such, we must recreate.
		if err := syncDirectorBackends(client, s, config.DirectorBackends); err != nil {
			return fmt.Errorf("Error syncing director backend mappings: %s", err)
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
