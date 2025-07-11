package internal

import (
	"context"
	"net"
	"sync/atomic"
)

// Listener defines the interface for protocol-specific connection handling
type Listener interface {
	// Start starts the listener and begins accepting connections
	Start(ctx context.Context) error

	// Stop stops the listener gracefully
	Stop(ctx context.Context) error

	// Accept accepts a new connection and returns it as net.Conn
	Accept(ctx context.Context, widGener *atomic.Uint64) (net.Conn, uint64, error)

	// Endpoint returns the endpoint URL for this listener
	Endpoint() (string, error)
}
