package xray

import "testing"

func TestConfigBuilderInjectsRemnawaveAPI(t *testing.T) {
	mtls, err := NewInternalMTLSBundle()
	if err != nil {
		t.Fatalf("NewInternalMTLSBundle() error = %v", err)
	}
	builder := ConfigBuilder{XTLSAPIPort: 61000, InternalMTLS: mtls}
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
	apiInbound := inbounds[0].(map[string]any)
	if apiInbound["listen"] != "127.0.0.1" || apiInbound["port"] != 61000 {
		t.Fatalf("api inbound = %#v", apiInbound)
	}
	streamSettings := apiInbound["streamSettings"].(map[string]any)
	if streamSettings["security"] != "tls" {
		t.Fatalf("security = %v, want tls", streamSettings["security"])
	}
	tlsSettings := streamSettings["tlsSettings"].(map[string]any)
	if tlsSettings["serverName"] != InternalServerName ||
		tlsSettings["disableSystemRoot"] != true ||
		tlsSettings["rejectUnknownSni"] != true {
		t.Fatalf("tlsSettings = %#v", tlsSettings)
	}
	certificates := tlsSettings["certificates"].([]any)
	if len(certificates) != 2 {
		t.Fatalf("certificates len = %d, want 2", len(certificates))
	}
	serverCert := certificates[0].(map[string]any)
	if len(serverCert["certificate"].([]any)) == 0 || len(serverCert["key"].([]any)) == 0 {
		t.Fatalf("server certificate not injected: %#v", serverCert)
	}
	verifyCert := certificates[1].(map[string]any)
	if verifyCert["usage"] != "verify" || len(verifyCert["certificate"].([]any)) == 0 {
		t.Fatalf("verify certificate not injected: %#v", verifyCert)
	}

	rules := config["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("routing rules len = %d, want 2", len(rules))
	}
	firstRule := rules[0].(map[string]any)
	if firstRule["outboundTag"] != APITag {
		t.Fatalf("first routing rule = %#v", firstRule)
	}

	level0 := config["policy"].(map[string]any)["levels"].(map[string]any)["0"].(map[string]any)
	if level0["statsUserOnline"] != false {
		t.Fatalf("level0 statsUserOnline = %v, want false without CAP_NET_ADMIN", level0["statsUserOnline"])
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
