package contracts_test

import (
	"encoding/json"
	"testing"

	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/testkit"
)

func TestOfficialRequestFixturesDecode(t *testing.T) {
	fixture := testkit.LoadPanelAPIFixture(t)
	for _, route := range fixture.Routes {
		if len(route.Request) == 0 {
			continue
		}
		t.Run(route.Name, func(t *testing.T) {
			decodeOfficialRequest(t, route.Name, route.Request)
		})
	}
}

func TestOfficialResponseFixturesEncode(t *testing.T) {
	fixture := testkit.LoadPanelAPIFixture(t)
	for _, route := range fixture.Routes {
		t.Run(route.Name, func(t *testing.T) {
			got := officialResponseValue(t, route.Name)
			testkit.AssertCanonicalJSONEqual(t, got, route.Response)
		})
	}
}

func decodeOfficialRequest(t *testing.T, name string, data json.RawMessage) {
	t.Helper()

	switch name {
	case "xray.start":
		testkit.MustStrictDecode[contracts.StartXrayRequest](t, data)
	case "handler.add-user":
		testkit.MustStrictDecode[contracts.AddUserRequest](t, data)
	case "handler.add-users":
		testkit.MustStrictDecode[contracts.AddUsersRequest](t, data)
	case "handler.remove-user":
		testkit.MustStrictDecode[contracts.RemoveUserRequest](t, data)
	case "handler.remove-users":
		testkit.MustStrictDecode[contracts.RemoveUsersRequest](t, data)
	case "handler.get-inbound-users", "handler.get-inbound-users-count":
		testkit.MustStrictDecode[contracts.InboundTagRequest](t, data)
	case "handler.drop-users-connections":
		testkit.MustStrictDecode[contracts.DropUsersConnectionsRequest](t, data)
	case "handler.drop-ips":
		testkit.MustStrictDecode[contracts.DropIPsRequest](t, data)
	case "stats.get-users-stats", "stats.get-all-inbounds-stats", "stats.get-all-outbounds-stats", "stats.get-combined-stats":
		testkit.MustStrictDecode[contracts.ResetRequest](t, data)
	case "stats.get-user-online-status":
		testkit.MustStrictDecode[contracts.UserOnlineStatusRequest](t, data)
	case "stats.get-user-ip-list":
		testkit.MustStrictDecode[contracts.UserIPListRequest](t, data)
	case "stats.get-inbound-stats", "stats.get-outbound-stats":
		testkit.MustStrictDecode[contracts.TaggedStatsRequest](t, data)
	case "vision.block-ip", "vision.unblock-ip":
		testkit.MustStrictDecode[contracts.VisionIPRequest](t, data)
	case "plugin.sync":
		testkit.MustStrictDecode[contracts.PluginSyncRequest](t, data)
	case "plugin.nftables.block-ips":
		testkit.MustStrictDecode[contracts.BlockIPsRequest](t, data)
	case "plugin.nftables.unblock-ips":
		testkit.MustStrictDecode[contracts.UnblockIPsRequest](t, data)
	default:
		t.Fatalf("no request decode mapping for %s", name)
	}
}

