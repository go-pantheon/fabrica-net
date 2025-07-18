package internal

import "context"

// Dialer defines the interface for protocol-specific connection dialing
type Dialer interface {
	// Dial establishes a connection to the target
	Dial(ctx context.Context, target string) (conns []ConnCarrier, err error)

	// Stop stops the dialer
	Stop(ctx context.Context) error

	// Target returns the target address/URL for this dialer
	Target() string
}
