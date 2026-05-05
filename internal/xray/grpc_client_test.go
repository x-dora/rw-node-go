package xray

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	statscommand "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestGRPCStatsClientPing(t *testing.T) {
	bundle := testMTLS(t)
	address, stop := startFakeStatsServer(t, bundle)
	defer stop()

	client, err := NewGRPCClient(GRPCClientConfig{Address: address, MTLS: bundle})
	if err != nil {
		t.Fatalf("NewGRPCClient() error = %v", err)
	}
	defer client.Close()

	stats, err := client.Stats()
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := stats.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

func TestGRPCStatsClientPingFailsWithWrongCA(t *testing.T) {
	serverBundle := testMTLS(t)
	address, stop := startFakeStatsServer(t, serverBundle)
	defer stop()

	clientBundle := testMTLS(t)
	client, err := NewGRPCClient(GRPCClientConfig{Address: address, MTLS: clientBundle})
	if err != nil {
		t.Fatalf("NewGRPCClient() error = %v", err)
	}
	defer client.Close()

	stats, err := client.Stats()
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := stats.Ping(ctx); err == nil {
		t.Fatalf("Ping() error = nil, want TLS failure")
	}
}

func startFakeStatsServer(t *testing.T, bundle InternalMTLSBundle) (string, func()) {
	t.Helper()
	serverCert, err := tls.X509KeyPair([]byte(bundle.ServerCertPEM), []byte(bundle.ServerKeyPEM))
	if err != nil {
		t.Fatalf("load server keypair: %v", err)
	}
	clientCAs := x509.NewCertPool()
	if ok := clientCAs.AppendCertsFromPEM([]byte(bundle.CACertPEM)); !ok {
		t.Fatalf("load client CA")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	})))
	statscommand.RegisterStatsServiceServer(server, fakeStatsServer{})
	go func() {
		_ = server.Serve(listener)
	}()
	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
	}
}

type fakeStatsServer struct {
	statscommand.UnimplementedStatsServiceServer
}

func (fakeStatsServer) GetSysStats(context.Context, *statscommand.SysStatsRequest) (*statscommand.SysStatsResponse, error) {
	return &statscommand.SysStatsResponse{NumGoroutine: 1}, nil
}
