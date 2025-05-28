package daemon

// Version info (set by ldflags during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// GetVersionInfo returns version information as a struct
type VersionInfo struct {
	Version   string
	GitCommit string
	BuildDate string
}

// GetVersionInfo returns the current version information
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}
}