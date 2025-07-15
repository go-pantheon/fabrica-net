package tcp

import (
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/xnet"
)

var _ xnet.Client = (*Client)(nil)

type Client struct {
	*internal.BaseClient

	bind string
}

func NewClient(id int64, bind string, handshakePack internal.HandshakePackFunc, opts ...client.Option) *Client {
	baseClient := internal.NewBaseClient(id, handshakePack, newDialer(id, bind), client.NewOptions(opts...))

	return &Client{
		BaseClient: baseClient,
		bind:       bind,
	}
}

func (c *Client) Bind() string {
	return c.bind
}
