package client

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/xnet"
)

var _ xnet.Client = (*Client)(nil)

// Client is a KCP client implementation
type Client struct {
	*internal.BaseClient

	target string
}

// NewClient creates a new KCP client
func NewClient(id int64, target string, handshakePack xnet.Pack, opts ...client.Option) *Client {
	dialer := NewDialer(id, target, conf.Default().KCP)
	baseClient := internal.NewBaseClient(id, handshakePack, dialer, opts...)

	c := &Client{
		BaseClient: baseClient,
		target:     target,
	}

	return c
}

// Start starts the KCP client
func (c *Client) Start(ctx context.Context) error {
	log.Infof("[kcp.Client] connecting to %s", c.target)
	return c.BaseClient.Start(ctx)
}

// Stop stops the KCP client
func (c *Client) Stop(ctx context.Context) error {
	log.Infof("[kcp.Client] stopping")
	return c.BaseClient.Stop(ctx)
}
