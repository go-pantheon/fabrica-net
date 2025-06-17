package internal

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal/codec"
	"github.com/go-pantheon/fabrica-net/tunnel"
	"github.com/go-pantheon/fabrica-net/xcontext"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

var _ xnet.Worker = (*Worker)(nil)

type Worker struct {
	xsync.Stoppable

	id        uint64
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	connected atomic.Bool
	session   xnet.Session

	tunnelManager *tunnelManager

	conf             conf.Worker
	referer          string
	svc              xnet.Service
	createTunnelFunc CreateTunnelFunc

	readFilter  middleware.Middleware
	writeFilter middleware.Middleware

	replyChanStarted   atomic.Bool
	replyChanCompleted chan struct{}
	replyChan          chan xnet.Pack
}

func NewWorker(wid uint64, conn *net.TCPConn, logger log.Logger, conf conf.Worker, referer string,
	readFilter, writeFilter middleware.Middleware, svc xnet.Service) *Worker {
	w := &Worker{
		Stoppable:          xsync.NewStopper(conf.StopTimeout),
		tunnelManager:      newTunnelManager(conf.TunnelGroupSize),
		id:                 wid,
		conn:               conn,
		reader:             bufio.NewReader(conn),
		writer:             bufio.NewWriter(conn),
		conf:               conf,
		svc:                svc,
		referer:            referer,
		readFilter:         readFilter,
		writeFilter:        writeFilter,
		replyChanCompleted: make(chan struct{}),
		replyChan:          make(chan xnet.Pack, conf.ReplyChanSize),
	}

	w.createTunnelFunc = func(ctx context.Context, tp int32, oid int64) (xnet.Tunnel, error) {
		at, err := w.svc.CreateAppTunnel(ctx, w.session, tp, oid, w)
		if err != nil {
			return nil, err
		}

		return tunnel.NewTunnel(ctx, w, at), nil
	}

	w.session = xnet.DefaultSession()

	return w
}

func (w *Worker) Start(ctx context.Context) (err error) {
	if err = w.handshake(ctx); err != nil {
		return err
	}

	if err = w.svc.OnConnected(ctx, w.session); err != nil {
		return err
	}

	w.connected.Store(true)

	return
}

// handshake must only be used in auth
func (w *Worker) handshake(ctx context.Context) error {
	var (
		ss  xnet.Session
		out xnet.Pack
	)

	if err := w.Conn().SetDeadline(time.Now().Add(w.conf.HandshakeTimeout)); err != nil {
		return errors.Wrap(err, "set conn deadline before handshake failed")
	}

	in, free, err := codec.Decode(w.reader)
	if err != nil {
		return err
	}

	defer free()

	if out, ss, err = w.svc.Auth(ctx, in); err != nil {
		return err
	}

	if err = codec.Encode(w.writer, out); err != nil {
		return err
	}

	ss.SetClientIP(xcontext.RemoteAddr(w.conn))
	w.session = ss

	return nil
}

func (w *Worker) Run(ctx context.Context) error {
	ctx = xcontext.SetUID(ctx, w.UID())
	ctx = xcontext.SetSID(ctx, w.SID())
	ctx = xcontext.SetOID(ctx, w.UID())
	ctx = xcontext.SetColor(ctx, w.Color())
	ctx = xcontext.SetStatus(ctx, w.Status())
	ctx = xcontext.SetGateReferer(ctx, w.referer, w.WID())
	ctx = xcontext.SetClientIP(ctx, w.session.ClientIP())

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		select {
		case <-w.StopTriggered():
			return xsync.ErrStopByTrigger
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	eg.Go(func() error {
		err := xsync.RunSafe(func() error {
			return w.writeLoop(ctx)
		})

		return err
	})
	eg.Go(func() error {
		err := xsync.RunSafe(func() error {
			return w.readLoop(ctx)
		})

		return err
	})
	eg.Go(func() error {
		err := w.tick(ctx)
		return err
	})

	return eg.Wait()
}

func (w *Worker) Tunnel(ctx context.Context, mod int32, oid int64) (t xnet.Tunnel, err error) {
	tp, initCap, err := w.svc.TunnelType(mod)
	if err != nil {
		return nil, err
	}

	if t = w.tunnelManager.tunnel(tp, oid); t != nil {
		return t, nil
	}

	return w.tunnelManager.createTunnel(ctx, tp, oid, initCap, w.createTunnelFunc)
}

func (w *Worker) Push(ctx context.Context, out xnet.Pack) error {
	if w.OnStopping() {
		return errors.New("worker is stopping")
	}

	if len(out) == 0 {
		return errors.New("push msg len <= 0")
	}

	w.replyChan <- out

	return nil
}

const defaultWorkerTickInterval = time.Second * 10

func (w *Worker) tick(ctx context.Context) (err error) {
	interval := w.conf.TickInterval
	if interval <= 0 {
		interval = defaultWorkerTickInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.StopTriggered():
			return xsync.ErrStopByTrigger
		case <-ticker.C:
			if err = w.svc.Tick(ctx, w.session); err != nil {
				return err
			}
		}
	}
}

