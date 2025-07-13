package internal

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
)

var _ xnet.Client = (*BaseClient)(nil)

type BaseClient struct {
	*client.Options

	Id int64

	handshakePack xnet.Pack

	dialer    Dialer
	conn      net.Conn
	dialogMap *sync.Map

	receivedPackChan chan xnet.Pack
}

func NewBaseClient(id int64, handshakePack xnet.Pack, dialer Dialer, opts ...client.Option) *BaseClient {
	c := &BaseClient{
		Options:          client.NewOptions(opts...),
		Id:               id,
		handshakePack:    handshakePack,
		dialer:           dialer,
		dialogMap:        &sync.Map{},
		receivedPackChan: make(chan xnet.Pack, 1024),
	}

	return c
}

func (c *BaseClient) Start(ctx context.Context) (err error) {
	conn, wrappers, err := c.dialer.Dial(ctx, c.dialer.Target())
	if err != nil {
		return errors.Wrapf(err, "connect failed. target=%s", c.dialer.Target())
	}

	c.conn = conn

	for i, wrapper := range wrappers {
		id := int64(i)
		d := newDialog(c.Id, id, c.handshakePack, wrapper, c.AuthFunc(), c.receivedPackChan)
		c.dialogMap.Store(id, d)

		d.GoAndStop(fmt.Sprintf("client.receive.id-%d-%d", d.uid, d.id), func() error {
			return d.start(ctx)
		}, func() error {
			c.dialogMap.Delete(id)
			return d.stop()
		})
	}

	log.Infof("[client] %d started.", c.Id)

	return nil
}

func (c *BaseClient) Stop(ctx context.Context) (err error) {
	var wg sync.WaitGroup

	c.dialogMap.Range(func(key, value any) bool {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if stopErr := value.(*Dialog).stop(); stopErr != nil {
				err = errors.JoinUnsimilar(err, errors.Wrap(stopErr, "stop dialog failed"))
			}
		}()

		return true
	})

	wg.Wait()

	close(c.receivedPackChan)

	if c.conn != nil {
		if closeErr := c.conn.Close(); closeErr != nil {
			err = errors.Join(err, errors.Wrapf(closeErr, "close connection failed"))
		}
	}

	return err
}

func (c *BaseClient) Send(pack xnet.Pack) (err error) {
	c.dialogMap.Range(func(key, value any) bool {
		dialog := value.(*Dialog)
		if sendErr := dialog.send(pack); sendErr != nil {
			err = errors.Join(err, errors.Wrapf(sendErr, "send pack failed. id=%d", dialog.uid))
		}

		return false
	})

	return nil
}

func (c *BaseClient) SendSmux(pack xnet.Pack, streamID int64) error {
	dialog, ok := c.dialogMap.Load(streamID)
	if !ok {
		return errors.Errorf("dialog not found. id=%d", streamID)
	}

	return dialog.(*Dialog).send(pack)
}

func (c *BaseClient) Receive() <-chan xnet.Pack {
	return c.receivedPackChan
}

func (c *BaseClient) Target() string {
	return c.dialer.Target()
}
