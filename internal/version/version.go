package version

var (
	// ProjectVersion is rw-node-go's own release version. It starts at dev and
	// is overridden at build time from the repository VERSION file.
	ProjectVersion = "dev"
	// NodeVersion is the Panel-facing compatibility version. Keep this aligned
	// with official remnawave/node 3.0.x so Panel accepts the contract shape.
	NodeVersion = "3.0.0"
	Commit      = "unknown"
	BuildDate   = "unknown"
)
