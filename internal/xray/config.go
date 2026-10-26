package xray

import (
	"fmt"
)

const (
	APITag                    = "REMNAWAVE_API"
	APIInboundTag             = "REMNAWAVE_API_INBOUND"
	TorrentBlockerOutboundTag = "RW_TB_OUTBOUND_BLOCK"
)

type ConfigBuilder struct {
	StatsUserOnline bool
	// HysteriaListenPort > 0 时用它覆盖面板下发的 hysteria inbound 端口。
	// 只影响 hysteria：QUIC 走 UDP，能和监听同名 TCP 端口的前置分流器共存，
	// 这是单端口 PaaS 上跑 hy2 的唯一办法。
	HysteriaListenPort int
}

func (b ConfigBuilder) Build(panelConfig map[string]any) (map[string]any, error) {
	config := cloneMap(panelConfig)
	if hasTag(config, APITag) {
		return nil, fmt.Errorf("xray config tag %q conflicts with Remnawave API tag", APITag)
	}
	if hasTag(config, APIInboundTag) {
		return nil, fmt.Errorf("xray config tag %q conflicts with Remnawave API inbound tag", APIInboundTag)
	}

	applyHysteriaListenPort(config, b.HysteriaListenPort)

	config["stats"] = ensureMap(config["stats"])
	policy := ensureMap(config["policy"])
	levels := ensureMap(policy["levels"])
	level0 := ensureMap(levels["0"])
	level0["statsUserUplink"] = true
	level0["statsUserDownlink"] = true
	level0["statsUserOnline"] = b.StatsUserOnline
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

// applyHysteriaListenPort 就地改写所有 hysteria inbound 的 port。
// port 为 0 时不动配置，保持面板下发的行为。
func applyHysteriaListenPort(config map[string]any, port int) {
	if port <= 0 {
		return
	}
	for _, item := range ensureArray(config["inbounds"]) {
		inbound, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if inbound["protocol"] == "hysteria" {
			inbound["port"] = port
		}
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
