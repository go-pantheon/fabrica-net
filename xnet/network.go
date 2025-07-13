package xnet

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"
)

type Server interface {
	transport.Server
	transport.Endpointer

	Push(ctx context.Context, uid int64, pack Pack) error
	Multicast(ctx context.Context, uids []int64, pack Pack) error
	Broadcast(ctx context.Context, pack Pack) error
}

type Client interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	Send(pack Pack) error
	SendSmux(pack Pack, streamID int64) error
	Receive() <-chan Pack
}
