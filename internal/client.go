package internal

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/codec"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

var _ xnet.Client = (*BaseClient)(nil)

type BaseClient struct {
	xsync.Stoppable
	*client.Options

	Id      int64
	session xnet.Session

	handshakePack xnet.Pack

	dialer Dialer
	conn   io.ReadWriteCloser
	codec  codec.Codec

	receivedPackChan chan xnet.Pack
}

func NewBaseClient(id int64, handshakePack xnet.Pack, dialer Dialer, opts ...client.Option) *BaseClient {
	c := &BaseClient{
		Stoppable:     xsync.NewStopper(time.Second * 10),
		Options:       client.NewOptions(opts...),
		Id:            id,
		session:       xnet.DefaultSession(),
		handshakePack: handshakePack,
		dialer:        dialer,
	}

	c.receivedPackChan = make(chan xnet.Pack, 1024)

	return c
}

func (c *BaseClient) Start(ctx context.Context) (err error) {
	conn, codec, err := c.dialer.Dial(ctx, c.dialer.Target())
	if err != nil {
		return errors.Wrapf(err, "connect failed. target=%s", c.dialer.Target())
	}

	c.conn = conn
	c.codec = codec

	if err = c.handshake(ctx, c.handshakePack); err != nil {
		return err
	}

	c.GoAndStop(fmt.Sprintf("client.receive.id-%d", c.Id), func() error {
		return c.run(ctx)
	}, func() error {
		return c.Stop(ctx)
	})

	log.Infof("[client] %d started.", c.Id)

	return nil
}

func (c *BaseClient) handshake(ctx context.Context, pack xnet.Pack) (err error) {
	if err = c.Send(pack); err != nil {
		return err
	}

	log.Infof("[SEND] auth %d %s", c.Id, pack)

	pack, free, err := c.codec.Decode()
	if err != nil {
		return err
	}

	defer free()

	c.session, err = c.AuthFunc()(ctx, pack)
	if err != nil {
		return err
	}

	return nil
}

func (c *BaseClient) run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		select {
		case <-c.StopTriggered():
			return xsync.ErrStopByTrigger
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	eg.Go(func() error {
		return xsync.Run(func() error {
			return c.receiveLoop(ctx)
		})
	})

	return eg.Wait()
}

func (c *BaseClient) receiveLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			pack, free, err := c.codec.Decode()
			if err != nil {
				return err
			}

			defer free()

			if pack, err = c.session.Decrypt(pack); err != nil {
				return err
			}

			c.receivedPackChan <- pack
		}
	}
}

func (c *BaseClient) Send(pack xnet.Pack) (err error) {
	if c.OnStopping() {
		return xsync.ErrIsStopped
	}

	if pack, err = c.session.Encrypt(pack); err != nil {
		return err
	}

	return c.codec.Encode(pack)
}

func (c *BaseClient) Receive() <-chan xnet.Pack {
	return c.receivedPackChan
}

func (c *BaseClient) Stop(ctx context.Context) (err error) {
	return c.TurnOff(func() error {
		close(c.receivedPackChan)

		if c.conn != nil {
			if closeErr := c.conn.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}

		log.Infof("[client] %d stopped.", c.Id)

		return err
	})
}

func (c *BaseClient) Session() xnet.Session {
	return c.session
}

func (c *BaseClient) Target() string {
	return c.dialer.Target()
}
