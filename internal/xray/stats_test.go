package xray

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	statscommand "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestStatsClientSysStats(t *testing.T) {
	server := &recordingStatsServer{
		sysStats: &statscommand.SysStatsResponse{
			NumGoroutine: 3,
			NumGC:        4,
			Alloc:        5,
			TotalAlloc:   6,
			Sys:          7,
			Mallocs:      8,
			Frees:        9,
			LiveObjects:  10,
			PauseTotalNs: 11,
			Uptime:       12,
		},
	}
	client, stop := startStatsServer(t, server)
	defer stop()

	got, err := (&statsClient{raw: client}).SysStats(context.Background())
	if err != nil {
		t.Fatalf("SysStats() error = %v", err)
	}
	if got.NumGoroutine != 3 || got.NumGC != 4 || got.TotalAlloc != 6 || got.PauseTotalNs != 11 {
		t.Fatalf("SysStats() = %#v", got)
	}
}

func TestStatsClientUsersStatsAggregatesAndFilters(t *testing.T) {
	server := &recordingStatsServer{
		queryStats: []*statscommand.Stat{
			{Name: "user>>>beta>>>traffic>>>downlink", Value: 20},
			{Name: "user>>>alpha>>>traffic>>>uplink", Value: 10},
			{Name: "user>>>alpha>>>traffic>>>downlink", Value: 30},
			{Name: "user>>>alpha>>>online", Value: 1},
			{Name: "user>>>zero>>>traffic>>>uplink", Value: 0},
			{Name: "inbound>>>ignored>>>traffic>>>uplink", Value: 99},
		},
	}
	client, stop := startStatsServer(t, server)
	defer stop()

	got, err := (&statsClient{raw: client}).UsersStats(context.Background(), true)
	if err != nil {
		t.Fatalf("UsersStats() error = %v", err)
	}
	want := []UserTrafficStats{
		{Username: "alpha", Uplink: 10, Downlink: 30},
		{Username: "beta", Uplink: 0, Downlink: 20},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UsersStats() = %#v, want %#v", got, want)
	}
	if len(server.queryRequests) != 1 || server.queryRequests[0].GetPattern() != "user>>>" || !server.queryRequests[0].GetReset_() {
		t.Fatalf("queryRequests = %#v", server.queryRequests)
	}
}

func TestStatsClientSingleTagStatsUseExpectedNamesAndReset(t *testing.T) {
	server := &recordingStatsServer{
		getStats: map[string]int64{
			"inbound>>>VLESS_INBOUND>>>traffic>>>uplink":   11,
			"inbound>>>VLESS_INBOUND>>>traffic>>>downlink": 22,
			"outbound>>>DIRECT>>>traffic>>>uplink":         33,
			"outbound>>>DIRECT>>>traffic>>>downlink":       44,
		},
	}
	client, stop := startStatsServer(t, server)
	defer stop()
	stats := &statsClient{raw: client}

	inbound, err := stats.InboundStats(context.Background(), "VLESS_INBOUND", true)
	if err != nil {
		t.Fatalf("InboundStats() error = %v", err)
	}
	outbound, err := stats.OutboundStats(context.Background(), "DIRECT", false)
	if err != nil {
		t.Fatalf("OutboundStats() error = %v", err)
	}

	if inbound != (InboundTrafficStats{Inbound: "VLESS_INBOUND", Uplink: 11, Downlink: 22}) {
		t.Fatalf("inbound = %#v", inbound)
	}
	if outbound != (OutboundTrafficStats{Outbound: "DIRECT", Uplink: 33, Downlink: 44}) {
		t.Fatalf("outbound = %#v", outbound)
	}
	if len(server.getRequests) != 4 {
		t.Fatalf("getRequests len = %d", len(server.getRequests))
	}
	if !server.getRequests[0].GetReset_() || server.getRequests[2].GetReset_() {
		t.Fatalf("getRequests reset flags = %#v", server.getRequests)
	}
}

