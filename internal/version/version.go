package version

var (
	// Version is the Panel-facing nodeVersion. Keep the default aligned with
	// official remnawave/node 2.7.x so Panel does not reject local dev builds as
	// too old; release builds may still override it with ldflags.
	Version   = "2.7.0"
	Commit    = "unknown"
	BuildDate = "unknown"
)
