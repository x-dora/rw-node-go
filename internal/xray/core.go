package xray

import "context"

type Core interface {
	Start(ctx context.Context, config []byte) error
	Stop(ctx context.Context) error
	IsRunning() bool
	Version(ctx context.Context) (string, error)
	Handler() HandlerClient
	Stats() StatsClient
	Routing() RoutingClient
}

type HandlerClient interface{}

type StatsClient interface{}

type RoutingClient interface{}
