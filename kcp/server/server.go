package server

import (
	"context"
	"net/url"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/server"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
)

var _ xnet.Server = (*Server)(nil)

// Server is a KCP server implementation
type Server struct {
	*internal.BaseServer

	bind string
}

// NewServer creates a new KCP server
func NewServer(bind string, svc xnet.Service, opts ...server.Option) (*Server, error) {
	if bind == "" {
		return nil, errors.New("bind is required")
	}

	options := server.NewOptions(opts...)
	listener, err := newListener(bind, options.Conf().KCP)
	if err != nil {
		return nil, err
	}

	baseServer, err := internal.NewBaseServer(listener, svc, options)
	if err != nil {
		return nil, err
	}

	s := &Server{
		BaseServer: baseServer,
		bind:       bind,
	}

	return s, nil
}

// Start starts the KCP server
func (s *Server) Start(ctx context.Context) error {
	log.Infof("[kcp.Server] starting on %s smux=%t streamSize=%d", s.bind, s.Conf().KCP.Smux, s.Conf().KCP.SmuxStreamSize)
	return s.BaseServer.Start(ctx)
}

// Stop stops the KCP server
func (s *Server) Stop(ctx context.Context) error {
	log.Infof("[kcp.Server] stopping")
	return s.BaseServer.Stop(ctx)
}

// Endpoint returns the listening endpoint
func (s *Server) Endpoint() (*url.URL, error) {
	return s.BaseServer.Endpoint()
}
