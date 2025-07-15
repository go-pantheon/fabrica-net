package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/example/message"
	kcp "github.com/go-pantheon/fabrica-net/kcp/client"
	"github.com/go-pantheon/fabrica-net/kcp/frame"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

var ErrSendFinished = errors.New("send finished")

var (
	config = conf.MOBAConfig()
)

func main() {
	config.KCP.Smux = false

	if err := frame.InitMOBARingPool(); err != nil {
		log.Errorf("failed to initialize MOBA ring pool: %+v", err)
	}

	handshakePack, err := handshakePack()
	if err != nil {
		panic(err)
	}

	cli, err := kcp.NewClient(1, "127.0.0.1:17201", handshakePack,
		client.WithAuthFunc(authFunc),
		client.WithConf(config),
	)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cli.Start(ctx); err != nil {
		panic(err)
	}

	defer func() {
		if err := cli.Stop(ctx); err != nil {
			log.Errorf("stop client failed. %+v", err)
		}
	}()

	log.Infof("KCP client connected with gaming-optimized configuration")

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return sendEcho(cli)
		}
	})
	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return recvEcho(cli)
		}
	})
	eg.Go(func() error {
		return monitor(ctx)
	})

	c := make(chan os.Signal, 1)

	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
			<-c

			return xsync.ErrSignalStop
		}
	})

	if err := eg.Wait(); err != nil &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, xsync.ErrSignalStop) &&
		!errors.Is(err, ErrSendFinished) {
		log.Errorf("client error: %+v", err)
	} else {
		log.Infof("KCP client stopped")
	}
}

func handshakePack() (xnet.Pack, error) {
	authMsg := message.NewPacket(message.ModAuth, 0, 1, 0, []byte("Hi from KCP client!"), 0)

	authPack, err := json.Marshal(authMsg)
	if err != nil {
		return nil, errors.Wrap(err, "marshal auth pack failed")
	}

	return authPack, nil
}

func authFunc(ctx context.Context, pack xnet.Pack) (xnet.Session, error) {
	authMsg := &message.Packet{}
	if err := json.Unmarshal(pack, authMsg); err != nil {
		return nil, errors.Wrap(err, "unmarshal auth pack failed")
	}

	log.Infof("[RECV] auth %s", authMsg)

	return xnet.DefaultSession(), nil
}

func sendEcho(cli *kcp.Client) error {
	streamSize := config.KCP.SmuxStreamSize

	for i := range 20 {
		msg := message.NewPacket(message.ModEcho, 0, 1, int32(i),
			[]byte("Hello from KCP! This is a gaming-optimized message for low latency communication."), 0)

		if config.KCP.Smux {
			msg.StreamID = int32(i % streamSize)
		}

		pack, err := json.Marshal(msg)
		if err != nil {
			return errors.Wrap(err, "marshal message failed")
		}

		if err := cli.Send(pack); err != nil {
			return err
		}

		log.Infof("[SEND] %d echo %s", msg.StreamID, msg)

		time.Sleep(500 * time.Millisecond)
	}

	return ErrSendFinished
}

func recvEcho(cli *kcp.Client) error {
	for pack := range cli.Receive() {
		msg := &message.Packet{}
		if err := json.Unmarshal(pack, msg); err != nil {
			return errors.Wrap(err, "unmarshal echo recv pack failed")
		}

		log.Infof("[RECV] %d echo %s", msg.StreamID, msg)
	}

	return nil
}

func monitor(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			codecStats := frame.GetCodecStats()
			poolStats := frame.GetPoolStats()

			log.Infof("KCP Performance - Encodes: %d, Decodes: %d, Hit Ratio: %.2f%%, Avg Packet: %.1f bytes",
				codecStats.TotalEncodes,
				codecStats.TotalDecodes,
				codecStats.HitRatio*100,
				codecStats.AvgPacketSize)

			if len(poolStats) > 0 {
				totalAllocs := uint64(0)
				for _, stats := range poolStats {
					totalAllocs += stats.AllocCount
				}
				if totalAllocs > 0 {
					log.Debugf("Pool total allocations: %d", totalAllocs)
				}
			}
		}
	}
}
