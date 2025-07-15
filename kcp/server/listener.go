package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/kcp/frame"
	"github.com/go-pantheon/fabrica-net/kcp/util"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	kcpgo "github.com/xtaci/kcp-go/v5"
)

var _ internal.Listener = (*Listener)(nil)

// Listener implements internal.Listener for KCP protocol
type Listener struct {
	xsync.Stoppable

	bind        string
	conf        conf.KCP
	listener    *kcpgo.Listener
	connIdGener *internal.ConnIDGenerator
	streamChan  chan internal.ConnWrapper

	smuxIDGenerator *atomic.Int64
	smuxSessions    *sync.Map

	// Shared configuration utilities
	configurer *util.ConnConfigurer
	validator  *util.ConfigValidator
}

func newListener(bind string, conf conf.KCP) (*Listener, error) {
	validator := util.NewConfigValidator()
	if err := validator.Validate(conf); err != nil {
		return nil, err
	}

	return &Listener{
		Stoppable:       xsync.NewStopper(10 * time.Second),
		bind:            bind,
		conf:            conf,
		connIdGener:     internal.NewConnIDGenerator(internal.NetTypeKCP),
		streamChan:      make(chan internal.ConnWrapper, 1024),
		smuxIDGenerator: &atomic.Int64{},
		smuxSessions:    &sync.Map{},
		configurer:      util.NewConnConfigurer(conf),
		validator:       validator,
	}, nil
}

func (l *Listener) Start(ctx context.Context) error {
	listener, err := kcpgo.ListenWithOptions(l.bind, nil, l.conf.DataShards, l.conf.ParityShards)
	if err != nil {
		return errors.Wrapf(err, "kcp listen failed. bind=%s", l.bind)
	}

	if err := listener.SetReadBuffer(l.conf.ReadBufSize); err != nil {
		return errors.Wrapf(err, "set read buffer failed")
	}

	if err := listener.SetWriteBuffer(l.conf.WriteBufSize); err != nil {
		return errors.Wrapf(err, "set write buffer failed")
	}

	if err := listener.SetDSCP(l.conf.DSCP); err != nil {
		return errors.Wrapf(err, "set dscp failed")
	}

	l.listener = listener

	return nil
}

func (l *Listener) Accept(ctx context.Context) (internal.ConnWrapper, error) {
	for {
		select {
		case <-l.StopTriggered():
			return internal.ConnWrapper{}, xsync.ErrStopByTrigger
		case <-ctx.Done():
			return internal.ConnWrapper{}, ctx.Err()
		case conn := <-l.streamChan:
			return conn, nil
		default:
			wrapper, err := l.accept()
			if err != nil {
				return internal.ConnWrapper{}, err
			}

			if !l.conf.Smux {
				return wrapper, nil
			}

			if err := l.startSmux(ctx, wrapper); err != nil {
				return internal.ConnWrapper{}, err
			}

			continue
		}
	}
}

func (l *Listener) accept() (internal.ConnWrapper, error) {
	conn, err := l.listener.AcceptKCP()
	if err != nil {
		return internal.ConnWrapper{}, errors.Wrapf(err, "accept kcp failed")
	}

	l.configurer.ConfigureConnection(conn)

	return internal.NewConnWrapper(l.connIdGener.Next(), conn, frame.New(conn)), nil
}

func (l *Listener) startSmux(ctx context.Context, wrapper internal.ConnWrapper) error {
	id := l.smuxIDGenerator.Add(1)
	conn := wrapper.Conn.(*kcpgo.UDPSession)

	smux, err := newSmux(id, conn, l.conf, l.connIdGener)
	if err != nil {
		return errors.Wrapf(err, "new smux failed")
	}

	l.smuxSessions.Store(id, smux)

	smux.GoAndStop(fmt.Sprintf("kcp.Listener.newSmux.id-%d", id), func() error {
		return smux.start(ctx, l.streamChan)
	}, func() error {
		l.smuxSessions.Delete(id)
		return smux.stop()
	})

	return nil
}

func (l *Listener) Stop(ctx context.Context) (err error) {
	return l.TurnOff(func() error {
		var (
			wg      sync.WaitGroup
			safeErr = &errors.SafeJoinError{}
		)

		l.smuxSessions.Range(func(key, value any) bool {
			wg.Add(1)

			if err := xsync.Timeout(ctx, fmt.Sprintf("kcp.Listener.stop.smux-%d", key), func() error {
				defer wg.Done()

				return value.(*Smux).stop()
			}, 10*time.Second); err != nil {
				safeErr.Join(err)
			}

			return true
		})

		wg.Wait()

		if safeErr.Error() != "" {
			err = errors.Join(err, safeErr)
		}

		if l.listener != nil {
			if closeErr := l.listener.Close(); closeErr != nil {
				err = errors.Join(err, errors.Wrapf(closeErr, "close listener failed"))
			}
		}

		return err
	})
}

func (l *Listener) Endpoint() (string, error) {
	if l.listener == nil {
		return "", errors.New("listener not started")
	}

	return "kcp://" + l.listener.Addr().String(), nil
}
