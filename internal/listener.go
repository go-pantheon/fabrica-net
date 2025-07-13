package internal

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/go-pantheon/fabrica-net/codec"
)

const (
	NetTypeTCP = iota
	NetTypeWebSocket
	NetTypeKCP
)

// Listener defines the interface for protocol-specific connection handling
type Listener interface {
	// Start starts the listener and begins accepting connections
	Start(ctx context.Context) error

	// Stop stops the listener gracefully
	Stop(ctx context.Context) error

	// Accept accepts a new connection and returns it as net.Conn
	Accept(ctx context.Context) (ConnWrapper, error)

	// Endpoint returns the endpoint URL for this listener
	Endpoint() (string, error)
}

type ConnWrapper struct {
	WID   uint64
	Conn  net.Conn
	Codec codec.Codec
}

func NewConnWrapper(wid uint64, conn net.Conn, codec codec.Codec) ConnWrapper {
	return ConnWrapper{
		WID:   wid,
		Conn:  conn,
		Codec: codec,
	}
}

type WIDGenerator struct {
	counter *atomic.Uint64
	netType int
}

func NewWIDGenerator(netType int) *WIDGenerator {
	return &WIDGenerator{
		counter: &atomic.Uint64{},
		netType: netType,
	}
}

func (w *WIDGenerator) Next() uint64 {
	return w.counter.Add(1)<<4 | uint64(w.netType)
}
