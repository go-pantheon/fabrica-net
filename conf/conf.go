package conf

import (
	"runtime"
	"time"
)

type Config struct {
	Server    Server
	Worker    Worker
	Bucket    Bucket
	WebSocket WebSocket
}

type Server struct {
	WorkerSize   int
	WriteBufSize int
	ReadBufSize  int
	KeepAlive    bool
}

type Worker struct {
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
	tcp := Server{
		WorkerSize:   runtime.NumCPU(),
		WriteBufSize: 8192,
		ReadBufSize:  8192,
		KeepAlive:    true,
	}

	protocol := Worker{
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

	return Config{
		Server:    tcp,
		Worker:    protocol,
		Bucket:    bucket,
		WebSocket: websocket,
	}
}
