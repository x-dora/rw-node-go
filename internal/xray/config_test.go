package xray

import "testing"

func TestConfigBuilderInjectsOnlyEmbeddedStatsPolicy(t *testing.T) {
	input := map[string]any{
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
	}
	config, err := ConfigBuilder{}.Build(input)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if _, ok := config["api"]; ok {
		t.Fatalf("api was injected: %#v", config["api"])
	}
	inbounds := config["inbounds"].([]any)
	if len(inbounds) != 1 || inbounds[0].(map[string]any)["tag"] != "VLESS_INBOUND" {
		t.Fatalf("inbounds = %#v", inbounds)
	}
	outbounds := config["outbounds"].([]any)
	if len(outbounds) != 1 || outbounds[0].(map[string]any)["tag"] != "DIRECT" {
		t.Fatalf("outbounds = %#v", outbounds)
	}
	rules := config["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 1 || rules[0].(map[string]any)["outboundTag"] != "DIRECT" {
		t.Fatalf("routing rules = %#v", rules)
	}

	if _, ok := config["stats"].(map[string]any); !ok {
		t.Fatalf("stats was not ensured: %#v", config["stats"])
	}
	policy := config["policy"].(map[string]any)
	levels := policy["levels"].(map[string]any)
	level0 := levels["0"].(map[string]any)
	if level0["statsUserUplink"] != true ||
		level0["statsUserDownlink"] != true ||
		level0["statsUserOnline"] != false {
		t.Fatalf("level0 = %#v", level0)
	}
	if _, ok := levels["1"]; !ok {
		t.Fatalf("existing policy level lost: %#v", levels)
	}
	system := policy["system"].(map[string]any)
	for _, key := range []string{
		"statsInboundDownlink",
		"statsInboundUplink",
		"statsOutboundDownlink",
		"statsOutboundUplink",
	} {
		if system[key] != true {
			t.Fatalf("policy.system[%s] = %#v", key, system[key])
		}
	}

	if _, ok := input["stats"]; ok {
		t.Fatalf("input map was mutated: %#v", input)
	}
}

func TestConfigBuilderCanEnableStatsUserOnline(t *testing.T) {
	config, err := ConfigBuilder{StatsUserOnline: true}.Build(nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	policy := config["policy"].(map[string]any)
	level0 := policy["levels"].(map[string]any)["0"].(map[string]any)
	if level0["statsUserOnline"] != true {
		t.Fatalf("level0 = %#v", level0)
	}
}

func TestConfigBuilderDoesNotInjectPluginRuntime(t *testing.T) {
	config, err := ConfigBuilder{}.Build(map[string]any{
		"outbounds": []any{map[string]any{"tag": "DIRECT", "protocol": "freedom"}},
		"routing": map[string]any{
			"rules": []any{map[string]any{"ruleTag": "DIRECT_RULE", "outboundTag": "DIRECT"}},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	for _, outbound := range ensureArray(config["outbounds"]) {
		if outbound.(map[string]any)["tag"] == TorrentBlockerOutboundTag {
			t.Fatalf("unexpected torrent blocker outbound = %#v", outbound)
		}
	}
	for _, rule := range ensureArray(config["routing"].(map[string]any)["rules"]) {
		ruleMap, ok := rule.(map[string]any)
		if ok && (ruleMap["outboundTag"] == TorrentBlockerOutboundTag || ruleMap["webhook"] != nil) {
			t.Fatalf("unexpected plugin routing rule = %#v", ruleMap)
		}
	}
}

func TestConfigBuilderRejectsTagConflict(t *testing.T) {
	_, err := ConfigBuilder{}.Build(map[string]any{
		"inbounds": []any{map[string]any{"tag": APIInboundTag}},
	})
	if err == nil {
		t.Fatalf("Build() error = nil, want conflict")
	}
}

func TestConfigBuilderOverridesHysteriaListenPort(t *testing.T) {
	input := map[string]any{
		"inbounds": []any{
			map[string]any{"tag": "VLESS_TCP_REALITY", "protocol": "vless", "port": float64(8443)},
			map[string]any{"tag": "Hysteria", "protocol": "hysteria", "port": float64(30000)},
		},
	}

	config, err := ConfigBuilder{HysteriaListenPort: 3066}.Build(input)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	ports := map[string]any{}
	for _, item := range ensureArray(config["inbounds"]) {
		inbound := item.(map[string]any)
		ports[inbound["tag"].(string)] = inbound["port"]
	}
	if got := ports["Hysteria"]; got != 3066 {
		t.Fatalf("hysteria port = %#v, want 3066", got)
	}
	if got := ports["VLESS_TCP_REALITY"]; got != float64(8443) {
		t.Fatalf("non-hysteria port was rewritten: %#v", got)
	}
	if got := input["inbounds"].([]any)[1].(map[string]any)["port"]; got != float64(30000) {
		t.Fatalf("input config was mutated: %#v", got)
	}
}

func TestConfigBuilderKeepsHysteriaPortWhenUnset(t *testing.T) {
	input := map[string]any{
		"inbounds": []any{
			map[string]any{"tag": "Hysteria", "protocol": "hysteria", "port": float64(30000)},
		},
	}

	for _, port := range []int{0, -1} {
		config, err := ConfigBuilder{HysteriaListenPort: port}.Build(input)
		if err != nil {
			t.Fatalf("Build(port=%d) error = %v", port, err)
		}
		got := ensureArray(config["inbounds"])[0].(map[string]any)["port"]
		if got != float64(30000) {
			t.Fatalf("port=%d: hysteria port = %#v, want unchanged 30000", port, got)
		}
	}
}

func TestConfigBuilderIgnoresInboundWithoutHysteria(t *testing.T) {
	input := map[string]any{
		"inbounds": []any{
			map[string]any{"tag": "VLESS_WS_RAW", "protocol": "vless", "port": float64(8880)},
			"not-a-map",
		},
	}

	config, err := ConfigBuilder{HysteriaListenPort: 3066}.Build(input)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	got := ensureArray(config["inbounds"])[0].(map[string]any)["port"]
	if got != float64(8880) {
		t.Fatalf("vless port = %#v, want unchanged 8880", got)
	}
}
