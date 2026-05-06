package xray

import (
	"testing"

	"github.com/xtls/xray-core/common/protocol"
	hysteriaaccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	shadowsocks2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
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