func (w *Worker) writeLoop(ctx context.Context) (err error) {
	w.replyChanStarted.Store(true)

	defer close(w.replyChanCompleted)

	for pack := range w.replyChan {
		if err = w.write(ctx, pack); err != nil {
			return err
		}
	}

	return nil
}

func writeNext(ctx context.Context, pk any) (any, error) {
	return pk, nil
}

func (w *Worker) write(ctx context.Context, pack xnet.Pack) (err error) {
	next := writeNext
	if w.writeFilter != nil {
		next = w.writeFilter(next)
	}

	out, err := next(ctx, pack)
	if err != nil {
		return
	}

	if out, err = w.session.Encrypt(out.(xnet.Pack)); err != nil {
		return err
	}

	return codec.Encode(w.writer, out.(xnet.Pack))
}

func (w *Worker) readLoop(ctx context.Context) (err error) {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.StopTriggered():
			return xsync.ErrStopByTrigger
		default:
			if err = w.read(ctx); err != nil {
				return err
			}
		}
	}
}
func (w *Worker) read(ctx context.Context) error {
	if w.OnStopping() {
		return xsync.ErrIsStopped
	}

	if err := w.Conn().SetDeadline(time.Now().Add(w.conf.RequestIdleTimeout)); err != nil {
		return errors.Wrap(err, "set conn deadline after handshake failed")
	}

	pack, free, err := codec.Decode(w.reader)
	if err != nil {
		return err
	}

	defer free()

	if pack, err = w.session.Decrypt(pack); err != nil {
		return err
	}

	next := func(ctx context.Context, req any) (any, error) {
		return nil, w.svc.Handle(ctx, w.session, w, req.(xnet.Pack))
	}

	if w.readFilter != nil {
		next = w.readFilter(next)
	}

	if _, err := next(ctx, pack); err != nil {
		return err
	}

	return nil
}

func (w *Worker) Stop(ctx context.Context) (err error) {
	if doCloseErr := w.TurnOff(ctx, func(ctx context.Context) {
		if w.Connected() {
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			if disConnErr := w.svc.OnDisconnect(ctx, w.session); disConnErr != nil {
				err = errors.Join(err, disConnErr)
			}
		}

		if stopErr := w.tunnelManager.stop(ctx); stopErr != nil {
			err = errors.Join(err, stopErr)
		}

		close(w.replyChan)

		// wait replyChanCompleted to ensure all replyChan are sent
		if w.replyChanStarted.Load() {
			<-w.replyChanCompleted
		}

		if writerErr := w.writer.Flush(); writerErr != nil {
			err = errors.Join(err, writerErr)
		}

		if connCloseErr := w.conn.Close(); connCloseErr != nil {
			err = errors.Join(err, connCloseErr)
			xcontext.SetDeadlineWithContext(ctx, w.conn, fmt.Sprintf("wid=%d", w.WID()))
		}
	}); doCloseErr != nil {
		err = errors.Join(err, doCloseErr)
	}

	return err
}

func (w *Worker) Connected() bool {
	return w.connected.Load()
}

func (w *Worker) Conn() net.Conn {
	return w.conn
}

func (w *Worker) WID() uint64 {
	return w.id
}

func (w *Worker) UID() int64 {
	if w.session == nil {
		return 0
	}

	return w.session.UID()
}

func (w *Worker) SID() int64 {
	if w.session == nil {
		return 0
	}

	return w.session.SID()
}

func (w *Worker) Color() string {
	if w.session == nil {
		return ""
	}

	return w.session.Color()
}

func (w *Worker) Status() int64 {
	if w.session == nil {
		return 0
	}

	return w.session.Status()
}

func (w *Worker) Endpoint() string {
	if w.conn == nil {
		return ""
	}

	return w.conn.RemoteAddr().String()
}
