package xnet

import (
	"context"

	"github.com/go-pantheon/fabrica-util/xsync"
)

// Tunnel is an interface for a communication channel that can
// push messages and forward specialized messages.
type Tunnel interface {
	xsync.Closable
	Pusher

	Type() int32
	Forward(ctx context.Context, msg ForwardMessage) error
}

// TunnelManager is an interface that combines Pusher functionality with the ability
// to retrieve a specific Tunnel instance.
type TunnelManager interface {
	Tunnel(ctx context.Context, key int32, oid int64) (Tunnel, error)
}

// ForwardMessage is an interface for messages that can be forwarded
// through a tunnel with module, sequence, object ID, and payload data.
type ForwardMessage interface {
	GetMod() int32
	GetSeq() int32
	GetObj() int64
	GetData() []byte
}
