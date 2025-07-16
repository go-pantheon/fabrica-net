package internal

import (
	"context"
	"net"
	"sync/atomic"
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
	Codec Codec
}

func NewConnWrapper(wid uint64, conn net.Conn, codec Codec) ConnWrapper {
	return ConnWrapper{
		WID:   wid,
		Conn:  conn,
		Codec: codec,
	}
}

func (c ConnWrapper) Close() error {
	if c.Codec == nil {
		return nil
	}

	return c.Conn.Close()
}

type ConnIDGenerator struct {
	counter *atomic.Uint64
	netType int
}

func NewConnIDGenerator(netType int) *ConnIDGenerator {
	return &ConnIDGenerator{
		counter: &atomic.Uint64{},
		netType: netType,
	}
}

func (w *ConnIDGenerator) Next() uint64 {
	return w.counter.Add(1)<<4 | uint64(w.netType)
}