func TestStatsClientAllTagStatsAggregateMissingSidesAndSort(t *testing.T) {
	server := &recordingStatsServer{
		queryStats: []*statscommand.Stat{
			{Name: "inbound>>>B>>>traffic>>>uplink", Value: 2},
			{Name: "inbound>>>A>>>traffic>>>downlink", Value: 1},
			{Name: "inbound>>>A>>>traffic>>>uplink", Value: 3},
			{Name: "outbound>>>Z>>>traffic>>>downlink", Value: 4},
			{Name: "outbound>>>Y>>>traffic>>>uplink", Value: 5},
		},
	}
	client, stop := startStatsServer(t, server)
	defer stop()
	stats := &statsClient{raw: client}

	inbounds, err := stats.AllInboundStats(context.Background(), false)
	if err != nil {
		t.Fatalf("AllInboundStats() error = %v", err)
	}
	outbounds, err := stats.AllOutboundStats(context.Background(), true)
	if err != nil {
		t.Fatalf("AllOutboundStats() error = %v", err)
	}

	wantInbounds := []InboundTrafficStats{
		{Inbound: "A", Uplink: 3, Downlink: 1},
		{Inbound: "B", Uplink: 2, Downlink: 0},
	}
	wantOutbounds := []OutboundTrafficStats{
		{Outbound: "Y", Uplink: 5, Downlink: 0},
		{Outbound: "Z", Uplink: 0, Downlink: 4},
	}
	if !reflect.DeepEqual(inbounds, wantInbounds) {
		t.Fatalf("inbounds = %#v, want %#v", inbounds, wantInbounds)
	}
	if !reflect.DeepEqual(outbounds, wantOutbounds) {
		t.Fatalf("outbounds = %#v, want %#v", outbounds, wantOutbounds)
	}
	if len(server.queryRequests) != 2 || server.queryRequests[0].GetPattern() != "inbound>>>" || server.queryRequests[1].GetPattern() != "outbound>>>" || !server.queryRequests[1].GetReset_() {
		t.Fatalf("queryRequests = %#v", server.queryRequests)
	}
}

func TestStatsClientReturnsQueryErrors(t *testing.T) {
	client := &statsClient{raw: failingStatsClient{}}
	if _, err := client.UsersStats(context.Background(), false); err == nil {
		t.Fatalf("UsersStats() error = nil")
	}
	if _, err := client.InboundStats(context.Background(), "tag", false); err == nil {
		t.Fatalf("InboundStats() error = nil")
	}
}

func startStatsServer(t *testing.T, server *recordingStatsServer) (statscommand.StatsServiceClient, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	statscommand.RegisterStatsServiceServer(grpcServer, server)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return statscommand.NewStatsServiceClient(conn), func() {
		_ = conn.Close()
		grpcServer.Stop()
		_ = listener.Close()
	}
}

type recordingStatsServer struct {
	statscommand.UnimplementedStatsServiceServer
	sysStats      *statscommand.SysStatsResponse
	getStats      map[string]int64
	queryStats    []*statscommand.Stat
	getRequests   []*statscommand.GetStatsRequest
	queryRequests []*statscommand.QueryStatsRequest
}

func (s *recordingStatsServer) GetSysStats(context.Context, *statscommand.SysStatsRequest) (*statscommand.SysStatsResponse, error) {
	if s.sysStats != nil {
		return s.sysStats, nil
	}
	return &statscommand.SysStatsResponse{}, nil
}

func (s *recordingStatsServer) GetStats(ctx context.Context, request *statscommand.GetStatsRequest) (*statscommand.GetStatsResponse, error) {
	s.getRequests = append(s.getRequests, request)
	if s.getStats == nil {
		return &statscommand.GetStatsResponse{}, nil
	}
	return &statscommand.GetStatsResponse{Stat: &statscommand.Stat{Name: request.GetName(), Value: s.getStats[request.GetName()]}}, nil
}

func (s *recordingStatsServer) QueryStats(ctx context.Context, request *statscommand.QueryStatsRequest) (*statscommand.QueryStatsResponse, error) {
	s.queryRequests = append(s.queryRequests, request)
	return &statscommand.QueryStatsResponse{Stat: s.queryStats}, nil
}

type failingStatsClient struct {
	statscommand.StatsServiceClient
}

func (failingStatsClient) GetStats(context.Context, *statscommand.GetStatsRequest, ...grpc.CallOption) (*statscommand.GetStatsResponse, error) {
	return nil, errors.New("boom")
}

func (failingStatsClient) QueryStats(context.Context, *statscommand.QueryStatsRequest, ...grpc.CallOption) (*statscommand.QueryStatsResponse, error) {
	return nil, errors.New("boom")
}
