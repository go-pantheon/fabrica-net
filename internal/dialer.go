package internal

import (
	"context"
	"net"

	"github.com/go-pantheon/fabrica-net/codec"
)

// Dialer defines the interface for protocol-specific connection dialing
type Dialer interface {
	// Dial establishes a connection to the target
	Dial(ctx context.Context, target string) (net.Conn, codec.Codec, error)

	// Target returns the target address/URL for this dialer
	Target() string
}
