package xray

type ConfigBuilder struct {
	XTLSAPIPort int
}

func (b ConfigBuilder) Build(panelConfig map[string]any) (map[string]any, error) {
	// TODO(M1): inject Remnawave API inbound, stats, policy, and routing.
	return panelConfig, nil
}
