package config

type Paths struct {
	RWNodeDir         string
	XrayBin           string
	XrayConfigPath    string
	XrayAssetDir      string
	InternalSocket    string
	InternalRESTToken string
}

func (c Config) Paths() Paths {
	return Paths{
		RWNodeDir:         c.RWNodeDir,
		XrayBin:           c.XrayBin,
		XrayConfigPath:    c.XrayConfigPath,
		XrayAssetDir:      c.XrayAssetDir,
		InternalSocket:    c.InternalSocketPath,
		InternalRESTToken: c.InternalRESTToken,
	}
}
