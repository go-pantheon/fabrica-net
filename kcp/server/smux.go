package server

import (
	"context"
	"time"

	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/kcp/frame"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	kcpgo "github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
)

type Smux struct {
	xsync.Stoppable

	id       int64
	conn     *kcpgo.UDPSession
	session  *smux.Session
	widGener *internal.WIDGenerator
}

func newSmux(id int64, conn *kcpgo.UDPSession, conf conf.KCP, widGener *internal.WIDGenerator) (*Smux, error) {
	session, err := smux.Server(conn, newSmuxConfig(conf))
	if err != nil {
		return nil, errors.Wrapf(err, "create smux session failed")
	}

	return &Smux{
		Stoppable: xsync.NewStopper(10 * time.Second),
		id:        id,
		conn:      conn,
		session:   session,
		widGener:  widGener,
	}, nil
}

func (s *Smux) start(ctx context.Context, streamChan chan internal.ConnWrapper) error {
	for {
		select {
		case <-s.StopTriggered():
			return xsync.ErrStopByTrigger
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := s.accept()
			if err != nil {
				return err
			}

			streamChan <- conn
		}
	}
}

func (s *Smux) accept() (internal.ConnWrapper, error) {
	stream, err := s.session.AcceptStream()
	if err != nil {
		return internal.ConnWrapper{}, errors.Wrapf(err, "accept smux stream failed")
	}

	return internal.NewConnWrapper(s.widGener.Next(), stream, frame.New(stream)), nil
}

func (s *Smux) stop() error {
	return s.TurnOff(func() error {
		if s.conn != nil {
			if closeErr := s.conn.Close(); closeErr != nil {
				return errors.Wrapf(closeErr, "smux close kcp connection failed")
			}
		}

		if s.session != nil {
			if closeErr := s.session.Close(); closeErr != nil {
				return errors.Wrapf(closeErr, "close smux session failed")
			}
		}

		return nil
	})
}

func newSmuxConfig(conf conf.KCP) *smux.Config {
	smuxConfig := smux.DefaultConfig()
	smuxConfig.Version = 2
	smuxConfig.KeepAliveInterval = conf.KeepAliveInterval
	smuxConfig.KeepAliveTimeout = conf.KeepAliveTimeout
	smuxConfig.MaxFrameSize = conf.MaxFrameSize
	smuxConfig.MaxReceiveBuffer = conf.MaxReceiveBuffer

	return smuxConfig
}
