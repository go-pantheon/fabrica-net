package xnet

import (
	"context"

	"github.com/go-pantheon/fabrica-util/xsync"
)

// Worker is an interface that combines tunnel holding, pushing,
// and lifecycle management capabilities.
type Worker interface {
	xsync.WorkerDelayable
	xsync.Closable
	TunnelManager
	Pusher
}

// Pusher is an interface that provides the ability to push data through a tunnel.
type Pusher interface {
	Push(ctx context.Context, pack []byte) error
}
