package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/example/tcp/message"
	tcp "github.com/go-pantheon/fabrica-net/tcp/client"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

func main() {
	cli := tcp.NewClient(1, tcp.Bind("127.0.0.1:17101"))

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

	log.Infof("client started")

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		<-ctx.Done()
		return ctx.Err()
	})

	eg.Go(func() error {
		authMsg := message.NewPacket(message.ModAuth, 0, 1, 0, []byte("Hi!"), 0)

		authPack, err := json.Marshal(authMsg)
		if err != nil {
			return err
		}

		if err := cli.Send(authPack); err != nil {
			return err
		}

		log.Infof("[send] auth %s", authMsg)

		authRecvPack := <-cli.Receive()

		authRecvMsg := &message.Packet{}
		if err := json.Unmarshal(authRecvPack, authRecvMsg); err != nil {
			return err
		}

		log.Infof("[recv] auth %s", authRecvMsg)

		for i := range 10 {
			msg := message.NewPacket(message.ModEcho, 0, 1, int32(i), []byte("Hello Alice!"), 0)

			pack, err := json.Marshal(msg)
			if err != nil {
				return err
			}

			if err := cli.Send(pack); err != nil {
				return err
			}

			log.Infof("[send] echo %s", msg)
			time.Sleep(time.Second * 1)
		}

		return nil
	})

	eg.Go(func() error {
		for pack := range cli.Receive() {
			msg := &message.Packet{}
			if err := json.Unmarshal(pack, msg); err != nil {
				return err
			}

			log.Infof("[recv] echo %s", msg)
		}

		return nil
	})

	c := make(chan os.Signal, 1)

	eg.Go(func() error {
		signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		<-c

		return xsync.ErrSignalStop
	})

	if err := eg.Wait(); err != nil &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, xsync.ErrSignalStop) {
		log.Errorf("stop client failed. %+v", err)
	} else {
		log.Infof("client stopped")
	}
}
