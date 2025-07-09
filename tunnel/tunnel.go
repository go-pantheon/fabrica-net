package tunnel

import (
	"context"
	"fmt"
	"time"

	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

const (
	recordPushErrStartAt = time.Second * 10
	recordPushErrCount   = 10
)

var _ xnet.Tunnel = (*Tunnel)(nil)

type Tunnel struct {
	xsync.Stoppable
	xnet.AppTunnel

	pusher xnet.Pusher
	csChan chan xnet.TunnelMessage

	recordPushErrStartAt time.Time // the start time of the last 10 seconds
	recordPushErrCount   int       // the count of continuous push error in the last 10 seconds
}

func NewTunnel(ctx context.Context, pusher xnet.Pusher, app xnet.AppTunnel) *Tunnel {
	t := &Tunnel{
		Stoppable: xsync.NewStopper(time.Second * 10),
		AppTunnel: app,
		pusher:    pusher,
		csChan:    make(chan xnet.TunnelMessage, 1024),
	}

	t.GoAndStop(fmt.Sprintf("gate.Tunnel-%d-%d-%d", t.UID(), t.Type(), t.OID()), func() error {
		return t.run(ctx)
	}, func() error {
		return t.Stop(ctx)
	})

	return t
}

func (t *Tunnel) run(ctx context.Context) (err error) {
	defer func() {
		if stopErr := t.stop(ctx, err); stopErr != nil {
			err = errors.Join(err, stopErr)
		}
	}()

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.StopTriggered():
			return xsync.ErrStopByTrigger
		}
	})
	eg.Go(func() error {
		return xsync.Run(func() error {
			return t.csLoop(ctx)
		})
	})
	eg.Go(func() error {
		return xsync.Run(func() error {
			return t.scLoop(ctx)
		})
	})

	return eg.Wait()
}

func (t *Tunnel) csLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cs, ok := <-t.csChan:
			if !ok {
				return errors.New("cs channel closed")
			}

			if err := t.CSHandle(cs); err != nil {
				return err
			}
		}
	}
}

func (t *Tunnel) scLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := t.SCHandle()
			if err != nil {
				return err
			}

			pack, err := t.TunnelMsgToPack(ctx, msg)
			if err != nil {
				return err
			}

			if err = t.pusher.Push(ctx, pack); err != nil {
				if t.recordPushErr(ctx, err) {
					return err
				}
			} else {
				t.recordPushErrCount = 0 // reset the count when push success
			}
		}
	}
}

func (t *Tunnel) recordPushErr(ctx context.Context, err error) (overload bool) {
	t.Log().WithContext(ctx).Errorf("push failed uid=%d sid=%d color=%s status=%d oid=%d. %+v", t.UID(), t.Session().SID(), t.Color(), t.Session().Status(), t.OID(), err)

	now := time.Now()

	if now.Sub(t.recordPushErrStartAt) > recordPushErrStartAt {
		t.recordPushErrStartAt = now
		t.recordPushErrCount = 0
	}

	t.recordPushErrCount++

	return t.recordPushErrCount > recordPushErrCount
}

func (t *Tunnel) Forward(ctx context.Context, p xnet.TunnelMessage) error {
	if t.OnStopping() {
		return xsync.ErrIsStopped
	}

	t.csChan <- p

	return nil
}

func (t *Tunnel) stop(ctx context.Context, erreason error) (err error) {
	return t.TurnOff(func() error {
		close(t.csChan)

		if onStopErr := t.OnStop(ctx, erreason); onStopErr != nil {
			err = errors.Join(err, onStopErr)
		}

		return err
	})
}
