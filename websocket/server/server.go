package websocket

import (
	"context"
	"net/url"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/server"
	"github.com/go-pantheon/fabrica-net/websocket/frame"
	"github.com/go-pantheon/fabrica-net/xnet"
)

var _ xnet.Server = (*Server)(nil)

type Server struct {
	*internal.BaseServer

	bind string
	path string
}

func NewServer(bind string, path string, svc xnet.Service, opts ...server.Option) (*Server, error) {
	options := server.NewOptions(opts...)
	listener := newListener(bind, path, options.Conf())

	baseServer, err := internal.NewBaseServer(listener, svc, frame.New, options)
	if err != nil {
		return nil, err
	}

	s := &Server{
		BaseServer: baseServer,
		bind:       bind,
		path:       path,
	}

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	log.Infof("[websocket.Server] starting on %s%s", s.bind, s.path)
	return s.BaseServer.Start(ctx)
}

func (s *Server) Stop(ctx context.Context) error {
	log.Infof("[websocket.Server] stopping")
	return s.BaseServer.Stop(ctx)
}

func (s *Server) Endpoint() (*url.URL, error) {
	return s.BaseServer.Endpoint()
}
