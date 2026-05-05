package config

type Paths struct {
	RWNodeDir      string
	XrayBin        string
	XrayConfigPath string
}

func (c Config) Paths() Paths {
	return Paths{
		RWNodeDir:      c.RWNodeDir,
		XrayBin:        c.XrayBin,
		XrayConfigPath: c.XrayConfigPath,
	}
}