func officialResponseValue(t *testing.T, name string) any {
	t.Helper()

	switch name {
	case "xray.start":
		version := "25.1.1"
		nodeVersion := "2.7.0"
		return httpapi.Envelope{Response: contracts.StartXrayResponse{
			IsStarted:       true,
			Version:         &version,
			Error:           nil,
			NodeInformation: contracts.NodeInformation{Version: &nodeVersion},
			System:          fixtureSystemStats(),
		}}
	case "xray.stop":
		return httpapi.Envelope{Response: contracts.StopXrayResponse{IsStopped: true}}
	case "xray.healthcheck":
		return httpapi.Envelope{Response: contracts.HealthcheckResponse{
			IsAlive:                  true,
			XrayInternalStatusCached: false,
			XrayVersion:              nil,
			NodeVersion:              "2.7.0",
		}}
	case "handler.add-user", "handler.add-users", "handler.remove-user", "handler.remove-users", "vision.block-ip", "vision.unblock-ip":
		return httpapi.Envelope{Response: contracts.SuccessResponse()}
	case "handler.get-inbound-users":
		return httpapi.Envelope{Response: contracts.InboundUsersResponse{Users: []contracts.InboundUser{}}}
	case "handler.get-inbound-users-count":
		return httpapi.Envelope{Response: contracts.InboundUsersCountResponse{Count: 0}}
	case "handler.drop-users-connections", "handler.drop-ips":
		return httpapi.Envelope{Response: contracts.SimpleSuccess()}
	case "stats.get-system-stats":
		return httpapi.Envelope{Response: contracts.SystemStatsResponse{
			XrayInfo: &contracts.XraySysStats{
				NumGoroutine: 1,
				NumGC:        2,
				Alloc:        3,
				TotalAlloc:   4,
				Sys:          5,
				Mallocs:      6,
				Frees:        7,
				LiveObjects:  8,
				PauseTotalNs: 9,
				Uptime:       10,
			},
			Plugins: contracts.PluginStats{
				TorrentBlocker: contracts.TorrentBlockerPluginStats{ReportsCount: 0},
			},
			System: contracts.SystemStats{Stats: fixtureSystemStats().Stats},
		}}
	case "stats.get-users-stats":
		return httpapi.Envelope{Response: contracts.UsersStatsResponse{Users: []contracts.UserTrafficStats{}}}
	case "stats.get-user-online-status":
		return httpapi.Envelope{Response: contracts.UserOnlineStatusResponse{IsOnline: false}}
	case "stats.get-user-ip-list":
		return httpapi.Envelope{Response: contracts.UserIPListResponse{IPs: []contracts.IPLastSeen{}}}
	case "stats.get-users-ip-list":
		return httpapi.Envelope{Response: contracts.UsersIPListResponse{Users: []contracts.UserIPList{}}}
	case "stats.get-inbound-stats":
		return httpapi.Envelope{Response: contracts.InboundTrafficStatsResponse{Inbound: "VLESS_INBOUND"}}
	case "stats.get-outbound-stats":
		return httpapi.Envelope{Response: contracts.OutboundTrafficStatsResponse{Outbound: "DIRECT"}}
	case "stats.get-all-inbounds-stats":
		return httpapi.Envelope{Response: contracts.AllInboundsStatsResponse{Inbounds: []contracts.InboundTrafficStatsResponse{}}}
	case "stats.get-all-outbounds-stats":
		return httpapi.Envelope{Response: contracts.AllOutboundsStatsResponse{Outbounds: []contracts.OutboundTrafficStatsResponse{}}}
	case "stats.get-combined-stats":
		return httpapi.Envelope{Response: contracts.CombinedStatsResponse{
			Inbounds:  []contracts.InboundTrafficStatsResponse{},
			Outbounds: []contracts.OutboundTrafficStatsResponse{},
		}}
	case "plugin.sync", "plugin.nftables.block-ips", "plugin.nftables.unblock-ips", "plugin.nftables.recreate-tables":
		return httpapi.Envelope{Response: contracts.AcceptedResponse{Accepted: true}}
	case "plugin.torrent-blocker.collect":
		return httpapi.Envelope{Response: contracts.TorrentBlockerReportsResponse{Reports: []contracts.TorrentBlockerReport{}}}
	default:
		t.Fatalf("no response encode mapping for %s", name)
		return nil
	}
}

func fixtureSystemStats() contracts.SystemStatsPayload {
	return contracts.SystemStatsPayload{
		Info: contracts.NodeSystemInfo{
			Arch:              "amd64",
			CPUs:              2,
			CPUModel:          "fixture-cpu",
			MemoryTotal:       1024,
			Hostname:          "fixture-node",
			Platform:          "linux",
			Release:           "6.0.0",
			Type:              "Linux",
			Version:           "fixture-version",
			NetworkInterfaces: []string{"eth0"},
		},
		Stats: contracts.NodeSystemStats{
			MemoryFree: 512,
			MemoryUsed: 512,
			Uptime:     60,
			LoadAvg:    []float64{0, 0, 0},
			Interface:  nil,
		},
	}
}
