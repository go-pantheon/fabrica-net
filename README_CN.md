<div align="center">
  <h1>🏛️ FABRICA NET</h1>
  <p><em>面向 go-pantheon 生态系统的高性能多协议网络库</em></p>
</div>

<p align="center">
<a href="https://github.com/go-pantheon/fabrica-net/actions/workflows/test.yml"><img src="https://github.com/go-pantheon/fabrica-net/workflows/Test/badge.svg" alt="Test Status"></a>
<a href="https://github.com/go-pantheon/fabrica-net/releases"><img src="https://img.shields.io/github/v/release/go-pantheon/fabrica-net" alt="Latest Release"></a>
<a href="https://pkg.go.dev/github.com/go-pantheon/fabrica-net"><img src="https://pkg.go.dev/badge/github.com/go-pantheon/fabrica-net" alt="GoDoc"></a>
<a href="https://goreportcard.com/report/github.com/go-pantheon/fabrica-net"><img src="https://goreportcard.com/badge/github.com/go-pantheon/fabrica-net" alt="Go Report Card"></a>
<a href="https://github.com/go-pantheon/fabrica-net/blob/main/LICENSE"><img src="https://img.shields.io/github/license/go-pantheon/fabrica-net" alt="License"></a>
<a href="https://deepwiki.com/go-pantheon/fabrica-net"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</p>

> **语言**: [English](README.md) | [中文](README_CN.md)

## 关于 Fabrica Net

