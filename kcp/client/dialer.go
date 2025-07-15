package client

import (
	"context"
	"fmt"
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
	clientID   int64
	target     string
	conf       conf.KCP
	smux       *smux.Session
	configurer *util.ConnConfigurer
	validator  *util.ConfigValidator
}

func NewDialer(id int64, target string, conf conf.KCP) (*Dialer, error) {
	validator := util.NewConfigValidator()
	if err := validator.Validate(conf); err != nil {
		return nil, err
	}

	return &Dialer{
		target:     target,
		conf:       conf,
		configurer: util.NewConnConfigurer(conf),
		validator:  validator,
	}, nil
}

func (d *Dialer) Dial(ctx context.Context, target string) ([]internal.ConnWrapper, error) {
	conn, err := d.dial(ctx, target, time.Second*10)
	if err != nil {
		return nil, err
	}

	d.configurer.ConfigureConnection(conn)

	if !d.conf.Smux {
		return []internal.ConnWrapper{internal.NewConnWrapper(0, conn, frame.New(conn))}, nil
	}

	session, err := smux.Client(conn, d.configurer.CreateSmuxConfig())
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, errors.Wrapf(closeErr, "close kcp connection failed"))
		}

		return nil, errors.Wrapf(err, "create smux session failed")
	}

	d.smux = session

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
	}()

	for i := 1; i <= d.conf.SmuxStreamSize; i++ {
		stream, streamErr := session.OpenStream()
		if streamErr != nil {
			return nil, errors.Wrapf(streamErr, "create smux stream %d failed", i)
		}

		wrappers = append(wrappers, internal.NewConnWrapper(uint64(d.clientID*100+int64(i)), stream, frame.New(stream)))
	}

	return wrappers, nil
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

func (d *Dialer) Stop(ctx context.Context) error {
	if d.smux != nil {
		if err := d.smux.Close(); err != nil {
			return errors.Wrapf(err, "close smux session failed")
		}
	}

	return nil
}

func (d *Dialer) Target() string {
	return d.target
}
