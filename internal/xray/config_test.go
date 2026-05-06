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
	if len(outbounds) != 2 || outbounds[0].(map[string]any)["tag"] != "DIRECT" || outbounds[1].(map[string]any)["tag"] != BlockOutboundTag {
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

func TestConfigBuilderPreservesExistingBlockOutbound(t *testing.T) {
	config, err := ConfigBuilder{}.Build(map[string]any{
		"outbounds": []any{
			map[string]any{
				"tag":      BlockOutboundTag,
				"protocol": "blackhole",
				"settings": map[string]any{
					"response": map[string]any{"type": "http"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	outbounds := config["outbounds"].([]any)
	if len(outbounds) != 1 {
		t.Fatalf("outbounds = %#v", outbounds)
	}
	settings := outbounds[0].(map[string]any)["settings"].(map[string]any)
	if settings["response"] == nil {
		t.Fatalf("existing BLOCK outbound was overwritten: %#v", outbounds[0])
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
	config, err := ConfigBuilder{TorrentBlocker: TorrentBlockerInjection{Enabled: true}}.Build(map[string]any{
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
