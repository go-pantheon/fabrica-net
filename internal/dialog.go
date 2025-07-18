package internal

import (
	"context"
	"net"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

var _ xnet.ClientDialog = (*Dialog)(nil)

type Dialog struct {
	xsync.Stoppable

	id    uint64
	cliID int64

	authFunc      client.AuthFunc
	handshakePack client.HandshakePackFunc
	authed        chan struct{}
	session       xnet.Session

	conn  net.Conn
	codec Codec

	receivedPackChan chan xnet.Pack
}

func newDialog(id uint64, cid int64, handshakePack client.HandshakePackFunc, carrier ConnCarrier, authFunc client.AuthFunc) *Dialog {
	return &Dialog{
		Stoppable:        xsync.NewStopper(10 * time.Second),
		id:               id,
		cliID:            cid,
		authFunc:         authFunc,
		handshakePack:    handshakePack,
		authed:           make(chan struct{}),
		session:          xnet.DefaultSession(),
		conn:             carrier.Conn,
		codec:            carrier.Codec,
		receivedPackChan: make(chan xnet.Pack, 1024),
	}
}

func (d *Dialog) start(ctx context.Context) error {
	handshakePack, err := d.handshakePack(d.id)
	if err != nil {
		return err
	}

	if err := d.handshake(ctx, handshakePack); err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		select {
		case <-d.StopTriggered():
			return xsync.ErrStopByTrigger
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	eg.Go(func() error {
		return xsync.Run(func() error {
			return d.receiveLoop(ctx)
		})
	})

	close(d.authed)

	log.Infof("[dialog] %d-%d started.", d.cliID, d.id)

	return eg.Wait()
}

func (d *Dialog) stop() (err error) {
	return d.TurnOff(func() error {
		close(d.receivedPackChan)

		if d.conn != nil {
			if closeErr := d.conn.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}

		log.Infof("[dialog] %d-%d stopped.", d.cliID, d.id)

		return err
	})
}

func (d *Dialog) handshake(ctx context.Context, pack xnet.Pack) (err error) {
	if err = d.Send(pack); err != nil {
		return err
	}

	log.Debugf("[SEND] auth %d %d %s", d.cliID, d.id, pack)

	pack, free, err := d.codec.Decode()
	if err != nil {
		return err
	}

	defer free()

	d.session, err = d.authFunc(ctx, pack)
	if err != nil {
		return err
	}

	log.Debugf("[RECV] auth %d %d %s", d.cliID, d.id, pack)

	return nil
}

func (d *Dialog) receiveLoop(ctx context.Context) error {
	for {
		select {
		case <-d.StopTriggered():
			return xsync.ErrStopByTrigger
		case <-ctx.Done():
			return ctx.Err()
		default:
			pack, free, err := d.codec.Decode()
			if err != nil {
				return err
			}

			defer free()

			if pack, err = d.session.Decrypt(pack); err != nil {
				return err
			}

			d.receivedPackChan <- pack
		}
	}
}

func (d *Dialog) Send(pack xnet.Pack) (err error) {
	if d.OnStopping() {
		return xsync.ErrIsStopped
	}

	if pack, err = d.session.Encrypt(pack); err != nil {
		return err
	}

	return d.codec.Encode(pack)
}

func (d *Dialog) Receive() <-chan xnet.Pack {
	return d.receivedPackChan
}

func (d *Dialog) WaitAuthed() {
	<-d.authed
}

func (d *Dialog) ID() uint64 {
	return d.id
}
