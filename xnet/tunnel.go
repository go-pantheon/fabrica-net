package xnet

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-util/xsync"
)

// Tunnel is an interface for a communication channel that can
// push messages and forward specialized messages.
type Tunnel interface {
	xsync.Stoppable

	Type() int32
	Forward(ctx context.Context, msg TunnelMessage) error
	PacketToTunnelMsg(from PacketMessage) (to TunnelMessage)
}

type AppTunnel interface {
	AppBaseTunnel

	CSHandle(msg TunnelMessage) error
	SCHandle() (TunnelMessage, error)
	PacketToTunnelMsg(from PacketMessage) (to TunnelMessage)
	TunnelMsgToPack(ctx context.Context, msg TunnelMessage) (pack Pack, err error)
	OnStop(ctx context.Context, erreason error) (err error)
}

type AppBaseTunnel interface {
	Log() *log.Helper
	Type() int32
	UID() int64
	Color() string
	OID() int64
	Session() Session
}

// TunnelManager is an interface that combines Pusher functionality with the ability
// to retrieve a specific Tunnel instance.
type TunnelManager interface {
	Tunnel(ctx context.Context, key int32, oid int64) (Tunnel, error)
}

// PacketMessage is an interface for messages that from client app can be transformed
// into a ForwardMessage.
type PacketMessage interface {
	BaseMessage
}

// TunnelMessage is an interface for messages that can be forwarded to another application
type TunnelMessage interface {
	BaseMessage
	SmuxParam
}

type KCPTunnelMessage interface {
	BaseMessage
	SmuxParam
}

// BaseMessage is an common interface for messages that for transmission
type BaseMessage interface {
	GetMod() int32
	GetSeq() int32
	GetObj() int64
	GetData() []byte
	GetDataVersion() uint64
	GetIndex() int32
}

type SmuxParam interface {
	GetStreamID() int64
	GetFragID() int32
	GetFragCount() int32
	GetFragIndex() int32
}
