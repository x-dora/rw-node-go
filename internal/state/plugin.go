package state

type PluginState struct {
	TorrentBlockerEnabled bool
	NftablesEnabled       bool
	ConfigByName          map[string]map[string]any
}
