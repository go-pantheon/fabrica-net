<div align="center">
  <h1>🏛️ FABRICA NET</h1>
  <p><em>High-performance multi-protocol network library for the go-pantheon ecosystem</em></p>
</div>

<p align="center">
<a href="https://github.com/go-pantheon/fabrica-net/actions/workflows/test.yml"><img src="https://github.com/go-pantheon/fabrica-net/workflows/Test/badge.svg" alt="Test Status"></a>
<a href="https://github.com/go-pantheon/fabrica-net/releases"><img src="https://img.shields.io/github/v/release/go-pantheon/fabrica-net" alt="Latest Release"></a>
<a href="https://pkg.go.dev/github.com/go-pantheon/fabrica-net"><img src="https://pkg.go.dev/badge/github.com/go-pantheon/fabrica-net" alt="GoDoc"></a>
<a href="https://goreportcard.com/report/github.com/go-pantheon/fabrica-net"><img src="https://goreportcard.com/badge/github.com/go-pantheon/fabrica-net" alt="Go Report Card"></a>
<a href="https://github.com/go-pantheon/fabrica-net/blob/main/LICENSE"><img src="https://img.shields.io/github/license/go-pantheon/fabrica-net" alt="License"></a>
<a href="https://deepwiki.com/go-pantheon/fabrica-net"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</p>

> **Language**: [English](README.md) | [中文](README_CN.md)

## About Fabrica Net

