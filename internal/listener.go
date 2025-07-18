package internal

import (
	"context"
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
	Accept(ctx context.Context) (ConnCarrier, error)

	// Endpoint returns the endpoint URL for this listener
	Endpoint() (string, error)
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

func (g *ConnIDGenerator) Next() uint64 {
	return g.counter.Add(1)<<4 | uint64(g.netType)
}
