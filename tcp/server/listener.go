package tcp

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/internal/util"
	"github.com/go-pantheon/fabrica-util/errors"
)

var _ internal.Listener = (*Listener)(nil)

type Listener struct {
	bind     string
	conf     conf.Config
	listener *net.TCPListener
	widGener *atomic.Uint64
}

func newListener(bind string, conf conf.Config) *Listener {
	return &Listener{
		bind:     bind,
		conf:     conf,
		widGener: &atomic.Uint64{},
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

func (l *Listener) Accept(ctx context.Context, widGener *atomic.Uint64) (net.Conn, uint64, error) {
	conn, err := l.listener.AcceptTCP()
	if err != nil {
		return nil, 0, errors.Wrapf(err, "accept failed")
	}

	if err := l.configure(conn); err != nil {
		return nil, 0, errors.Wrapf(err, "configure connection failed")
	}

	return conn, l.widGener.Add(1), nil
}

func (l *Listener) configure(conn net.Conn) error {
	tcpConn := conn.(*net.TCPConn)

	if err := tcpConn.SetKeepAlive(l.conf.Server.KeepAlive); err != nil {
		return errors.Wrapf(err, "SetKeepAlive failed v=%v", l.conf.Server.KeepAlive)
	}

	if err := tcpConn.SetReadBuffer(l.conf.Server.ReadBufSize); err != nil {
		return errors.Wrapf(err, "SetReadBuffer failed v=%d", l.conf.Server.ReadBufSize)
	}

	if err := tcpConn.SetWriteBuffer(l.conf.Server.WriteBufSize); err != nil {
		return errors.Wrapf(err, "SetWriteBuffer failed v=%d", l.conf.Server.WriteBufSize)
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
