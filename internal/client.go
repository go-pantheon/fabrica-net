package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
)

var _ xnet.Client = (*BaseClient)(nil)

type HandshakePackFunc func(wid uint64) (xnet.Pack, error)

type BaseClient struct {
	*client.Options

	Id int64

	handshakePack HandshakePackFunc

	dialer    Dialer
	dialogMap *sync.Map

	receivedPackChan chan xnet.Pack
}

func NewBaseClient(id int64, handshakePack HandshakePackFunc, dialer Dialer, options *client.Options) *BaseClient {
	c := &BaseClient{
		Options:          options,
		Id:               id,
		handshakePack:    handshakePack,
		dialer:           dialer,
		dialogMap:        &sync.Map{},
		receivedPackChan: make(chan xnet.Pack, 1024),
	}

	return c
}

func (c *BaseClient) Start(ctx context.Context) (err error) {
	wrappers, err := c.dialer.Dial(ctx, c.dialer.Target())
	if err != nil {
		return errors.Wrapf(err, "connect failed. target=%s", c.dialer.Target())
	}

	for _, wrapper := range wrappers {
		d := newDialog(c.Id, wrapper.WID, c.handshakePack, wrapper, c.AuthFunc())
		c.dialogMap.Store(wrapper.WID, d)

		d.GoAndStop(fmt.Sprintf("client.receive.id-%d-%d", d.clientID, d.id), func() error {
			return d.start(ctx)
		}, func() error {
			c.dialogMap.Delete(wrapper.WID)
			return d.stop()
		})
	}

	log.Infof("[client] %d started.", c.Id)

	return nil
}

func (c *BaseClient) Stop(ctx context.Context) (err error) {
	var (
		wg      sync.WaitGroup
		safeErr = &errors.SafeJoinError{}
	)

	c.dialogMap.Range(func(key, value any) bool {
		wg.Add(1)

		if err := xsync.Timeout(ctx, fmt.Sprintf("client.stop.dialog-%d-%d", c.Id, key), func() error {
			defer wg.Done()

			return value.(*Dialog).stop()
		}, 10*time.Second); err != nil {
			safeErr.Join(err)
		}

		return true
	})

	wg.Wait()

	if safeErr.Error() != "" {
		err = errors.Join(err, safeErr)
	}

	if dialerErr := c.dialer.Stop(ctx); dialerErr != nil {
		err = errors.Join(err, errors.Wrapf(dialerErr, "stop dialer failed"))
	}

	close(c.receivedPackChan)

	return err
}

func (c *BaseClient) Target() string {
	return c.dialer.Target()
}

func (c *BaseClient) WalkDialogs(fn func(dialog xnet.ClientDialog)) {
	c.dialogMap.Range(func(key, value any) bool {
		fn(value.(*Dialog))
		return true
	})
}
