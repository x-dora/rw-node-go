package contracts_test

import (
	"encoding/json"
	"testing"

	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/testkit"
)

func TestStartInternalsAcceptsMetadataAndIntegrations(t *testing.T) {
	request := testkit.MustStrictDecode[contracts.StartXrayRequest](t, json.RawMessage(`{
		"internals": {
			"metadata": {
				"name": "node-1",
				"uuid": "55555555-5555-4555-8555-555555555555",
				"id": 7,
				"tags": ["a", "b"],
				"countryCode": "DE"
			},
			"integrations": {"torrent-blocker": {"enabled": true}},
			"forceRestart": true,
			"hashes": {
				"emptyConfig": "hash",
				"inbounds": [{"usersCount": 1, "hash": "h", "tag": "VLESS_INBOUND"}]
			}
		},
		"xrayConfig": {"inbounds": []}
	}`))

	if request.Internals.Metadata == nil ||
		request.Internals.Metadata.Name != "node-1" ||
		request.Internals.Metadata.UUID != "55555555-5555-4555-8555-555555555555" ||
		request.Internals.Metadata.ID != 7 ||
		len(request.Internals.Metadata.Tags) != 2 ||
		request.Internals.Metadata.CountryCode != "DE" {
		t.Fatalf("metadata = %#v", request.Internals.Metadata)
	}
	if len(request.Internals.Integrations) != 1 {
		t.Fatalf("integrations = %#v", request.Internals.Integrations)
	}
	if raw, ok := request.Internals.Integrations["torrent-blocker"]; !ok || string(raw) != `{"enabled": true}` {
		t.Fatalf("integrations[torrent-blocker] = %s", raw)
	}
}

func TestStartInternalsMetadataAndIntegrationsAreOptional(t *testing.T) {
	request := testkit.MustStrictDecode[contracts.StartXrayRequest](t, json.RawMessage(`{
		"internals": {
			"forceRestart": false,
			"hashes": {
				"emptyConfig": "hash",
				"inbounds": []
			}
		},
		"xrayConfig": {}
	}`))

	if request.Internals.Metadata != nil {
		t.Fatalf("metadata = %#v, want nil", request.Internals.Metadata)
	}
	if request.Internals.Integrations != nil {
		t.Fatalf("integrations = %#v, want nil", request.Internals.Integrations)
	}
}
