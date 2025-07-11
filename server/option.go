package server

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-pantheon/fabrica-net/conf"
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

func WithAfterConnectFunc(f WrapperFunc) Option {
	return func(o *Options) {
		o.afterConnectFunc = f
	}
}

func WithAfterDisconnectFunc(f WrapperFunc) Option {
	return func(o *Options) {
		o.afterDisconnectFunc = f
	}
}

type WrapperFunc func(ctx context.Context, uid int64, color string) error

type Options struct {
	conf                conf.Config
	logger              log.Logger
	referer             string
	afterConnectFunc    WrapperFunc
	afterDisconnectFunc WrapperFunc
	readFilter          middleware.Middleware
	writeFilter         middleware.Middleware
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
		afterConnectFunc: func(ctx context.Context, uid int64, color string) error {
			return nil
		},
		afterDisconnectFunc: func(ctx context.Context, uid int64, color string) error {
			return nil
		},
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

func (o *Options) AfterConnectFunc() WrapperFunc {
	return o.afterConnectFunc
}

func (o *Options) AfterDisconnectFunc() WrapperFunc {
	return o.afterDisconnectFunc
}

func (o *Options) ReadFilter() middleware.Middleware {
	return o.readFilter
}

func (o *Options) WriteFilter() middleware.Middleware {
	return o.writeFilter
}
