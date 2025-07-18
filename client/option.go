package client

import (
	"context"
	"errors"

	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/xnet"
)

// ErrTimeout is an error that occurs when the client times out.
var ErrTimeout = errors.New("i/o timeout")

type Option func(o *Options)

type AuthFunc func(ctx context.Context, pack xnet.Pack) (xnet.Session, error)

type HandshakePackFunc func(wid uint64) (xnet.Pack, error)

func WithAuthFunc(authFunc AuthFunc) Option {
	return func(o *Options) {
		o.authFunc = authFunc
	}
}

func WithConf(conf conf.Config) Option {
	return func(o *Options) {
		o.conf = conf
	}
}

func defaultAuthFunc(ctx context.Context, pack xnet.Pack) (xnet.Session, error) {
	return xnet.DefaultSession(), nil
}

type Options struct {
	authFunc AuthFunc
	conf     conf.Config
}

func NewOptions(opts ...Option) *Options {
	o := &Options{
		authFunc: defaultAuthFunc,
		conf:     conf.Default(),
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (o *Options) AuthFunc() AuthFunc {
	return o.authFunc
}

func (o *Options) Conf() conf.Config {
	return o.conf
}

func DialogID(id int64, index int) uint64 {
	return uint64(id)*100 + uint64(index)
}
