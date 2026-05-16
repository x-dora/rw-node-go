package version

var (
	// ProjectVersion is rw-node-go's own release version. It starts at dev and
	// is overridden at build time from the repository VERSION file.
	ProjectVersion = "dev"
	// NodeVersion is the Panel-facing compatibility version. Keep this aligned
	// with official remnawave/node 2.8.x so Panel accepts the contract shape.
	NodeVersion = "2.8.0"
	Commit      = "unknown"
	BuildDate   = "unknown"
)