Fabrica Net is a high-performance, enterprise-grade network library designed specifically for the [go-pantheon/janus](https://github.com/go-pantheon/janus) gateway service. It provides secure, multi-protocol communication capabilities with advanced session management and real-time monitoring for game server infrastructure.

For more information, please check out: [deepwiki/go-pantheon/fabrica-net](https://deepwiki.com/go-pantheon/fabrica-net)

## About go-pantheon Ecosystem

**go-pantheon** is an out-of-the-box game server framework providing high-performance, highly available game server cluster solutions based on microservices architecture using [go-kratos](https://github.com/go-kratos/kratos). Fabrica Net serves as the network communication foundation that supports the core components:

- **Roma**: Game core logic services
- **Janus**: Gateway service for client connection handling and request forwarding
- **Lares**: Account service for user authentication and account management
- **Senate**: Backend management service providing operational interfaces

### Core Features

- 🌐 **Multi-Protocol Support**: TCP, KCP, and WebSocket with unified API
- 🔒 **Enterprise Security**: ECDH key exchange with AES-GCM encryption
- ⚡ **High Performance**: Worker pool architecture with zero-copy operations
- 📊 **Monitoring & Observability**: Prometheus metrics and distributed tracing
- 🔧 **Session Management**: Comprehensive user session lifecycle management
- 🛡️ **Connection Management**: Auto-reconnection with heartbeat detection
- 🔄 **Graceful Shutdown**: Timeout-controlled shutdown with connection draining
- 🎯 **Load Balancing**: Weight-based routing with health checks

## Network Protocols

### TCP Server (`tcp/server/`)
High-performance TCP server with connection pooling:
- Multi-worker architecture for concurrent connection handling
- Configurable buffer sizes and keep-alive settings
- Connection lifecycle management with hooks
- Middleware support for request/response filtering

### TCP Client (`tcp/client/`)
Robust TCP client with auto-reconnection:
- Encrypted communication with session management
- Retry mechanisms with exponential backoff
- Connection state tracking and recovery

### Network Abstractions (`xnet/`)
Core network abstractions and utilities:
- **Session**: User session with encryption and state management
- **Transport**: Multi-protocol transport layer implementation
- **Cryptor**: AES-GCM encryption/decryption interface
- **ECDHable**: Elliptic Curve Diffie-Hellman key exchange

## Technology Stack

| Technology/Component | Purpose                      | Version |
| -------------------- | ---------------------------- | ------- |
| Go                   | Primary development language | 1.23+   |
| go-kratos            | Microservices framework      | v2.8.4  |
| fabrica-util         | Common utilities library     | v0.0.18 |
| Prometheus           | Metrics and monitoring       | v1.22.0 |
| gRPC                 | Inter-service communication  | v1.73.0 |
| golang.org/x/crypto  | Cryptographic operations     | v0.39.0 |

## Requirements

- Go 1.23+

## Quick Start

### Installation

```bash
go get github.com/go-pantheon/fabrica-net
```

### Initialize Development Environment

```bash
make init
```

### Run Tests

```bash
make test
```

## Usage Examples

### Basic TCP Server

```go
package main

import (
    "context"
    "log"

    "github.com/go-pantheon/fabrica-net/tcp/server"
    "github.com/go-pantheon/fabrica-net/xnet"
)

type GameService struct{}

func (s *GameService) Handle(ctx context.Context, session xnet.Session, data []byte) error {
    log.Printf("Received from user %d: %s", session.UID(), string(data))
    return nil
}

func main() {
    service := &GameService{}

    srv, err := server.NewServer(service,
        server.Bind(":8080"),
        server.Logger(log.Default()),
    )
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    if err := srv.Start(ctx); err != nil {
        log.Fatal(err)
    }

    select {} // Keep running
}
```

### TCP Client Connection

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/go-pantheon/fabrica-net/tcp/client"
)

func main() {
    // Create TCP client with ID and bind address
    client := client.NewClient(12345, client.Bind("localhost:8080"))

    // Start connection
    ctx := context.Background()
    if err := client.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Send message
    message := []byte("Hello Server!")
    if err := client.Send(message); err != nil {
        log.Fatal(err)
    }

    // Receive messages
    go func() {
        for data := range client.Receive() {
            log.Printf("Received: %s", string(data))
        }
    }()

    // Keep client running
    time.Sleep(time.Second * 5)
}
```

### Session Management with Encryption

```go
package main

import (
    "log"

    "github.com/go-pantheon/fabrica-net/xnet"
)

func main() {
    // Create encrypted session
    key := []byte("0123456789abcdef0123456789abcdef")
    cryptor, err := xnet.NewCryptor(key)
    if err != nil {
        log.Fatal(err)
    }

    session := xnet.NewSession(12345, 1, time.Now().Unix(), cryptor, xnet.NewUnECDH(), "game", 1)

    // Encrypt data
    data := []byte("sensitive game data")
    encrypted, err := session.Encrypt(data)
    if err != nil {
        log.Fatal(err)
    }

    // Decrypt data
    decrypted, err := session.Decrypt(encrypted)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Original: %s, Decrypted: %s", data, decrypted)
}
```

### Configuration Setup

```go
package main

import (
    "time"

    "github.com/go-pantheon/fabrica-net/conf"
)

func main() {
    config := conf.Config{
        Server: conf.Server{
            WorkerSize:   8,
            Bind:         ":7000",
            WriteBufSize: 30000,
            ReadBufSize:  30000,
            KeepAlive:    true,
            StopTimeout:  time.Second * 30,
        },
        Worker: conf.Worker{
            ReaderBufSize:         8192,
            ReplyChanSize:         1024,
            HandshakeTimeout:      time.Second * 10,
            RequestIdleTimeout:    time.Second * 60,
            StopTimeout:           time.Second * 3,
            TunnelGroupSize:       32,
            TickInterval:          time.Second * 10,
        },
    }

    // Use configuration...
}
```

## Project Structure

```
.
├── tcp/                # TCP protocol implementation
│   ├── server/         # TCP server with worker pool
│   └── client/         # TCP client with auto-reconnection
├── xnet/               # Core network abstractions
│   ├── session.go      # Session management
│   ├── transport.go    # Transport layer
│   ├── crypto.go       # AES-GCM encryption
│   └── ecdh.go         # ECDH key exchange
├── xcontext/           # Context utilities
├── http/               # HTTP utilities
│   └── health/         # Health check endpoints
├── middleware/         # Middleware components
├── internal/           # Internal implementations
│   ├── manager.go      # Worker manager
│   ├── worker.go       # Connection worker
│   └── tunnel.go       # Communication tunnel
└── conf/               # Configuration management
```

## Integration with go-pantheon Components

Fabrica Net is designed to be imported by other go-pantheon components:

```go
import (
    // TCP server for Janus gateway
    "github.com/go-pantheon/fabrica-net/tcp/server"

    // Session management for user connections
    "github.com/go-pantheon/fabrica-net/xnet"

    // Health checks for load balancers
    "github.com/go-pantheon/fabrica-net/http/health"

    // Configuration management
    "github.com/go-pantheon/fabrica-net/conf"
)
```

## Development Guide

### Environment Configuration

Configure fabrica-net through environment variables:

```bash
export REGION="us-west-1"           # Deployment region
export ZONE="us-west-1a"            # Availability zone
export DEPLOY_ENV="production"      # Environment (dev/staging/prod)
export ADDRS="10.0.1.100"          # Server public IP addresses
export WEIGHT="100"                 # Load balancing weight
export OFFLINE="false"              # Offline mode flag
```

### Testing

Run the complete test suite:

```bash
# Run all tests with coverage
make test

# Run benchmarks
make benchmark

# Run linting
make lint
```

### Adding New Protocols

When adding new network protocols:

1. Create a new package under the protocol name (e.g., `kcp/`, `websocket/`)
2. Implement server and client components
3. Follow the existing TCP implementation patterns
4. Add comprehensive unit tests and benchmarks
5. Update documentation with usage examples
6. Ensure compatibility with existing `xnet` abstractions

### Contribution Guidelines

1. Fork this repository
2. Create a feature branch from `main`
3. Implement changes with comprehensive tests
4. Ensure all tests pass and linting is clean
5. Update documentation for any API changes
6. Submit a Pull Request with clear description

## Performance Considerations

- **Connection Pooling**: TCP servers use worker pools for optimal connection handling
- **Memory Management**: Zero-copy operations where possible to reduce GC pressure
- **Encryption**: AES-GCM operations are optimized for high throughput
- **Session Management**: Session state is cached to minimize lookup overhead
- **Buffer Management**: Configurable buffer sizes for different workload patterns
- **Graceful Shutdown**: Connection draining prevents data loss during restarts

## License

This project is licensed under the terms specified in the LICENSE file.
