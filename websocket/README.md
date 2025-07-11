# WebSocket Protocol Implementation for Fabrica-Net

This package provides a WebSocket implementation for the `fabrica-net` library, following the same architecture and patterns as the TCP implementation.

## Features

- **Protocol-agnostic design**: Reuses existing `internal.Worker` and codec infrastructure
- **WebSocket frame handling**: Supports binary message frames with length validation
- **Session management**: Full session lifecycle with authentication and encryption
- **Middleware support**: Read and write filters for request/response processing
- **Connection pooling**: Reuses existing ring buffer pool for efficient memory management
- **Graceful shutdown**: Proper cleanup of connections and resources

## Architecture

The WebSocket implementation follows the same patterns as the TCP implementation:

```
websocket/
├── server/           # WebSocket server implementation
│   ├── server.go     # Main server with HTTP upgrade handling
│   └── server_test.go
├── client/           # WebSocket client implementation
│   └── client.go     # WebSocket client with dial and messaging
└── README.md

internal/
├── websocket_conn.go # Connection adapter for WebSocket
└── codec/
    └── websocket.go  # WebSocket-specific codec functions
```

## Usage

### Server

```go
package main

import (
    "context"
    "github.com/go-kratos/kratos/v2/log"
    websocketserver "github.com/go-pantheon/fabrica-net/websocket/server"
)

func main() {
    logger := log.NewStdLogger(os.Stdout)
    service := NewMyService(logger) // implements xnet.Service

    server, err := websocketserver.NewServer(":8080", "/ws", service,
        websocketserver.Logger(logger),
    )
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    if err := server.Start(ctx); err != nil {
        log.Fatal(err)
    }

    // Server is now listening on ws://localhost:8080/ws
}
```

### Client

```go
package main

import (
    "context"
    websocketclient "github.com/go-pantheon/fabrica-net/websocket/client"
)

func main() {
    handshakePack := []byte("auth:user123")

    client := websocketclient.NewClient(1, "ws://localhost:8080/ws", handshakePack)

    ctx := context.Background()
    if err := client.Start(ctx); err != nil {
        log.Fatal(err)
    }

    // Send message
    if err := client.Send([]byte("hello")); err != nil {
        log.Error(err)
    }

    // Receive messages
    for pack := range client.Receive() {
        log.Infof("Received: %s", string(pack))
    }
}
```

## Key Components

### WebSocket Server

- **HTTP Upgrade**: Handles WebSocket upgrade requests on specified path
- **Worker Integration**: Creates `internal.Worker` instances with WebSocket connection adapters
- **Lifecycle Management**: Proper start/stop with graceful shutdown
- **Broadcasting**: Supports Push, Multicast, and Broadcast operations

### WebSocket Client

- **Dial Support**: Connects to WebSocket servers with configurable dialer
- **Authentication**: Handles handshake with server-side authentication
- **Message Handling**: Async receive loop with channel-based message delivery

### Connection Adapter

- **net.Conn Interface**: Adapts WebSocket connections to standard net.Conn interface
- **Deadline Support**: Implements read/write deadline functionality
- **Address Information**: Provides local and remote address information

### Codec Integration

- **Frame Handling**: Encodes/decodes WebSocket binary frames
- **Length Validation**: Enforces maximum packet size limits
- **Buffer Pooling**: Reuses ring buffer pool for efficient memory management

## Configuration

The WebSocket implementation supports all the same configuration options as TCP:

```go
server, err := websocketserver.NewServer(":8080", "/ws", service,
    websocketserver.WithConf(customConfig),
    websocketserver.Logger(logger),
    websocketserver.ReadFilter(authMiddleware),
    websocketserver.WriteFilter(loggingMiddleware),
    websocketserver.AfterConnectFunc(onConnect),
    websocketserver.AfterDisconnectFunc(onDisconnect),
    websocketserver.WithUpgrader(customUpgrader),
)
```

## Examples

See the `example/websocket/` directory for complete server and client examples with message handling.

## Testing

Run the test suite:

```bash
go test ./websocket/...
```

## Protocol Compatibility

This WebSocket implementation maintains full compatibility with the existing `xnet` interfaces:

- `xnet.Server` - Server interface implementation
- `xnet.Client` - Client interface implementation
- `xnet.Service` - Service interface for business logic
- `xnet.Worker` - Worker interface for connection handling
- `xnet.Session` - Session management and metadata
- `xnet.Pack` - Binary packet format

The implementation seamlessly integrates with existing TCP services and can be used as a drop-in replacement for TCP transport.
