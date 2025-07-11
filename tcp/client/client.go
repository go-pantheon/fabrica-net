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

func NewClient(id int64, bind string, handshakePack xnet.Pack, opts ...client.Option) *Client {
	dialer := newDialer(id, bind)
	baseClient := internal.NewBaseClient(id, handshakePack, dialer, opts...)

	return &Client{
		BaseClient: baseClient,
		bind:       bind,
	}
}

func (c *Client) Bind() string {
	return c.bind
}
