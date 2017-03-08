package version

import "fmt"

// These will be set at linking time.
var (
	Version = "0.0"
	// Should be in YYYYMMDD format.
	VersionDate string
	// Git commit shortID
	VersionCommit string
)

func FullVersion() string {
	if VersionDate == "" || VersionCommit == "" {
		return Version
	}
	return fmt.Sprintf("%s~git%s.%s", Version, VersionDate, VersionCommit)
}
