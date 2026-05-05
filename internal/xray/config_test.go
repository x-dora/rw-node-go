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

func TestConfigBuilderInjectsTorrentBlockerWhenEnabled(t *testing.T) {
	mtls, err := NewInternalMTLSBundle()
	if err != nil {
		t.Fatalf("NewInternalMTLSBundle() error = %v", err)
	}
	builder := ConfigBuilder{
		XTLSAPIPort:        61000,
		InternalMTLS:       mtls,
		InternalSocketPath: "/tmp/remnawave-node.sock",
		InternalRESTToken:  "internal-token",
		TorrentBlocker: TorrentBlockerInjection{
			Enabled:         true,
			IncludeRuleTags: []string{"DIRECT_RULE"},
		},
	}
	config, err := builder.Build(map[string]any{
		"outbounds": []any{map[string]any{"tag": "DIRECT", "protocol": "freedom"}},
		"routing": map[string]any{
			"rules": []any{
				map[string]any{"ruleTag": "DIRECT_RULE", "outboundTag": "DIRECT"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	outbounds := config["outbounds"].([]any)
	lastOutbound := outbounds[len(outbounds)-1].(map[string]any)
	if lastOutbound["tag"] != TorrentBlockerOutboundTag || lastOutbound["protocol"] != "blackhole" {
		t.Fatalf("torrent outbound = %#v", lastOutbound)
	}

	rules := config["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("rules len = %d, want 3", len(rules))
	}
	torrentRule := rules[1].(map[string]any)
	if torrentRule["outboundTag"] != TorrentBlockerOutboundTag {
		t.Fatalf("torrent rule = %#v", torrentRule)
	}
	protocols := torrentRule["protocol"].([]any)
	if len(protocols) != 1 || protocols[0] != "bittorrent" {
		t.Fatalf("torrent rule protocol = %#v", protocols)
	}
	webhook := torrentRule["webhook"].(map[string]any)
	wantWebhookURL := "/tmp/remnawave-node.sock:/internal/webhook?token=internal-token"
	if webhook["url"] != wantWebhookURL || webhook["deduplication"] != 5 {
		t.Fatalf("torrent webhook = %#v", webhook)
	}

	includeRule := rules[2].(map[string]any)
	includeWebhook := includeRule["webhook"].(map[string]any)
	if includeWebhook["url"] != wantWebhookURL || includeWebhook["deduplication"] != 5 {
		t.Fatalf("include rule webhook = %#v", includeWebhook)
	}
}

func TestConfigBuilderBuildsOfficialInternalWebhookURL(t *testing.T) {
	builder := ConfigBuilder{
		InternalSocketPath: "/tmp/remnawave-node.sock",
		InternalRESTToken:  "token-1",
	}
	got := builder.InternalWebhookURL()
	want := "/tmp/remnawave-node.sock:/internal/webhook?token=token-1"
	if got != want {
		t.Fatalf("InternalWebhookURL() = %q, want %q", got, want)
	}
}

func TestConfigBuilderDoesNotInjectTorrentBlockerWhenDisabled(t *testing.T) {
	config, err := ConfigBuilder{XTLSAPIPort: 61000}.Build(map[string]any{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	for _, outbound := range ensureArray(config["outbounds"]) {
		if outbound.(map[string]any)["tag"] == TorrentBlockerOutboundTag {
			t.Fatalf("unexpected torrent blocker outbound = %#v", outbound)
		}
	}
	rules := config["routing"].(map[string]any)["rules"].([]any)
	for _, rule := range rules {
		if ruleMap, ok := rule.(map[string]any); ok && ruleMap["outboundTag"] == TorrentBlockerOutboundTag {
			t.Fatalf("unexpected torrent blocker rule = %#v", ruleMap)
		}
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
