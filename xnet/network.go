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
	Dialog(dialogID uint64) (d ClientDialog, ok bool)
	DefaultDialog() (d ClientDialog, ok bool)
}

type ClientDialog interface {
	ID() uint64
	Send(pack Pack) (err error)
	Receive() <-chan Pack
	WaitAuthed()
}
