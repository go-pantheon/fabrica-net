package internal

import (
	"context"
	"net"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/codec"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

type Dialog struct {
	xsync.Stoppable

	id     int64
	connID int64

	authFunc      client.AuthFunc
	handshakePack HandshakePackFunc
	session       xnet.Session

	conn  net.Conn
	codec codec.Codec

	receivedPackChan chan xnet.Pack
}

func newDialog(id, connID int64, handshakePack HandshakePackFunc, wrapper ConnWrapper, authFunc client.AuthFunc, receivedPackChan chan xnet.Pack) *Dialog {
	return &Dialog{
		Stoppable:        xsync.NewStopper(10 * time.Second),
		id:               id,
		connID:           connID,
		authFunc:         authFunc,
		handshakePack:    handshakePack,
		session:          xnet.DefaultSession(),
		conn:             wrapper.Conn,
		codec:            wrapper.Codec,
		receivedPackChan: receivedPackChan,
	}
}

func (d *Dialog) start(ctx context.Context) error {
	handshakePack, err := d.handshakePack(d.connID)
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

	return eg.Wait()
}

func (d *Dialog) handshake(ctx context.Context, pack xnet.Pack) (err error) {
	if err = d.send(pack); err != nil {
		return err
	}

	log.Infof("[SEND] auth %d %d %s", d.id, d.connID, pack)

	pack, free, err := d.codec.Decode()
	if err != nil {
		return err
	}

	defer free()

	d.session, err = d.authFunc(ctx, pack)
	if err != nil {
		return err
	}

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

func (d *Dialog) send(pack xnet.Pack) (err error) {
	if d.OnStopping() {
		return xsync.ErrIsStopped
	}

	if pack, err = d.session.Encrypt(pack); err != nil {
		return err
	}

	return d.codec.Encode(pack)
}

func (d *Dialog) stop() (err error) {
	return d.TurnOff(func() error {
		close(d.receivedPackChan)

		if d.conn != nil {
			if closeErr := d.conn.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}

		log.Infof("[dialog] %d-%d stopped.", d.id, d.connID)

		return err
	})
}
