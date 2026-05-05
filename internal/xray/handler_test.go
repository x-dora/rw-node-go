package xray

import (
	"context"
	"errors"
	"net"
	"testing"

	handlercommand "github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	hysteriaaccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	shadowsocks2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestBuildProtocolUserAccounts(t *testing.T) {
	tests := []struct {
		name     string
		spec     UserSpec
		assertFn func(*testing.T, *protocol.User)
	}{
		{
			name: "vless",
			spec: UserSpec{Protocol: ProtocolVLESS, Username: "user-1", UUID: "11111111-1111-4111-8111-111111111111", Flow: "xtls-rprx-vision"},
			assertFn: func(t *testing.T, user *protocol.User) {
				account := typedAccount[*vless.Account](t, user)
				if account.GetId() != "11111111-1111-4111-8111-111111111111" || account.GetFlow() != "xtls-rprx-vision" {
					t.Fatalf("vless account = %#v", account)
				}
			},
		},
		{
			name: "trojan",
			spec: UserSpec{Protocol: ProtocolTrojan, Username: "user-1", Password: "trojan-password"},
			assertFn: func(t *testing.T, user *protocol.User) {
				account := typedAccount[*trojan.Account](t, user)
				if account.GetPassword() != "trojan-password" {
					t.Fatalf("trojan password = %q", account.GetPassword())
				}
			},
		},
		{
			name: "shadowsocks",
			spec: UserSpec{Protocol: ProtocolShadowsocks, Username: "user-1", Password: "ss-password", CipherType: 7},
			assertFn: func(t *testing.T, user *protocol.User) {
				account := typedAccount[*shadowsocks.Account](t, user)
				if account.GetPassword() != "ss-password" || account.GetCipherType() != shadowsocks.CipherType_CHACHA20_POLY1305 || account.GetIvCheck() {
					t.Fatalf("shadowsocks account = %#v", account)
				}
			},
		},
		{
			name: "shadowsocks2022",
			spec: UserSpec{Protocol: ProtocolShadowsocks22, Username: "user-1", Key: "plain-key"},
			assertFn: func(t *testing.T, user *protocol.User) {
				account := typedAccount[*shadowsocks2022.Account](t, user)
				if account.GetKey() != "plain-key" {
					t.Fatalf("shadowsocks2022 key = %q", account.GetKey())
				}
			},
		},
		{
			name: "hysteria",
			spec: UserSpec{Protocol: ProtocolHysteria, Username: "user-1", Password: "hysteria-auth"},
			assertFn: func(t *testing.T, user *protocol.User) {
				account := typedAccount[*hysteriaaccount.Account](t, user)
				if account.GetAuth() != "hysteria-auth" {
					t.Fatalf("hysteria auth = %q", account.GetAuth())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := BuildProtocolUser(tt.spec)
			if err != nil {
				t.Fatalf("BuildProtocolUser() error = %v", err)
			}
			if user.GetEmail() != "user-1" || user.GetLevel() != 0 {
				t.Fatalf("user = %#v", user)
			}
			tt.assertFn(t, user)
		})
	}
}

func TestHandlerClientOperations(t *testing.T) {
	server := &fakeHandlerServer{
		users: []*protocol.User{
			{Email: "user-1", Level: 0},
			{Email: "user-2", Level: 2},
		},
		count: 2,
	}
	raw, stop := startFakeHandlerServer(t, server)
	defer stop()
	client := &handlerClient{raw: raw}

	ctx := context.Background()
	if err := client.AddUser(ctx, UserSpec{Protocol: ProtocolTrojan, Tag: "TROJAN_INBOUND", Username: "user-1", Password: "pw"}); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
	if err := client.RemoveUser(ctx, "VLESS_INBOUND", "user-1"); err != nil {
		t.Fatalf("RemoveUser() error = %v", err)
	}
	users, err := client.GetInboundUsers(ctx, "VLESS_INBOUND")
	if err != nil {
		t.Fatalf("GetInboundUsers() error = %v", err)
	}
	count, err := client.GetInboundUsersCount(ctx, "VLESS_INBOUND")
	if err != nil {
		t.Fatalf("GetInboundUsersCount() error = %v", err)
	}

	if count != 2 || len(users) != 2 || users[1].Username != "user-2" || users[1].Level != 2 {
		t.Fatalf("users=%#v count=%d", users, count)
	}
	if len(server.alterRequests) != 2 {
		t.Fatalf("alterRequests len = %d, want 2", len(server.alterRequests))
	}
	add := typedOperation[*handlercommand.AddUserOperation](t, server.alterRequests[0].GetOperation())
	if server.alterRequests[0].GetTag() != "TROJAN_INBOUND" || add.GetUser().GetEmail() != "user-1" {
		t.Fatalf("add request = %#v", server.alterRequests[0])
	}
	remove := typedOperation[*handlercommand.RemoveUserOperation](t, server.alterRequests[1].GetOperation())
	if server.alterRequests[1].GetTag() != "VLESS_INBOUND" || remove.GetEmail() != "user-1" {
		t.Fatalf("remove request = %#v", server.alterRequests[1])
	}
}

func TestHandlerClientReturnsErrors(t *testing.T) {
	client := &handlerClient{raw: failingHandlerClient{}}
	if err := client.AddUser(context.Background(), UserSpec{Protocol: ProtocolTrojan, Tag: "tag", Username: "user", Password: "pw"}); err == nil {
		t.Fatalf("AddUser() error = nil")
	}
	if err := client.RemoveUser(context.Background(), "tag", "user"); err == nil {
		t.Fatalf("RemoveUser() error = nil")
	}
}

func typedAccount[T any](t *testing.T, user *protocol.User) T {
	t.Helper()
	instance, err := user.GetAccount().GetInstance()
	if err != nil {
		t.Fatalf("GetInstance() error = %v", err)
	}
	account, ok := instance.(T)
	if !ok {
		t.Fatalf("account type = %T", instance)
	}
	return account
}

func typedOperation[T any](t *testing.T, message *serial.TypedMessage) T {
	t.Helper()
	instance, err := message.GetInstance()
	if err != nil {
		t.Fatalf("GetInstance() error = %v", err)
	}
	op, ok := instance.(T)
	if !ok {
		t.Fatalf("operation type = %T", instance)
	}
	return op
}

func startFakeHandlerServer(t *testing.T, server *fakeHandlerServer) (handlercommand.HandlerServiceClient, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	handlercommand.RegisterHandlerServiceServer(grpcServer, server)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return handlercommand.NewHandlerServiceClient(conn), func() {
		_ = conn.Close()
		grpcServer.Stop()
		_ = listener.Close()
	}
}

type fakeHandlerServer struct {
	handlercommand.UnimplementedHandlerServiceServer
	alterRequests []*handlercommand.AlterInboundRequest
	users         []*protocol.User
	count         int64
}

func (s *fakeHandlerServer) AlterInbound(ctx context.Context, request *handlercommand.AlterInboundRequest) (*handlercommand.AlterInboundResponse, error) {
	s.alterRequests = append(s.alterRequests, request)
	return &handlercommand.AlterInboundResponse{}, nil
}

func (s *fakeHandlerServer) GetInboundUsers(ctx context.Context, request *handlercommand.GetInboundUserRequest) (*handlercommand.GetInboundUserResponse, error) {
	return &handlercommand.GetInboundUserResponse{Users: s.users}, nil
}

func (s *fakeHandlerServer) GetInboundUsersCount(ctx context.Context, request *handlercommand.GetInboundUserRequest) (*handlercommand.GetInboundUsersCountResponse, error) {
	return &handlercommand.GetInboundUsersCountResponse{Count: s.count}, nil
}

type failingHandlerClient struct {
	handlercommand.HandlerServiceClient
}

func (failingHandlerClient) AlterInbound(context.Context, *handlercommand.AlterInboundRequest, ...grpc.CallOption) (*handlercommand.AlterInboundResponse, error) {
	return nil, errors.New("boom")
}
