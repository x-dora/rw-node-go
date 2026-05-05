package xray

import (
	"fmt"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	hysteriaaccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	shadowsocks2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"google.golang.org/protobuf/proto"
)

type User struct {
	Username string
	Protocol UserProtocol
}

type UserSpec struct {
	Protocol   UserProtocol
	Tag        string
	Username   string
	Password   string
	UUID       string
	Flow       string
	CipherType int
	Key        string
}

type InboundUser struct {
	Username string
	Email    string
	Level    int
}

func BuildProtocolUser(spec UserSpec) (*protocol.User, error) {
	if spec.Username == "" {
		return nil, fmt.Errorf("xray user username is empty")
	}

	var account any
	switch spec.Protocol {
	case ProtocolVLESS:
		account = &vless.Account{Id: spec.UUID, Flow: spec.Flow}
	case ProtocolTrojan:
		account = &trojan.Account{Password: spec.Password}
	case ProtocolShadowsocks:
		account = &shadowsocks.Account{
			Password:   spec.Password,
			CipherType: shadowsocks.CipherType(spec.CipherType),
			IvCheck:    false,
		}
	case ProtocolShadowsocks22:
		account = &shadowsocks2022.Account{Key: firstNonEmpty(spec.Key, spec.Password)}
	case ProtocolHysteria:
		account = &hysteriaaccount.Account{Auth: firstNonEmpty(spec.Password, spec.UUID)}
	default:
		return nil, fmt.Errorf("unsupported xray user protocol %q", spec.Protocol)
	}

	message, ok := account.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("unsupported xray user account for protocol %q", spec.Protocol)
	}

	return &protocol.User{
		Level:   0,
		Email:   spec.Username,
		Account: serial.ToTypedMessage(message),
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
