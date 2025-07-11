package xnet

import (
	"context"
	"io"
	"net"

	"github.com/go-pantheon/fabrica-util/xsync"
)

// Worker is an interface that combines tunnel holding, pushing,
// and lifecycle management capabilities.
type Worker interface {
	xsync.Stoppable
	TunnelManager
	Pusher

	WID() uint64
	Conn() net.Conn
	Session() Session
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
