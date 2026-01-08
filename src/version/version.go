package version

var (
	// Version is the semantic version of the application.
	// This should be set via linker flags during build:
	// -ldflags "-X tether/src/version.Version=1.0.0"
	Version = "dev"
)
