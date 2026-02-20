package version

// Version is set via ldflags during build (-ldflags="-X github.com/heldtogether/switchyard/internal/version.Version=x.y.z")
// Defaults to "dev" for development builds
var Version = "dev"
