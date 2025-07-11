package websocket

import (
	"context"
	"net"
	"net/http"
	"slices"
	"sync/atomic"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
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

	widGener *atomic.Uint64
	connChan chan connWrapper
}

type connWrapper struct {
	conn net.Conn
	wid  uint64
}

func newListener(bind string, path string, conf conf.Config) *listener {
	return &listener{
		bind:     bind,
		path:     path,
		conf:     conf,
		widGener: &atomic.Uint64{},
		connChan: make(chan connWrapper, 100),
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

	wid := l.widGener.Add(1)
	wsConn := wsconn.NewWebSocketConn(conn)

	select {
	case l.connChan <- connWrapper{conn: wsConn, wid: wid}:
	default:
		log.Error("[websocket.Listener] connection channel full, dropping connection")

		if err := conn.Close(); err != nil {
			log.Errorf("[websocket.Listener] close connection failed: %+v", err)
		}
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

func (l *listener) Accept(ctx context.Context, widGener *atomic.Uint64) (net.Conn, uint64, error) {
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	case wrapper := <-l.connChan:
		return wrapper.conn, wrapper.wid, nil
	}
}

func (l *listener) Endpoint() (string, error) {
	return "ws://" + l.bind + l.path, nil
}
