package client

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/kcp/frame"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	kcpgo "github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
)

var _ internal.Dialer = (*Dialer)(nil)

type Dialer struct {
	id     int64
	target string
	conf   conf.KCP
}

func NewDialer(id int64, target string, conf conf.KCP) *Dialer {
	return &Dialer{
		id:     id,
		target: target,
		conf:   conf,
	}
}

func (d *Dialer) Dial(ctx context.Context, target string) (net.Conn, []internal.ConnWrapper, error) {
	conn, err := d.dial(ctx, target, time.Second*10)
	if err != nil {
		return nil, nil, err
	}

	if err := d.configureConn(conn); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, errors.Wrapf(closeErr, "close kcp connection failed. target=%s", target))
		}

		return nil, nil, err
	}

	if !d.conf.Smux {
		return conn, []internal.ConnWrapper{internal.NewConnWrapper(uint64(d.id), conn, frame.New(conn))}, nil
	}

	session, err := smux.Client(conn, d.newSmuxConfig())
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, errors.Wrapf(closeErr, "close kcp connection failed"))
		}

		return nil, nil, errors.Wrapf(err, "create smux session failed")
	}

	wrappers := make([]internal.ConnWrapper, 0, d.conf.SmuxStreamSize)

	defer func() {
		if err == nil {
			return
		}

		for _, wrapper := range wrappers {
			if closeErr := wrapper.Close(); closeErr != nil {
				err = errors.JoinUnsimilar(err, errors.Wrapf(closeErr, "close stream during cleanup failed"))
			}
		}

		if session != nil {
			if closeErr := session.Close(); closeErr != nil {
				err = errors.JoinUnsimilar(err, errors.Wrapf(closeErr, "close session during cleanup failed"))
			}
		}

		if closeErr := conn.Close(); closeErr != nil {
			err = errors.JoinUnsimilar(err, errors.Wrapf(closeErr, "close connection during cleanup failed"))
		}
	}()

	for i := 0; i < d.conf.SmuxStreamSize; i++ {
		stream, streamErr := session.OpenStream()
		if streamErr != nil {
			return nil, nil, errors.Wrapf(streamErr, "create smux stream %d failed", i)
		}

		wrappers = append(wrappers, internal.NewConnWrapper(uint64(d.id), stream, frame.New(stream)))
	}

	return conn, wrappers, nil
}

func (d *Dialer) dial(ctx context.Context, target string, timeout time.Duration) (conn *kcpgo.UDPSession, err error) {
	err = xsync.GoTimeout(ctx, fmt.Sprintf("kcp.dial-%s", target), func(errChan chan<- error) {
		conn, err = kcpgo.DialWithOptions(target, nil, d.conf.DataShards, d.conf.ParityShards)
		if err != nil {
			errChan <- err
			return
		}
	}, timeout)

	return conn, err
}

func (d *Dialer) Target() string {
	return d.target
}

func (d *Dialer) configureConn(conn *kcpgo.UDPSession) error {
	conn.SetNoDelay(d.conf.NoDelay[0], d.conf.NoDelay[1], d.conf.NoDelay[2], d.conf.NoDelay[3])
	conn.SetWindowSize(d.conf.WindowSize[0], d.conf.WindowSize[1])
	conn.SetMtu(d.conf.MTU)
	conn.SetACKNoDelay(d.conf.ACKNoDelay)
	conn.SetWriteDelay(d.conf.WriteDelay)

	if err := conn.SetReadBuffer(d.conf.ReadBufSize); err != nil {
		return errors.Wrapf(err, "set read buffer failed")
	}

	if err := conn.SetWriteBuffer(d.conf.WriteBufSize); err != nil {
		return errors.Wrapf(err, "set write buffer failed")
	}

	if err := conn.SetDSCP(d.conf.DSCP); err != nil {
		return errors.Wrapf(err, "set dscp failed")
	}

	return nil
}

func (d *Dialer) newSmuxConfig() *smux.Config {
	smuxConfig := smux.DefaultConfig()
	smuxConfig.Version = 2
	smuxConfig.KeepAliveInterval = d.conf.KeepAliveInterval
	smuxConfig.KeepAliveTimeout = d.conf.KeepAliveTimeout
	smuxConfig.MaxFrameSize = d.conf.MaxFrameSize
	smuxConfig.MaxReceiveBuffer = d.conf.MaxReceiveBuffer

	return smuxConfig
}
