package tcp

import (
	"context"
	"fmt"
	"net"

	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/internal/util"
	"github.com/go-pantheon/fabrica-net/tcp/frame"
	"github.com/go-pantheon/fabrica-util/errors"
)

var _ internal.Dialer = (*dialer)(nil)

type dialer struct {
	cliID int64
	bind  string
}

func newDialer(cliID int64, bind string) *dialer {
	return &dialer{
		cliID: cliID,
		bind:  bind,
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

	util.SetDeadlineWithContext(ctx, conn, fmt.Sprintf("client=%d", d.cliID))

	return []internal.ConnWrapper{internal.NewConnWrapper(client.DialogID(d.cliID, 0), conn, frame.New(conn))}, nil
}

func (d *dialer) Stop(ctx context.Context) error {
	return nil
}

func (d *dialer) Target() string {
	return d.bind
}
