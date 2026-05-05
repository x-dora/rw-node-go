package xray

import (
	"context"

	handlercommand "github.com/xtls/xray-core/app/proxyman/command"
	routercommand "github.com/xtls/xray-core/app/router/command"
	statscommand "github.com/xtls/xray-core/app/stats/command"
)

type Core interface {
	Start(ctx context.Context, config []byte) error
	Stop(ctx context.Context) error
	IsRunning() bool
	Health(ctx context.Context) error
	Version(ctx context.Context) (string, error)
	Handler() HandlerClient
	Stats() StatsClient
	Routing() RoutingClient
}

type HandlerClient interface {
	AddUser(ctx context.Context, spec UserSpec) error
	RemoveUser(ctx context.Context, tag string, username string) error
	GetInboundUsers(ctx context.Context, tag string) ([]InboundUser, error)
	GetInboundUsersCount(ctx context.Context, tag string) (int, error)
	Raw() handlercommand.HandlerServiceClient
}

type StatsClient interface {
	Ping(ctx context.Context) error
	SysStats(ctx context.Context) (SysStats, error)
	UsersStats(ctx context.Context, reset bool) ([]UserTrafficStats, error)
	InboundStats(ctx context.Context, tag string, reset bool) (InboundTrafficStats, error)
	OutboundStats(ctx context.Context, tag string, reset bool) (OutboundTrafficStats, error)
	AllInboundStats(ctx context.Context, reset bool) ([]InboundTrafficStats, error)
	AllOutboundStats(ctx context.Context, reset bool) ([]OutboundTrafficStats, error)
	Raw() statscommand.StatsServiceClient
}

type RoutingClient interface {
	Raw() routercommand.RoutingServiceClient
}
