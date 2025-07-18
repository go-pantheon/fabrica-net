package client

import (
	"github.com/go-pantheon/fabrica-net/client"
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
func NewClient(id int64, target string, handshakePack client.HandshakePackFunc, opts ...client.Option) (*Client, error) {
	options := client.NewOptions(opts...)

	dialer, err := NewDialer(id, target, options.Conf().KCP)
	if err != nil {
		return nil, err
	}

	baseClient := internal.NewBaseClient(id, handshakePack, dialer, options)

	return &Client{
		BaseClient: baseClient,
		target:     target,
	}, nil
}

// Target returns the target address for interface consistency
func (c *Client) Target() string {
	return c.target
}
