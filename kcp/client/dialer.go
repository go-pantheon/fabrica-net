package client

import (
	"context"
	"fmt"
	"net"
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

var _ internal.Dialer = (*Dialer)(nil)

type Dialer struct {
	id         int64
	target     string
	conf       conf.KCP
	configurer *util.ConnConfigurer
	validator  *util.ConfigValidator
}

func NewDialer(id int64, target string, conf conf.KCP) (*Dialer, error) {
	validator := util.NewConfigValidator()
	if err := validator.Validate(conf); err != nil {
		return nil, err
	}

	return &Dialer{
		id:         id,
		target:     target,
		conf:       conf,
		configurer: util.NewConnConfigurer(conf),
		validator:  validator,
	}, nil
}

func (d *Dialer) Dial(ctx context.Context, target string) (net.Conn, []internal.ConnWrapper, error) {
	conn, err := d.dial(ctx, target, time.Second*10)
	if err != nil {
		return nil, nil, err
	}

	if err := d.configurer.ConfigureConnection(conn); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, errors.Wrapf(closeErr, "close kcp connection failed. target=%s", target))
		}

		return nil, nil, err
	}

	if !d.conf.Smux {
		return conn, []internal.ConnWrapper{internal.NewConnWrapper(uint64(d.id), conn, frame.New(conn))}, nil
	}

	session, err := smux.Client(conn, d.configurer.CreateSmuxConfig())
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
	err = xsync.Timeout(ctx, fmt.Sprintf("kcp.dial-%s", target), func() error {
		if conn, err = kcpgo.DialWithOptions(target, nil, d.conf.DataShards, d.conf.ParityShards); err != nil {
			return err
		}

		return nil
	}, timeout)

	return conn, err
}

func (d *Dialer) Target() string {
	return d.target
}
