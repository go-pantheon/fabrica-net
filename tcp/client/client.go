package tcp

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/internal/bufreader"
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

type Client struct {
	xsync.Closable

	Id   int64
	bind string

	conn   *net.TCPConn
	reader *bufreader.Reader

	receivedPackChan chan []byte
}

func NewClient(id int64, opts ...Option) *Client {
	c := &Client{
		Closable: xsync.NewClosure(time.Second * 10),
		Id:       id,
	}

	for _, o := range opts {
		o(c)
	}

	c.receivedPackChan = make(chan []byte, 1024)

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

	c.reader = bufreader.NewReader(conn, 4096)
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
		if c.reader != nil {
			if err := c.reader.Close(); err != nil {
				log.Errorf("[tcp.Client] cli=%d bufreader close failed", c.Id)
			}
		}

		close(c.receivedPackChan)

		log.Infof("[tcp.Client] client is closed. id=%d", c.Id)
	}); doCloseErr != nil {
		log.Errorf("[tcp.Client] client is closed. id=%d err=%v", c.Id, doCloseErr)
	}
}

func (c *Client) readPackLoop() error {
	for {
		pack, err := c.read()
		if err != nil {
			return err
		}

		c.receivedPackChan <- pack
	}
}

func (c *Client) read() (buf []byte, err error) {
	lb, err := c.reader.ReadFull(xnet.PackLenSize)
	if err != nil {
		return nil, errors.Wrapf(err, "read pack len failed")
	}

	var packLen int32

	err = binary.Read(bytes.NewReader(lb), binary.BigEndian, &packLen)
	if err != nil {
		return nil, errors.Wrapf(err, "parse pack len failed")
	}

	buf, err = c.reader.ReadFull(int(packLen))
	if err != nil {
		return nil, errors.Wrapf(err, "read pack body failed. len=%d", packLen)
	}

	return buf, nil
}

func (c *Client) write(pack []byte) (err error) {
	var buf bytes.Buffer

	packLen := int32(len(pack))
	err = binary.Write(&buf, binary.BigEndian, packLen)
	if err != nil {
		return errors.Wrapf(err, "write pack len failed. len=%d", packLen)
	}

	_, err = buf.Write(pack)
	if err != nil {
		return errors.Wrapf(err, "write pack body failed. len=%d", packLen)
	}

	_, err = c.conn.Write(buf.Bytes())
	if err != nil {
		return errors.Wrapf(err, "write pack failed. len=%d", packLen)
	}

	return nil
}

func (c *Client) Send(pack []byte) (err error) {
	return c.write(pack)
}

func (c *Client) Receive() <-chan []byte {
	return c.receivedPackChan
}
