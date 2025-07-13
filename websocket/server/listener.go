package websocket

import (
	"context"
	"net"
	"net/http"
	"slices"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/websocket/frame"
	"github.com/go-pantheon/fabrica-net/websocket/wsconn"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"github.com/gorilla/websocket"
)

var _ internal.Listener = (*listener)(nil)

type listener struct {
	bind string
	path string
	conf conf.Config

	listener net.Listener
	server   *http.Server
	upgrader websocket.Upgrader

	widGener *internal.WIDGenerator
	connChan chan internal.ConnWrapper
}

func newListener(bind string, path string, conf conf.Config) *listener {
	return &listener{
		bind:     bind,
		path:     path,
		conf:     conf,
		widGener: internal.NewWIDGenerator(internal.NetTypeWebSocket),
		connChan: make(chan internal.ConnWrapper, 1024),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  conf.WebSocket.ReadBufSize,
			WriteBufferSize: conf.WebSocket.WriteBufSize,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return slices.Contains(conf.WebSocket.AllowOrigins, origin)
			},
		},
	}
}

func (l *listener) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(l.path, l.handleWebSocket)

	l.server = &http.Server{
		Addr:         l.bind,
		Handler:      mux,
		ReadTimeout:  l.conf.WebSocket.ReadTimeout,
		WriteTimeout: l.conf.WebSocket.WriteTimeout,
	}

	listener, err := net.Listen("tcp", l.bind)
	if err != nil {
		return errors.Wrapf(err, "listen failed. bind=%s", l.bind)
	}

	l.listener = listener

	xsync.Go("websocket.Listener", func() error {
		return l.server.Serve(listener)
	})

	return nil
}

func (l *listener) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("[websocket.Listener] upgrade failed: %+v", err)
		return
	}

	wid := l.widGener.Next()
	wsConn := wsconn.NewWebSocketConn(conn)
	codec := frame.New(wsConn)

	select {
	case l.connChan <- internal.NewConnWrapper(wid, wsConn, codec):
	default:
		log.Error("[websocket.Listener] connection channel full, dropping connection")
	}
}

func (l *listener) Stop(ctx context.Context) error {
	close(l.connChan)

	if l.server != nil {
		if err := l.server.Shutdown(ctx); err != nil {
			return err
		}
	}

	if l.listener != nil {
		if err := l.listener.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (l *listener) Accept(ctx context.Context) (internal.ConnWrapper, error) {
	select {
	case <-ctx.Done():
		return internal.ConnWrapper{}, ctx.Err()
	case wrapper := <-l.connChan:
		return wrapper, nil
	}
}

func (l *listener) Endpoint() (string, error) {
	return "ws://" + l.bind + l.path, nil
}
