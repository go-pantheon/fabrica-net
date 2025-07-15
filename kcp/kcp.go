package kcp

import (
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/internal"
	kcpclient "github.com/go-pantheon/fabrica-net/kcp/client"
	kcpserver "github.com/go-pantheon/fabrica-net/kcp/server"
	"github.com/go-pantheon/fabrica-net/server"
	"github.com/go-pantheon/fabrica-net/xnet"
)

// NewServer creates a new KCP server
func NewServer(bind string, svc xnet.Service, opts ...server.Option) (*kcpserver.Server, error) {
	return kcpserver.NewServer(bind, svc, opts...)
}

// NewClient creates a new KCP client
func NewClient(id int64, target string, handshakePack internal.HandshakePackFunc, opts ...client.Option) (*kcpclient.Client, error) {
	return kcpclient.NewClient(id, target, handshakePack, opts...)
}
