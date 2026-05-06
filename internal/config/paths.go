package config

type Paths struct {
	RWNodeDir string
}

func (c Config) Paths() Paths {
	return Paths{
		RWNodeDir: c.RWNodeDir,
	}
}
