package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
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

	cli.WalkDialogs(func(dialog xnet.ClientDialog) {
		eg.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return recvEcho(dialog)
			}
		})
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

func handshakePack(wid uint64) (xnet.Pack, error) {
	authMsg := message.NewPacket(message.ModAuth, 0, 1, 0, fmt.Appendf(nil, "Hi! %d", wid), 0)

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

	return xnet.DefaultSession(), nil
}

func sendEcho(cli *tcp.Client) error {
	var wg sync.WaitGroup

	cli.WalkDialogs(func(dialog xnet.ClientDialog) {
		dialog.WaitAuthed()
		wg.Add(1)

		xsync.Go(fmt.Sprintf("sendEcho-%d", dialog.ID()), func() error {
			defer wg.Done()

			for i := range 10 {
				msg := message.NewPacket(message.ModEcho, 0, 1, int32(i), []byte("Hello Alice!"), 0)

				pack, err := json.Marshal(msg)
				if err != nil {
					return errors.Wrap(err, "marshal message failed")
				}

				if err := dialog.Send(pack); err != nil {
					return errors.Wrap(err, "send echo failed")
				}

				log.Infof("[SEND] %d echo %s", dialog.ID(), msg)
				time.Sleep(time.Millisecond * 500)
			}

			return nil
		})
	})

	wg.Wait()

	return ErrSendFinished
}

func recvEcho(dialog xnet.ClientDialog) error {
	for pack := range dialog.Receive() {
		msg := &message.Packet{}
		if err := json.Unmarshal(pack, msg); err != nil {
			return errors.Wrap(err, "unmarshal echo recv pack failed")
		}

		log.Infof("[RECV] echo %s", msg)
	}

	return nil
}