Fabrica Net 是一个高性能、企业级网络库，专门为 [go-pantheon/janus](https://github.com/go-pantheon/janus) 网关服务设计。它为游戏服务器基础设施提供安全的多协议通信能力、先进的会话管理和实时监控功能。

更多信息请访问：[deepwiki/go-pantheon/fabrica-net](https://deepwiki.com/go-pantheon/fabrica-net)

## 关于 go-pantheon 生态系统

**go-pantheon** 是一个开箱即用的游戏服务器框架，基于 [go-kratos](https://github.com/go-kratos/kratos) 微服务架构提供高性能、高可用的游戏服务器集群解决方案。Fabrica Net 作为网络通信基础，支撑以下核心组件：

- **Roma**: 游戏核心逻辑服务
- **Janus**: 客户端连接处理和请求转发的网关服务
- **Lares**: 用户认证和账户管理的账号服务
- **Senate**: 提供运营接口的后台管理服务

### 核心特性

- 🌐 **多协议支持**: TCP、KCP 和 WebSocket 统一 API
- 🔒 **企业级安全**: ECDH 密钥交换与 AES-GCM 加密
- ⚡ **高性能**: 工作池架构与零拷贝操作
- 📊 **监控与可观测性**: Prometheus 指标和分布式链路追踪
- 🔧 **会话管理**: 全面的用户会话生命周期管理
- 🛡️ **连接管理**: 自动重连与心跳检测
- 🔄 **优雅关闭**: 带连接排空的超时控制关闭
- 🎯 **负载均衡**: 基于权重的路由与健康检查

## 网络协议

### TCP 服务器 (`tcp/server/`)
高性能 TCP 服务器，支持连接池：
- 多工作器架构处理并发连接
- 可配置的缓冲区大小和保活设置
- 带钩子的连接生命周期管理
- 支持请求/响应过滤的中间件

### TCP 客户端 (`tcp/client/`)
健壮的 TCP 客户端，支持自动重连：
- 带会话管理的加密通信
- 指数退避的重试机制
- 连接状态跟踪和恢复

### 网络抽象 (`xnet/`)
核心网络抽象和工具：
- **Session**: 带加密和状态管理的用户会话
- **Transport**: 多协议传输层实现
- **Cryptor**: AES-GCM 加密/解密接口
- **ECDHable**: 椭圆曲线 Diffie-Hellman 密钥交换

## 技术栈

| 技术/组件           | 用途         | 版本    |
| ------------------- | ------------ | ------- |
| Go                  | 主要开发语言 | 1.23+   |
| go-kratos           | 微服务框架   | v2.8.4  |
| fabrica-util        | 通用工具库   | v0.0.20 |
| Prometheus          | 指标和监控   | v1.22.0 |
| gRPC                | 服务间通信   | v1.73.0 |
| golang.org/x/crypto | 加密操作     | v0.39.0 |

## 系统要求

- Go 1.23+

## 快速开始

### 安装

```bash
go get github.com/go-pantheon/fabrica-net
```

### 初始化开发环境

```bash
make init
```

### 运行测试

```bash
make test
```

## 使用示例

### 基础 TCP 服务器

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    tcp "github.com/go-pantheon/fabrica-net/tcp/server"
    "github.com/go-pantheon/fabrica-net/xnet"
)

type GameService struct{}

// Auth 处理客户端认证
func (s *GameService) Auth(ctx context.Context, in xnet.Pack) (out xnet.Pack, ss xnet.Session, err error) {
    // 认证逻辑
    userID := int64(12345) // 从认证数据中提取
    ss = xnet.NewSession(userID, "game", 1)

    return []byte("认证成功"), ss, nil
}

// Handle 处理客户端消息
func (s *GameService) Handle(ctx context.Context, ss xnet.Session, tm xnet.TunnelManager, in xnet.Pack) error {
    log.Printf("从用户 %d 收到消息: %s", ss.UID(), string(in))
    return nil
}

// 其他必需的 Service 接口方法...
func (s *GameService) TunnelType(mod int32) (int32, int, error) { return 1, 1, nil }
func (s *GameService) CreateAppTunnel(ctx context.Context, ss xnet.Session, tp int32, rid int64, w xnet.Worker) (xnet.AppTunnel, error) { return nil, nil }
func (s *GameService) OnConnected(ctx context.Context, ss xnet.Session) error { return nil }
func (s *GameService) OnDisconnect(ctx context.Context, ss xnet.Session) error { return nil }
func (s *GameService) Tick(ctx context.Context, ss xnet.Session) error { return nil }

func main() {
    service := &GameService{}

    srv, err := tcp.NewServer(":8080", service)
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := srv.Start(ctx); err != nil {
        log.Fatal(err)
    }

    defer func() {
        if err := srv.Stop(ctx); err != nil {
            log.Printf("停止服务器失败: %+v", err)
        }
    }()

    // 等待中断信号
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
    <-c

    log.Printf("服务器已停止")
}
```

### TCP 客户端连接

```go
package main

import (
    "context"
    "log"
    "time"

    tcp "github.com/go-pantheon/fabrica-net/tcp/client"
    "github.com/go-pantheon/fabrica-net/xnet"
)

func main() {
    // 创建带 ID 和绑定地址的 TCP 客户端
    client := tcp.NewClient(12345, tcp.Bind("localhost:8080"))

    // 启动连接
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := client.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer func() {
        if err := client.Stop(ctx); err != nil {
            log.Printf("停止客户端失败: %+v", err)
        }
    }()

    // 发送消息
    message := xnet.Pack([]byte("你好服务器!"))
    if err := client.Send(message); err != nil {
        log.Fatal(err)
    }

    // 接收消息
    go func() {
        for data := range client.Receive() {
            log.Printf("收到消息: %s", string(data))
        }
    }()

    // 保持客户端运行
    time.Sleep(time.Second * 5)
}
```

### 会话管理与加密

```go
package main

import (
    "log"
    "time"

    "github.com/go-pantheon/fabrica-net/xnet"
)

func main() {
    // 使用选项创建加密会话
    key := []byte("0123456789abcdef0123456789abcdef")
    cryptor, err := xnet.NewCryptor(key)
    if err != nil {
        log.Fatal(err)
    }

    session := xnet.NewSession(12345, "game", 1,
        xnet.WithEncryptor(cryptor),
        xnet.WithSID(1),
        xnet.WithStartTime(time.Now().Unix()),
    )

    // 加密数据
    data := xnet.Pack([]byte("敏感游戏数据"))
    encrypted, err := session.Encrypt(data)
    if err != nil {
        log.Fatal(err)
    }

    // 解密数据
    decrypted, err := session.Decrypt(encrypted)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("原始数据: %s, 解密后: %s", data, decrypted)
}
```

### 配置设置

```go
package main

import (
    "runtime"
    "time"

    "github.com/go-pantheon/fabrica-net/conf"
)

func main() {
    config := conf.Config{
        Server: conf.Server{
            WorkerSize:   runtime.NumCPU(),
            WriteBufSize: 30000,
            ReadBufSize:  30000,
            KeepAlive:    true,
        },
        Worker: conf.Worker{
            ReaderBufSize:         8192,
            ReplyChanSize:         1024,
            HandshakeTimeout:      time.Second * 10,
            RequestIdleTimeout:    time.Second * 60,
            WaitMainTunnelTimeout: time.Second * 30,
            StopTimeout:           time.Second * 3,
            TunnelGroupSize:       32,
            TickInterval:          time.Second * 10,
        },
        Bucket: conf.Bucket{
            BucketSize: 128,
        },
    }

    // 通过服务器选项使用配置
    // srv, err := tcp.NewServer(":8080", service, tcp.WithConf(config))
}
```

## 项目结构

```
.
├── tcp/                # TCP 协议实现
│   ├── server/         # 带工作池的 TCP 服务器
│   └── client/         # 带自动重连的 TCP 客户端
├── xnet/               # 核心网络抽象
│   ├── session.go      # 会话管理
│   ├── transport.go    # 传输层
│   ├── crypto.go       # AES-GCM 加密
│   ├── ecdh.go         # ECDH 密钥交换
│   ├── service.go      # 服务接口
│   ├── tunnel.go       # 隧道管理
│   └── worker.go       # 工作器接口
├── tunnel/             # 隧道实现
├── xcontext/           # 上下文工具
├── http/               # HTTP 工具
│   └── health/         # 健康检查端点
├── middleware/         # 中间件组件
├── internal/           # 内部实现
│   ├── workermanager.go    # 工作器管理器
│   ├── tunnelmanager.go    # 隧道管理器
│   ├── worker.go           # 连接工作器
│   ├── bufpool/            # 缓冲池工具
│   ├── codec/              # 消息编码/解码
│   └── ip/                 # IP 工具
├── conf/               # 配置管理
│   └── conf.go         # 配置结构
└── example/            # 示例应用
    └── tcp/            # TCP 客户端/服务器示例
```

## 与 go-pantheon 组件集成

Fabrica Net 专为其他 go-pantheon 组件导入而设计：

```go
import (
    // Janus 网关的 TCP 服务器
    tcp "github.com/go-pantheon/fabrica-net/tcp/server"

    // 用户连接的会话管理
    "github.com/go-pantheon/fabrica-net/xnet"

    // 负载均衡器的健康检查
    "github.com/go-pantheon/fabrica-net/http/health"

    // 配置管理
    "github.com/go-pantheon/fabrica-net/conf"
)
```

## 开发指南

### 环境配置

通过环境变量配置 fabrica-net：

```bash
export REGION="us-west-1"           # 部署区域
export ZONE="us-west-1a"            # 可用区
export DEPLOY_ENV="production"      # 环境 (dev/staging/prod)
export ADDRS="10.0.1.100"          # 服务器公网 IP 地址
export WEIGHT="100"                 # 负载均衡权重
export OFFLINE="false"              # 离线模式标志
```

### 测试

运行完整的测试套件：

```bash
# 运行所有测试并生成覆盖率报告
make test

# 运行基准测试
make benchmark

# 运行代码检查
make lint
```

### 运行示例

项目在 `example/` 目录中包含全面的示例：

```bash
# 构建并运行 TCP 服务器示例
cd example/tcp
make build-server
./bin/server

# 构建并运行 TCP 客户端示例
make build-client
./bin/client
```

### 添加新协议

添加新网络协议时：

1. 在协议名称下创建新包（如 `kcp/`、`websocket/`）
2. 实现服务器和客户端组件
3. 遵循现有 TCP 实现模式
4. 添加全面的单元测试和基准测试
5. 使用示例更新文档
6. 确保与现有 `xnet` 抽象的兼容性

### 贡献指南

1. Fork 此仓库
2. 从 `main` 分支创建功能分支
3. 实现更改并编写全面的测试
4. 确保所有测试通过且代码检查无误
5. 更新任何 API 更改的文档
6. 提交带有清晰描述的 Pull Request

## 性能考虑

- **连接池**: TCP 服务器使用工作池优化连接处理
- **内存管理**: 尽可能使用零拷贝操作减少 GC 压力
- **加密**: AES-GCM 操作针对高吞吐量进行优化
- **会话管理**: 会话状态被缓存以最小化查找开销
- **缓冲区管理**: 针对不同工作负载模式的可配置缓冲区大小
- **优雅关闭**: 连接排空防止重启期间的数据丢失

## 许可证

本项目基于 LICENSE 文件中指定的条款授权。
