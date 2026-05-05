package xray

import "testing"

func TestConfigBuilderInjectsRemnawaveAPI(t *testing.T) {
	builder := ConfigBuilder{XTLSAPIPort: 61000}
	config, err := builder.Build(map[string]any{
		"inbounds": []any{
			map[string]any{"tag": "VLESS_INBOUND", "protocol": "vless"},
		},
		"outbounds": []any{
			map[string]any{"tag": "DIRECT", "protocol": "freedom"},
		},
		"routing": map[string]any{
			"rules": []any{map[string]any{"outboundTag": "DIRECT"}},
		},
		"policy": map[string]any{
			"levels": map[string]any{
				"1": map[string]any{"handshake": float64(4)},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	api := config["api"].(map[string]any)
	if api["tag"] != APITag {
		t.Fatalf("api tag = %v", api["tag"])
	}

	inbounds := config["inbounds"].([]any)
	if len(inbounds) != 2 {
		t.Fatalf("inbounds len = %d, want 2", len(inbounds))
	}
	apiInbound := inbounds[1].(map[string]any)
	if apiInbound["listen"] != "127.0.0.1" || apiInbound["port"] != 61000 {
		t.Fatalf("api inbound = %#v", apiInbound)
	}

	rules := config["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("routing rules len = %d, want 2", len(rules))
	}

	level0 := config["policy"].(map[string]any)["levels"].(map[string]any)["0"].(map[string]any)
	if level0["statsUserOnline"] != true {
		t.Fatalf("level0 statsUserOnline = %v", level0["statsUserOnline"])
	}
}

func TestConfigBuilderRejectsTagConflict(t *testing.T) {
	_, err := ConfigBuilder{XTLSAPIPort: 61000}.Build(map[string]any{
		"inbounds": []any{map[string]any{"tag": APIInboundTag}},
	})
	if err == nil {
		t.Fatalf("Build() error = nil, want conflict")
	}
}
