package server

import (
	"context"
	"time"

	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/kcp/frame"
	"github.com/go-pantheon/fabrica-net/kcp/util"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	kcpgo "github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
)

type Smux struct {
	xsync.Stoppable

	id        int64
	conn      *kcpgo.UDPSession
	session   *smux.Session
	connIDGen *internal.ConnIDGenerator
}

func newSmux(id int64, conn *kcpgo.UDPSession, conf conf.KCP, widGen *internal.ConnIDGenerator) (*Smux, error) {
	configurer := util.NewConnConfigurer(conf)
	session, err := smux.Server(conn, configurer.CreateSmuxConfig())
	if err != nil {
		return nil, errors.Wrapf(err, "create smux session failed")
	}

	return &Smux{
		Stoppable: xsync.NewStopper(10 * time.Second),
		id:        id,
		conn:      conn,
		session:   session,
		connIDGen: widGen,
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
			stream, err := s.session.AcceptStream()
			if err != nil {
				return errors.Wrapf(err, "accept stream failed")
			}

			streamChan <- internal.NewConnWrapper(s.connIDGen.Next(), stream, frame.New(stream))
		}
	}
}

func (s *Smux) stop() error {
	return s.TurnOff(func() error {
		if s.session != nil {
			if closeErr := s.session.Close(); closeErr != nil {
				return errors.Wrapf(closeErr, "close smux session failed")
			}
		}

		return nil
	})
}
