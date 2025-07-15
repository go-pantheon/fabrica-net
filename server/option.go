package server

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/websocket/wsconn"
	"github.com/go-pantheon/fabrica-net/xnet"
)

type Option func(o *Options)

func WithConf(conf conf.Config) Option {
	return func(o *Options) {
		o.conf = conf
	}
}

func WithReferer(referer string) Option {
	return func(o *Options) {
		o.referer = referer
	}
}

func WithLogger(logger log.Logger) Option {
	return func(o *Options) {
		o.logger = logger
	}
}

func WithReadFilter(m middleware.Middleware) Option {
	return func(o *Options) {
		if o.readFilter == nil {
			o.readFilter = m
			return
		}

		o.readFilter = middleware.Chain(o.readFilter, m)
	}
}

func WithWriteFilter(m middleware.Middleware) Option {
	return func(o *Options) {
		if o.writeFilter == nil {
			o.writeFilter = m
			return
		}

		o.writeFilter = middleware.Chain(o.writeFilter, m)
	}
}

func WithAfterConnect(f Inspector) Option {
	return func(o *Options) {
		o.afterConnect = Wrap(o.afterConnect, f)
	}
}

func WithAfterDisconnect(f Inspector) Option {
	return func(o *Options) {
		o.afterDisconnect = Wrap(o.afterDisconnect, f)
	}
}

func WithWsReadLimit(limit int64) Option {
	return func(o *Options) {
		o.afterConnect = Wrap(o.afterConnect, func(f InspectorFunc) InspectorFunc {
			return func(ctx context.Context, w xnet.Worker) error {
				if conn, ok := w.Conn().(*wsconn.WebSocketConn); ok {
					conn.SetReadLimit(limit)
				}

				return f(ctx, w)
			}
		})
	}
}

func WithPongRenewal(timeout time.Duration) Option {
	return func(o *Options) {
		o.afterConnect = Wrap(o.afterConnect, func(f InspectorFunc) InspectorFunc {
			return func(ctx context.Context, w xnet.Worker) error {
				if conn, ok := w.Conn().(*wsconn.WebSocketConn); ok {
					conn.SetPongHandler(func(string) error {
						return conn.SetReadDeadline(time.Now().Add(timeout))
					})
				}

				return f(ctx, w)
			}
		})
	}
}

type Options struct {
	conf            conf.Config
	logger          log.Logger
	referer         string
	afterConnect    Inspector
	afterDisconnect Inspector
	readFilter      middleware.Middleware
	writeFilter     middleware.Middleware
}

func NewOptions(opts ...Option) *Options {
	ret := &Options{
		conf:   conf.Default(),
		logger: log.DefaultLogger,
		readFilter: middleware.Chain(
			recovery.Recovery(),
		),
		writeFilter: middleware.Chain(
			recovery.Recovery(),
		),
		afterConnect:    emptyInspector,
		afterDisconnect: emptyInspector,
	}

	for _, o := range opts {
		o(ret)
	}

	return ret
}

func (o *Options) Conf() conf.Config {
	return o.conf
}

func (o *Options) Logger() log.Logger {
	return o.logger
}

func (o *Options) Referer() string {
	return o.referer
}

func (o *Options) AfterConnect() Inspector {
	return o.afterConnect
}

func (o *Options) AfterDisconnect() Inspector {
	return o.afterDisconnect
}

func (o *Options) ReadFilter() middleware.Middleware {
	return o.readFilter
}

func (o *Options) WriteFilter() middleware.Middleware {
	return o.writeFilter
}
