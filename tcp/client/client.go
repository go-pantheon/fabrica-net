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
	xsync.Closable

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
		Closable: xsync.NewClosure(time.Second * 10),
		Id:       id,
		cryptor:  xnet.NewUnCryptor(),
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

	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)
	c.conn = conn

	xsync.GoSafe(fmt.Sprintf("tcp.client.id=%d", c.Id), func() error {
		return c.receive(ctx)
	})

	return nil
}

func (c *Client) receive(ctx context.Context) error {
	defer c.stop()

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		select {
		case <-c.CloseTriggered():
			c.stop()
			return xsync.ErrGroupIsClosing
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	eg.Go(func() error {
		return xsync.RunSafe(func() error {
			return c.readPackLoop()
		})
	})

	return eg.Wait()
}

func (c *Client) stop() {
	if doCloseErr := c.DoClose(func() {
		close(c.receivedPackChan)

		if err := c.writer.Flush(); err != nil {
			log.Errorf("[tcp.Client] client is closed. id=%d err=%v", c.Id, err)
		}

		log.Infof("[tcp.Client] client is closed. id=%d", c.Id)
	}); doCloseErr != nil {
		log.Errorf("[tcp.Client] client is closed. id=%d err=%v", c.Id, doCloseErr)
	}
}

func (c *Client) readPackLoop() error {
	for {
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

func (c *Client) Send(pack xnet.Pack) (err error) {
	if pack, err = c.cryptor.Encrypt(pack); err != nil {
		return err
	}

	return codec.Encode(c.writer, pack)
}

func (c *Client) Receive() <-chan xnet.Pack {
	return c.receivedPackChan
}
