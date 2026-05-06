package xray

import "testing"

func TestParseTrafficStatName(t *testing.T) {
	tag, direction, ok := parseTrafficStatName("user>>>alpha>>>traffic>>>uplink", "user")
	if !ok || tag != "alpha" || direction != "uplink" {
		t.Fatalf("parseTrafficStatName() = %q %q %v", tag, direction, ok)
	}
	if _, _, ok := parseTrafficStatName("user>>>alpha>>>online", "user"); ok {
		t.Fatalf("parseTrafficStatName() accepted online stat")
	}
}

func TestApplyDirection(t *testing.T) {
	var uplink, downlink int64
	applyDirection("uplink", 10, &uplink, &downlink)
	applyDirection("downlink", 20, &uplink, &downlink)
	if uplink != 10 || downlink != 20 {
		t.Fatalf("uplink=%d downlink=%d", uplink, downlink)
	}
}
