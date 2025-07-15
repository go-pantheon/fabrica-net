package tcp

import (
	"context"
	"fmt"
	"net"

	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/internal/util"
	"github.com/go-pantheon/fabrica-net/tcp/frame"
	"github.com/go-pantheon/fabrica-util/errors"
)

var _ internal.Dialer = (*dialer)(nil)

type dialer struct {
	bind string
	id   int64
}

func newDialer(id int64, bind string) *dialer {
	return &dialer{
		bind: bind,
		id:   id,
	}
}

func (d *dialer) Dial(ctx context.Context, target string) ([]internal.ConnWrapper, error) {
	addr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		return nil, errors.Wrapf(err, "resolve addr failed. addr=%s", target)
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "connect failed. addr=%s", target)
	}

	util.SetDeadlineWithContext(ctx, conn, fmt.Sprintf("client=%d", d.id))

	return []internal.ConnWrapper{internal.NewConnWrapper(uint64(d.id), conn, frame.New(conn))}, nil
}

func (d *dialer) Stop(ctx context.Context) error {
	return nil
}

func (d *dialer) Target() string {
	return d.bind
}
