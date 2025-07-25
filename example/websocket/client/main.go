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
	ws "github.com/go-pantheon/fabrica-net/websocket/client"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

var ErrSendFinished = errors.New("send finished")

func main() {
	cli := ws.NewClient(1, "ws://localhost:8080/ws", handshakePack, client.WithAuthFunc(authFunc))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cli.Start(ctx); err != nil {
		log.Errorf("failed to start client: %+v", err)
		return
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

	if err := eg.Wait(); err != nil {
		log.Errorf("stop client failed. %+v", err)
	}

	log.Infof("client stopped")
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

	log.Infof("[RECV] %d auth %s", authMsg.ConnID, authMsg)

	return xnet.DefaultSession(), nil
}

func sendEcho(cli *ws.Client) error {
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

		log.Infof("[RECV] %d echo %s", dialog.ID(), msg)
	}

	return nil
}
