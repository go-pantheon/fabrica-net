package websocket

import (
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/xnet"
)

var _ xnet.Client = (*Client)(nil)

type Client struct {
	*internal.BaseClient

	url string
}

func NewClient(id int64, url string, handshakePack xnet.Pack, opts ...client.Option) *Client {
	dialer := newDialer(id, url, "*")
	baseClient := internal.NewBaseClient(id, handshakePack, dialer, opts...)

	return &Client{
		BaseClient: baseClient,
		url:        url,
	}
}

func (c *Client) URL() string {
	return c.url
}
