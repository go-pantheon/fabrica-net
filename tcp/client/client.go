package tcp

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/internal/codec"
	"github.com/go-pantheon/fabrica-net/xcontext"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

// ErrTimeout is an error that occurs when the client times out.
var ErrTimeout = errors.New("i/o timeout")

// Option is a function that configures the client.
type Option func(c *Client)

func Bind(bind string) Option {
	return func(s *Client) {
		s.bind = bind
	}
}

func Cryptor(cryptor xnet.Cryptor) Option {
	return func(s *Client) {
		s.cryptor = cryptor
	}
}

type Client struct {
	xsync.Stoppable

	Id   int64
	bind string

	conn   *net.TCPConn
	reader *bufio.Reader
	writer *bufio.Writer

	cryptor xnet.Cryptor

	receivedPackChan chan xnet.Pack
}

func NewClient(id int64, opts ...Option) *Client {
	c := &Client{
		Stoppable: xsync.NewStopper(time.Second * 10),
		cryptor:   xnet.NewUnCryptor(),
		Id:        id,
	}

	for _, o := range opts {
		o(c)
	}

	c.receivedPackChan = make(chan xnet.Pack, 1024)

	return c
}

func (c *Client) Start(ctx context.Context) (err error) {
	addr, err := net.ResolveTCPAddr("tcp", c.bind)
	if err != nil {
		return errors.Wrapf(err, "resolve addr failed. addr=%s", c.bind)
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return errors.Wrapf(err, "connect failed. addr=%s", c.bind)
	}

	xcontext.SetDeadlineWithContext(ctx, conn, fmt.Sprintf("client=%d", c.Id))

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)

	c.GoAndQuickStop(fmt.Sprintf("tcp.client.receive.id=%d", c.Id), func() error {
		return c.receive(ctx)
	}, func() error {
		return c.Stop(ctx)
	})

	log.Infof("[tcp.client] %d started.", c.Id)

	return nil
}

func (c *Client) receive(ctx context.Context) error {
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

func (c *Client) receiveLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			pack, free, err := codec.Decode(c.reader)
			if err != nil {
				return err
			}

			defer free()

			if pack, err = c.cryptor.Decrypt(pack); err != nil {
				return err
			}

			c.receivedPackChan <- pack
		}
	}
}

func (c *Client) Stop(ctx context.Context) (err error) {
	return c.TurnOff(func() error {
		close(c.receivedPackChan)

		if flushErr := c.writer.Flush(); flushErr != nil {
			err = errors.Join(err, flushErr)
		}

		if closeErr := c.conn.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}

		log.Infof("[tcp.client] %d stopped.", c.Id)

		return err
	})
}

func (c *Client) Send(pack xnet.Pack) (err error) {
	if c.OnStopping() {
		return xsync.ErrIsStopped
	}

	if pack, err = c.cryptor.Encrypt(pack); err != nil {
		return err
	}

	return codec.Encode(c.writer, pack)
}

func (c *Client) Receive() <-chan xnet.Pack {
	return c.receivedPackChan
}
