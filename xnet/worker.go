package xnet

import (
	"context"
	"io"

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
	Push(ctx context.Context, pack Pack) error
}

type Pack []byte

type StreamFrameCodec interface {
	Encode(io.Writer, Pack) error
	Decode(io.Reader) (Pack, error)
}
