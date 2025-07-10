package conf

import (
	"runtime"
	"time"
)

type Config struct {
	Server Server
	Worker Worker
	Bucket Bucket
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

	return Config{
		Server: tcp,
		Worker: protocol,
		Bucket: bucket,
	}
}
