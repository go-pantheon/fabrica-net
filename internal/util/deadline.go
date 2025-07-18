package util

import (
	"context"
	"io"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
)

// ReadDeadline https://github.com/google/mtail/commit/8dd02e80f9e42eebb59fee10c24c7cc686f9e481
type ReadDeadline interface {
	SetReadDeadline(t time.Time) error
}

type Deadline interface {
	SetDeadline(t time.Time) error
}

// SetDeadlineWithContext use context to control the deadline of the connection
func SetDeadlineWithContext(ctx context.Context, d Deadline, tag string) {
	xsync.Go("util.SetDeadlineWithContext", func() error {
		<-ctx.Done()

		log.Debugf("[xcontext.SetDeadlineWithContext] %s start to close", tag)

		if err := d.SetDeadline(time.Now()); err != nil {
			return errors.Wrapf(err, "[xcontext.SetDeadlineWithContext] %s close failed", tag)
		}

		return nil
	})
}

// CloseOnCancel close the connection when the context is canceled
func CloseOnCancel(ctx context.Context, closer io.Closer, tag string) {
	xsync.Go("util.CloseOnCancel", func() error {
		<-ctx.Done()

		log.Debugf("[xcontext.CloseOnCancel] %s start to close", tag)

		if err := closer.Close(); err != nil {
			return errors.Wrapf(err, "[xcontext.CloseOnCancel] %s close failed", tag)
		}

		return nil
	})
}

// SetDeadlineWithTimeout set the deadline to close the connection after the specified timeout
func SetDeadlineWithTimeout(d Deadline, timeout time.Duration, tag string) {
	xsync.Go("util.SetDeadlineWithTimeout", func() error {
		timer := time.NewTimer(timeout)
		<-timer.C

		log.Debugf("[xcontext.SetDeadlineWithTimeout] %s start to close", tag)

		if err := d.SetDeadline(time.Now()); err != nil {
			return errors.Wrapf(err, "[xcontext.SetDeadlineWithTimeout] %s close failed", tag)
		}

		return nil
	})
}
