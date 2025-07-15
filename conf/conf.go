package conf

import (
	"runtime"
	"time"
)

type Config struct {
	Worker Worker
	Bucket Bucket

	WebSocket WebSocket
	TCP       TCP
	KCP       KCP
}

type TCP struct {
	KeepAlive    bool
	WriteBufSize int
	ReadBufSize  int
}

type KCP struct {
	WriteBufSize int
	ReadBufSize  int
	DataShards   int
	ParityShards int

	NoDelay    [4]int // nodelay, interval, resend, nc
	WindowSize [2]int // sndwnd, rcvwnd
	MTU        int    // MAX MTU for UDP networks
	ACKNoDelay bool   // ACK immediately
	WriteDelay bool   // immediate sending
	DSCP       int    // EF (Expedited Forwarding) for low latency

	Smux              bool
	SmuxStreamSize    int
	KeepAliveInterval time.Duration
	KeepAliveTimeout  time.Duration
	MaxFrameSize      int
	MaxReceiveBuffer  int
}

type Worker struct {
	WorkerSize         int
	ReplyChanSize      int
	HandshakeTimeout   time.Duration
	RequestIdleTimeout time.Duration
	StopTimeout        time.Duration
	TunnelGroupSize    int
	TickInterval       time.Duration
}

type Bucket struct {
	BucketSize int
}

type WebSocket struct {
	ReadBufSize  int
	WriteBufSize int
	AllowOrigins []string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func Default() Config {
	worker := Worker{
		WorkerSize:         runtime.NumCPU() * 2,
		ReplyChanSize:      1024,
		HandshakeTimeout:   time.Second * 10,
		RequestIdleTimeout: time.Second * 60,
		StopTimeout:        time.Second * 3,
		TunnelGroupSize:    32,
		TickInterval:       time.Second * 10,
	}

	bucket := Bucket{
		BucketSize: 256,
	}

	websocket := WebSocket{
		ReadBufSize:  8192,
		WriteBufSize: 8192,
		AllowOrigins: []string{"*"},
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
	}

	tcp := TCP{
		KeepAlive:    true,
		WriteBufSize: 8192,
		ReadBufSize:  8192,
	}

	kcp := KCP{
		WriteBufSize:      8192,
		ReadBufSize:       8192,
		DataShards:        10,
		ParityShards:      3,
		NoDelay:           [4]int{1, 10, 2, 1},
		WindowSize:        [2]int{128, 128},
		MTU:               1400,
		ACKNoDelay:        true,
		WriteDelay:        false,
		DSCP:              46,
		Smux:              true,
		SmuxStreamSize:    3,
		KeepAliveInterval: 10 * time.Second,
		KeepAliveTimeout:  30 * time.Second,
		MaxFrameSize:      4096,
		MaxReceiveBuffer:  4 * 1024 * 1024,
	}

	return Config{
		Worker:    worker,
		Bucket:    bucket,
		WebSocket: websocket,
		TCP:       tcp,
		KCP:       kcp,
	}
}

func MOBAConfig() Config {
	config := Default()

	config.KCP.WriteBufSize = 16384
	config.KCP.ReadBufSize = 16384
	config.KCP.DataShards = 8
	config.KCP.ParityShards = 2
	config.KCP.NoDelay = [4]int{1, 5, 2, 1}
	config.KCP.WindowSize = [2]int{256, 256}
	config.KCP.MTU = 1200
	config.KCP.SmuxStreamSize = 4
	config.KCP.KeepAliveInterval = 5 * time.Second
	config.KCP.KeepAliveTimeout = 15 * time.Second
	config.KCP.MaxFrameSize = 2048
	config.KCP.MaxReceiveBuffer = 8 * 1024 * 1024

	config.Worker.ReplyChanSize = 2048
	config.Worker.HandshakeTimeout = 5 * time.Second
	config.Worker.RequestIdleTimeout = 30 * time.Second
	config.Worker.StopTimeout = 5 * time.Second
	config.Worker.TunnelGroupSize = 64
	config.Worker.TickInterval = 5 * time.Second

	return config
}
