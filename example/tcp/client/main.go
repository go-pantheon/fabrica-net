package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/example/message"
	tcp "github.com/go-pantheon/fabrica-net/tcp/client"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

var ErrSendFinished = errors.New("send finished")

func main() {
	cli := tcp.NewClient(1, "127.0.0.1:17101", handshakePack, client.WithAuthFunc(authFunc))

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
		!errors.Is(err, xsync.ErrSignalStop) {
		log.Errorf("stop client failed. %+v", err)
	} else {
		log.Infof("client stopped")
	}
}

func handshakePack(connID int64) (xnet.Pack, error) {
	authMsg := message.NewPacket(message.ModAuth, 0, 1, 0, fmt.Appendf(nil, "Hi! %d", connID), 0)

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

	log.Infof("[recv] auth %s", authMsg)

	return xnet.DefaultSession(), nil
}

func sendEcho(cli *tcp.Client) error {
	for i := range 10 {
		msg := message.NewPacket(message.ModEcho, 0, 1, int32(i), []byte("Hello Alice!"), 0)

		pack, err := json.Marshal(msg)
		if err != nil {
			return errors.Wrap(err, "marshal message failed")
		}

		if err := cli.Send(pack); err != nil {
			return err
		}

		log.Infof("[SEND] echo %s", msg)
		time.Sleep(time.Second * 1)
	}

	return ErrSendFinished
}

func recvEcho(cli *tcp.Client) error {
	for pack := range cli.Receive() {
		msg := &message.Packet{}
		if err := json.Unmarshal(pack, msg); err != nil {
			return errors.Wrap(err, "unmarshal echo recv pack failed")
		}

		log.Infof("[RECV] echo %s", msg)
	}

	return nil
}
