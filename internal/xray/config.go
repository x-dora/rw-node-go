package xray

import "fmt"

const (
	APITag        = "REMNAWAVE_API"
	APIInboundTag = "REMNAWAVE_API_INBOUND"
)

type ConfigBuilder struct {
	XTLSAPIPort int
}

func (b ConfigBuilder) Build(panelConfig map[string]any) (map[string]any, error) {
	config := cloneMap(panelConfig)
	if hasTag(config, APITag) {
		return nil, fmt.Errorf("xray config tag %q conflicts with Remnawave API tag", APITag)
	}
	if hasTag(config, APIInboundTag) {
		return nil, fmt.Errorf("xray config tag %q conflicts with Remnawave API inbound tag", APIInboundTag)
	}

	config["stats"] = ensureMap(config["stats"])
	config["api"] = map[string]any{
		"services": []any{"HandlerService", "StatsService", "RoutingService"},
		"tag":      APITag,
	}

	inbounds := ensureArray(config["inbounds"])
	inbounds = append(inbounds, b.apiInbound())
	config["inbounds"] = inbounds

	routing := ensureMap(config["routing"])
	rules := ensureArray(routing["rules"])
	rules = append(rules, map[string]any{
		"type":        "field",
		"inboundTag":  []any{APIInboundTag},
		"outboundTag": APITag,
	})
	routing["rules"] = rules
	config["routing"] = routing

	policy := ensureMap(config["policy"])
	levels := ensureMap(policy["levels"])
	level0 := ensureMap(levels["0"])
	level0["statsUserUplink"] = true
	level0["statsUserDownlink"] = true
	level0["statsUserOnline"] = true
	levels["0"] = level0
	policy["levels"] = levels

	system := ensureMap(policy["system"])
	system["statsInboundDownlink"] = true
	system["statsInboundUplink"] = true
	system["statsOutboundDownlink"] = true
	system["statsOutboundUplink"] = true
	policy["system"] = system
	config["policy"] = policy

	return config, nil
}

func (b ConfigBuilder) apiInbound() map[string]any {
	return map[string]any{
		"tag":      APIInboundTag,
		"port":     b.XTLSAPIPort,
		"listen":   "127.0.0.1",
		"protocol": "dokodemo-door",
		"settings": map[string]any{
			"address": "127.0.0.1",
		},
		"streamSettings": map[string]any{
			"security": "tls",
			"tlsSettings": map[string]any{
				"alpn":              []any{"h2"},
				"serverName":        "internal.remnawave.local",
				"disableSystemRoot": true,
				"rejectUnknownSni":  true,
				"certificates":      []any{},
			},
		},
	}
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = cloneValue(value)
	}
	return output
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		output := make([]any, len(typed))
		for i, item := range typed {
			output[i] = cloneValue(item)
		}
		return output
	default:
		return value
	}
}

func ensureMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return cloneMap(typed)
	}
	return map[string]any{}
}

func ensureArray(value any) []any {
	if typed, ok := value.([]any); ok {
		output := make([]any, len(typed))
		copy(output, typed)
		return output
	}
	return []any{}
}

func hasTag(config map[string]any, tag string) bool {
	for _, section := range []string{"inbounds", "outbounds"} {
		items := ensureArray(config[section])
		for _, item := range items {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if itemMap["tag"] == tag {
				return true
			}
		}
	}
	return false
}
