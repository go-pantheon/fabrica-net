package xnet

import (
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc/metadata"
)

type NetKind string

const (
	// NetKindTCP is the kind of the TCP transport.
	NetKindTCP NetKind = "tcp"
	// NetKindKCP is the kind of the KCP transport.
	NetKindKCP NetKind = "kcp"
	// NetKindWebSocket is the kind of WebSocket transport.
	NetKindWebSocket NetKind = "ws"
)

var _ transport.Transporter = (*Transport)(nil)

// Transport is a custom transport implement of transport.Transporter in kratos.
type Transport struct {
	endpoint      string
	operation     string
	requestHeader HeaderCarrier
	replyHeader   HeaderCarrier
}

// NewTransport creates a new transport.
//
// endpoint: the endpoint of the transport.
// operation: the operation of the transport.
// requestHeader: the request header of the transport.
// replyHeader: the reply header of the transport.
func NewTransport(endpoint string,
	operation string,
	requestHeader HeaderCarrier,
	replyHeader HeaderCarrier,
) *Transport {
	return &Transport{
		endpoint:      endpoint,
		operation:     operation,
		requestHeader: requestHeader,
		replyHeader:   replyHeader,
	}
}

// Kind returns the kind of the transport.
func (tr *Transport) Kind() transport.Kind {
	return transport.KindGRPC
}

// Endpoint returns the endpoint.
func (tr *Transport) Endpoint() string {
	return tr.endpoint
}

// Operation returns the operation.
func (tr *Transport) Operation() string {
	return tr.operation
}

// RequestHeader returns the request header.
func (tr *Transport) RequestHeader() transport.Header {
	return tr.requestHeader
}

// ReplyHeader returns the reply header.
func (tr *Transport) ReplyHeader() transport.Header {
	return tr.replyHeader
}

// HeaderCarrier is a wrapper around metadata.MD.
type HeaderCarrier metadata.MD

// Get returns the first value associated with the given key.
// If there are no values associated with the key, Get returns "".
func (mc HeaderCarrier) Get(key string) string {
	vals := metadata.MD(mc).Get(key)
	if len(vals) > 0 {
		return vals[0]
	}

	return ""
}

// Set sets the value associated with key to value.
func (mc HeaderCarrier) Set(key string, value string) {
	metadata.MD(mc).Set(key, value)
}

// Keys returns all keys present in this metadata.
func (mc HeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(mc))
	for k := range metadata.MD(mc) {
		keys = append(keys, k)
	}

	return keys
}

// Add appends the value to the existing values for the given key.
func (mc HeaderCarrier) Add(key string, value string) {
	metadata.MD(mc).Append(key, value)
}

// Values returns all values associated with the key.
func (mc HeaderCarrier) Values(key string) []string {
	return metadata.MD(mc).Get(key)
}
