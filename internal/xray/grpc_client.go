package xray

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync"

	handlercommand "github.com/xtls/xray-core/app/proxyman/command"
	routercommand "github.com/xtls/xray-core/app/router/command"
	statscommand "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/serial"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type GRPCClientConfig struct {
	Address string
	MTLS    InternalMTLSBundle
}

type GRPCClient struct {
	address string
	tls     credentials.TransportCredentials

	mu      sync.Mutex
	conn    *grpc.ClientConn
	stats   *statsClient
	handler *handlerClient
	routing *routingClient
}

func NewGRPCClient(cfg GRPCClientConfig) (*GRPCClient, error) {
	creds, err := transportCredentials(cfg.MTLS)
	if err != nil {
		return nil, err
	}
	return &GRPCClient{address: cfg.Address, tls: creds}, nil
}

func (c *GRPCClient) Stats() (StatsClient, error) {
	conn, err := c.connection()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stats == nil {
		c.stats = &statsClient{raw: statscommand.NewStatsServiceClient(conn)}
	}
	return c.stats, nil
}

func (c *GRPCClient) Handler() (HandlerClient, error) {
	conn, err := c.connection()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handler == nil {
		c.handler = &handlerClient{raw: handlercommand.NewHandlerServiceClient(conn)}
	}
	return c.handler, nil
}

func (c *GRPCClient) Routing() (RoutingClient, error) {
	conn, err := c.connection()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.routing == nil {
		c.routing = &routingClient{raw: routercommand.NewRoutingServiceClient(conn)}
	}
	return c.routing, nil
}

func (c *GRPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.stats = nil
	c.handler = nil
	c.routing = nil
	return err
}

func (c *GRPCClient) connection() (*grpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}
	conn, err := grpc.NewClient(c.address, grpc.WithTransportCredentials(c.tls))
	if err != nil {
		return nil, fmt.Errorf("create xray gRPC client: %w", err)
	}
	c.conn = conn
	return conn, nil
}

func transportCredentials(bundle InternalMTLSBundle) (credentials.TransportCredentials, error) {
	cert, err := tls.X509KeyPair([]byte(bundle.ClientCertPEM), []byte(bundle.ClientKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("load internal client keypair: %w", err)
	}
	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM([]byte(bundle.CACertPEM)); !ok {
		return nil, fmt.Errorf("load internal CA")
	}
	return credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		ServerName:   InternalServerName,
		RootCAs:      roots,
		Certificates: []tls.Certificate{cert},
	}), nil
}

type statsClient struct {
	raw statscommand.StatsServiceClient
}

func (c *statsClient) Ping(ctx context.Context) error {
	if _, err := c.raw.GetSysStats(ctx, &statscommand.SysStatsRequest{}); err != nil {
		return fmt.Errorf("xray stats get system stats: %w", err)
	}
	return nil
}

func (c *statsClient) Raw() statscommand.StatsServiceClient {
	return c.raw
}

type handlerClient struct {
	raw handlercommand.HandlerServiceClient
}

func (c *handlerClient) AddUser(ctx context.Context, spec UserSpec) error {
	user, err := BuildProtocolUser(spec)
	if err != nil {
		return err
	}
	_, err = c.raw.AlterInbound(ctx, &handlercommand.AlterInboundRequest{
		Tag: spec.Tag,
		Operation: serial.ToTypedMessage(&handlercommand.AddUserOperation{
			User: user,
		}),
	})
	if err != nil {
		return fmt.Errorf("xray handler add user to inbound %q: %w", spec.Tag, err)
	}
	return nil
}

func (c *handlerClient) RemoveUser(ctx context.Context, tag string, username string) error {
	_, err := c.raw.AlterInbound(ctx, &handlercommand.AlterInboundRequest{
		Tag: tag,
		Operation: serial.ToTypedMessage(&handlercommand.RemoveUserOperation{
			Email: username,
		}),
	})
	if err != nil {
		return fmt.Errorf("xray handler remove user from inbound %q: %w", tag, err)
	}
	return nil
}

func (c *handlerClient) GetInboundUsers(ctx context.Context, tag string) ([]InboundUser, error) {
	response, err := c.raw.GetInboundUsers(ctx, &handlercommand.GetInboundUserRequest{Tag: tag})
	if err != nil {
		return nil, fmt.Errorf("xray handler get inbound users %q: %w", tag, err)
	}
	users := make([]InboundUser, 0, len(response.GetUsers()))
	for _, user := range response.GetUsers() {
		if user == nil {
			continue
		}
		email := user.GetEmail()
		users = append(users, InboundUser{
			Username: email,
			Email:    email,
			Level:    int(user.GetLevel()),
		})
	}
	return users, nil
}

func (c *handlerClient) GetInboundUsersCount(ctx context.Context, tag string) (int, error) {
	response, err := c.raw.GetInboundUsersCount(ctx, &handlercommand.GetInboundUserRequest{Tag: tag})
	if err != nil {
		return 0, fmt.Errorf("xray handler get inbound users count %q: %w", tag, err)
	}
	return int(response.GetCount()), nil
}

func (c *handlerClient) Raw() handlercommand.HandlerServiceClient {
	return c.raw
}

type routingClient struct {
	raw routercommand.RoutingServiceClient
}

func (c *routingClient) Raw() routercommand.RoutingServiceClient {
	return c.raw
}
