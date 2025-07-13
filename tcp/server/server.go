package tcp

import (
	"context"
	"net/url"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/server"
	"github.com/go-pantheon/fabrica-net/xnet"
)

var _ xnet.Server = (*Server)(nil)

type Server struct {
	*internal.BaseServer

	bind string
}

func NewServer(bind string, svc xnet.Service, opts ...server.Option) (*Server, error) {
	options := server.NewOptions(opts...)
	listener := newListener(bind, options.Conf().TCP)

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

func (s *Server) Start(ctx context.Context) error {
	log.Infof("[tcp.Server] starting on %s", s.bind)
	return s.BaseServer.Start(ctx)
}

func (s *Server) Stop(ctx context.Context) error {
	log.Infof("[tcp.Server] stopping")
	return s.BaseServer.Stop(ctx)
}

func (s *Server) Endpoint() (*url.URL, error) {
	return s.BaseServer.Endpoint()
}
