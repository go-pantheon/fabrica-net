package xnet

import (
	"context"
)

const (
	// PackLenSize is the size of the packet length.
	PackLenSize = 4
	// MaxBodySize is the maximum size of the body.
	MaxBodySize = int32(1 << 14)
)

// Service is the interface for the worker service.
type Service interface {
	Auth(ctx context.Context, in []byte) (out []byte, ss Session, err error)
	OnConnected(ctx context.Context, ss Session) (err error)
	OnDisconnect(ctx context.Context, ss Session) (err error)

	TunnelType(mod int32) (t int32, initCapacity int, err error)
	CreateTunnel(ctx context.Context, ss Session, tp int32, rid int64, w Worker) (t Tunnel, err error)

	Handle(ctx context.Context, ss Session, tm TunnelManager, in []byte) (err error)
	Tick(ctx context.Context, ss Session) (err error)
}
