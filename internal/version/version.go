package version

var (
	// ProjectVersion is rw-node-go's own release version. It starts at 1.0.0 and
	// is overridden at build time from the repository VERSION file.
	ProjectVersion = "1.0.0"
	// NodeVersion is the Panel-facing compatibility version. Keep this aligned
	// with official remnawave/node 2.7.x so Panel accepts the contract shape.
	NodeVersion = "2.7.0"
	Commit      = "unknown"
	BuildDate   = "unknown"
)
