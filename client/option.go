package client

import (
	"context"
	"errors"

	"github.com/go-pantheon/fabrica-net/xnet"
)

// ErrTimeout is an error that occurs when the client times out.
var ErrTimeout = errors.New("i/o timeout")

type Option func(o *Options)

type AuthFunc func(ctx context.Context, pack xnet.Pack) (xnet.Session, error)

func WithAuthFunc(authFunc AuthFunc) Option {
	return func(o *Options) {
		o.authFunc = authFunc
	}
}

func defaultAuthFunc(ctx context.Context, pack xnet.Pack) (xnet.Session, error) {
	return xnet.DefaultSession(), nil
}

type Options struct {
	authFunc AuthFunc
}

func NewOptions(opts ...Option) *Options {
	o := &Options{
		authFunc: defaultAuthFunc,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (o *Options) AuthFunc() AuthFunc {
	return o.authFunc
}
