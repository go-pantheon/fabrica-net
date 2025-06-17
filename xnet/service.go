package xnet

import (
	"context"
)

const (
	// PackLenSize is the size of the packet length (int32).
	PackLenSize = int32(4)
	// MaxPackSize is the maximum size of the body.
	MaxPackSize = int32(1 << 14)
)

// Service is the interface for the worker service.
type Service interface {
	Auth(ctx context.Context, in Pack) (out Pack, ss Session, err error)
	OnConnected(ctx context.Context, ss Session) (err error)
	OnDisconnect(ctx context.Context, ss Session) (err error)

	TunnelType(mod int32) (t int32, initCapacity int, err error)
	CreateAppTunnel(ctx context.Context, ss Session, tp int32, rid int64, w Worker) (t AppTunnel, err error)

	Handle(ctx context.Context, ss Session, tm TunnelManager, in Pack) (err error)
	Tick(ctx context.Context, ss Session) (err error)
}
