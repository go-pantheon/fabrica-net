package tcp

import (
	"context"
	"net"

	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/internal/util"
	"github.com/go-pantheon/fabrica-net/tcp/frame"
	"github.com/go-pantheon/fabrica-util/errors"
)

var _ internal.Listener = (*Listener)(nil)

type Listener struct {
	bind     string
	conf     conf.TCP
	listener *net.TCPListener
	widGener *internal.WIDGenerator
}

func newListener(bind string, conf conf.TCP) *Listener {
	return &Listener{
		bind:     bind,
		conf:     conf,
		widGener: internal.NewWIDGenerator(internal.NetTypeTCP),
	}
}

func (l *Listener) Start(ctx context.Context) error {
	addr, err := net.ResolveTCPAddr("tcp", l.bind)
	if err != nil {
		return errors.Wrapf(err, "resolve bind failed. bind=%s", l.bind)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return errors.Wrapf(err, "listen failed. addr=%s", addr.String())
	}

	util.SetDeadlineWithContext(ctx, listener, "TcpListener")
	l.listener = listener

	return nil
}

func (l *Listener) Stop(ctx context.Context) error {
	if l.listener != nil {
		return l.listener.Close()
	}

	return nil
}

func (l *Listener) Accept(ctx context.Context) (wrapper internal.ConnWrapper, err error) {
	conn, err := l.listener.AcceptTCP()
	if err != nil {
		return internal.ConnWrapper{}, errors.Wrapf(err, "accept failed")
	}

	defer func() {
		if err != nil {
			if closeErr := conn.Close(); closeErr != nil {
				err = errors.Join(err, errors.Wrapf(closeErr, "close tcp connection failed"))
			}
		}
	}()

	if err := l.configure(conn); err != nil {
		return internal.ConnWrapper{}, errors.Wrapf(err, "configure connection failed")
	}

	return internal.NewConnWrapper(l.widGener.Next(), conn, frame.New(conn)), nil
}

func (l *Listener) configure(conn net.Conn) error {
	tcpConn := conn.(*net.TCPConn)

	if err := tcpConn.SetKeepAlive(l.conf.KeepAlive); err != nil {
		return errors.Wrapf(err, "SetKeepAlive failed v=%v", l.conf.KeepAlive)
	}

	if err := tcpConn.SetReadBuffer(l.conf.ReadBufSize); err != nil {
		return errors.Wrapf(err, "SetReadBuffer failed v=%d", l.conf.ReadBufSize)
	}

	if err := tcpConn.SetWriteBuffer(l.conf.WriteBufSize); err != nil {
		return errors.Wrapf(err, "SetWriteBuffer failed v=%d", l.conf.WriteBufSize)
	}

	return nil
}

func (l *Listener) Endpoint() (string, error) {
	addr, err := util.Extract(l.bind, l.listener)
	if err != nil {
		return "", err
	}

	return "tcp://" + addr, nil
}
