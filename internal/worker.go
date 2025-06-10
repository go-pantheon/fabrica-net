package internal

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal/bufreader"
	"github.com/go-pantheon/fabrica-net/xcontext"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
	"golang.org/x/sync/errgroup"
)

var _ xnet.Worker = (*Worker)(nil)

type Worker struct {
	xsync.Closable

	id      uint64
	conn    *net.TCPConn
	reader  *bufreader.Reader
	started atomic.Bool
	session xnet.Session

	tunnelManager *tunnelManager

	conf             conf.Worker
	svc              xnet.Service
	createTunnelFunc CreateTunnelFunc
	delyClosure      xsync.Delayable
	referer          string

	readFilter  middleware.Middleware
	writeFilter middleware.Middleware

	replyChanStarted   atomic.Bool
	replyChanCompleted chan struct{}
	replyChan          chan []byte
}

func NewWorker(wid uint64, conn *net.TCPConn, logger log.Logger, conf conf.Worker, referer string,
	readFilter, writeFilter middleware.Middleware, svc xnet.Service) *Worker {
	w := &Worker{
		Closable:           xsync.NewClosure(conf.StopTimeout),
		delyClosure:        xsync.NewDelayer(),
		tunnelManager:      newTunnelManager(conf.TunnelGroupSize),
		id:                 wid,
		conn:               conn,
		conf:               conf,
		svc:                svc,
		referer:            referer,
		readFilter:         readFilter,
		writeFilter:        writeFilter,
		replyChanCompleted: make(chan struct{}),
	}

	w.createTunnelFunc = func(ctx context.Context, tp int32, oid int64) (xnet.Tunnel, error) {
		return w.svc.CreateTunnel(ctx, w.session, tp, oid, w)
	}

	w.session = xnet.DefaultSession()
	w.replyChan = make(chan []byte, conf.ReplyChanSize)
	w.reader = bufreader.NewReader(conn, conf.ReaderBufSize)

	return w
}

func (w *Worker) Start(ctx context.Context) (err error) {
	if err = w.handshake(ctx); err != nil {
		return err
	}

	if err = w.svc.OnConnected(ctx, w.session); err != nil {
		return err
	}

	w.started.Store(true)

	return
}

// handshake must only be used in auth
func (w *Worker) handshake(ctx context.Context) error {
	var (
		ss  xnet.Session
		in  []byte
		out []byte
		err error
	)

	if err = w.Conn().SetDeadline(time.Now().Add(w.conf.HandshakeTimeout)); err != nil {
		return errors.Wrap(err, "set conn deadline before handshake failed")
	}

	if in, err = w.read(); err != nil {
		return err
	}

	if out, ss, err = w.svc.Auth(ctx, in); err != nil {
		return err
	}

	if err = w.write(out); err != nil {
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
		case <-w.CloseTriggered():
			return xsync.ErrGroupIsClosing
		case <-w.delyClosure.Wait():
			return errors.Wrapf(xsync.ErrDelayerExpired, "wid=%d", w.WID())
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	eg.Go(func() error {
		err := xsync.RunSafe(func() error {
			return w.writePackLoop(ctx)
		})

		return err
	})
	eg.Go(func() error {
		err := xsync.RunSafe(func() error {
			return w.readPackLoop(ctx)
		})

		return err
	})
	eg.Go(func() error {
		err := w.tick(ctx)
		return err
	})

	if err := eg.Wait(); err != nil {
		if errors.Is(err, xsync.ErrGroupIsClosing) {
			return nil
		}
		return err
	}

	return nil
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

func (w *Worker) Push(ctx context.Context, out []byte) error {
	if w.OnClosing() {
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
		case <-ticker.C:
			if err = w.svc.Tick(ctx, w.session); err != nil {
				return err
			}
		}
	}
}

func (w *Worker) writePackLoop(ctx context.Context) (err error) {
	defer close(w.replyChanCompleted)

	w.replyChanStarted.Store(true)

	for pack := range w.replyChan {
		if err = w.writePack(ctx, pack); err != nil {
			return err
		}
	}

	return nil
}

func writeNext(ctx context.Context, pk any) (any, error) {
	return pk, nil
}

func (w *Worker) writePack(ctx context.Context, pack []byte) (err error) {
	next := writeNext
	if w.writeFilter != nil {
		next = w.writeFilter(next)
	}

	var out any

	if out, err = next(ctx, pack); err != nil {
		return
	}

	return w.write(out.([]byte))
}

func (w *Worker) write(pack []byte) error {
	pack, err := w.session.Encrypt(pack)
	if err != nil {
		return err
	}

	packLen := int32(len(pack))

	var buf bytes.Buffer

	if err = binary.Write(&buf, binary.BigEndian, packLen); err != nil {
		return errors.Wrap(err, "write packet length failed")
	}

	if _, err = buf.Write(pack); err != nil {
		return errors.Wrap(err, "write packet body failed")
	}

	if _, err = w.conn.Write(buf.Bytes()); err != nil {
		return errors.Wrapf(err, "write packet failed. len=%d", packLen)
	}

	return nil
}

func (w *Worker) readPackLoop(ctx context.Context) (err error) {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.ClosingStart():
			return nil
		default:
			if err = w.readPack(ctx); err != nil {
				return err
			}
		}
	}
}
func (w *Worker) readPack(ctx context.Context) error {
	if err := w.Conn().SetDeadline(time.Now().Add(w.conf.RequestIdleTimeout)); err != nil {
		return errors.Wrap(err, "set conn deadline after handshake failed")
	}

	in, err := w.read()
	if err != nil {
		return err
	}

	next := func(ctx context.Context, req any) (any, error) {
		return w.handle(ctx, req)
	}

	if w.readFilter != nil {
		next = w.readFilter(next)
	}

	if _, err := next(ctx, in); err != nil {
		return err
	}

	return nil
}

func (w *Worker) read() ([]byte, error) {
	lenBytes, err := w.reader.ReadFull(xnet.PackLenSize)
	if err != nil {
		return nil, errors.Wrap(err, "read packet length failed")
	}

	var packLen int32
	if err := binary.Read(bytes.NewReader(lenBytes), binary.BigEndian, &packLen); err != nil {
		return nil, errors.Wrap(err, "read packet length failed")
	}

	if packLen <= 0 {
		return nil, errors.New("packet len must greater than 0")
	}

	if packLen > xnet.MaxBodySize {
		return nil, errors.Errorf("packet len=%d must less than %d", packLen, xnet.MaxBodySize)
	}

	buf, err := w.reader.ReadFull(int(packLen))
	if err != nil {
		return nil, errors.Wrapf(err, "read packet body failed. len=%d", packLen)
	}

	if buf, err = w.session.Decrypt(buf); err != nil {
		return nil, err
	}

	return buf, nil
}

func (w *Worker) handle(ctx context.Context, req any) (any, error) {
	return nil, w.svc.Handle(ctx, w.session, w, req.([]byte))
}

func (w *Worker) Close(ctx context.Context) (err error) {
	if doCloseErr := w.DoClose(func() {
		if w.IsStarted() {
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			if disConnErr := w.svc.OnDisconnect(ctx, w.session); disConnErr != nil {
				err = errors.Join(err, disConnErr)
			}
		}

		if readerCloseErr := w.reader.Close(); readerCloseErr != nil {
			err = errors.Join(err, readerCloseErr)
		}

		w.tunnelManager.close()
		w.delyClosure.Close()

		close(w.replyChan)

		if w.replyChanStarted.Load() {
			<-w.replyChanCompleted
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

func (w *Worker) IsStarted() bool {
	return w.started.Load()
}

// SetStopCountDownTime Pass in the current time to set the worker shutdown countdown when the main tunnel is disconnected
func (w *Worker) SetExpiryTime(now time.Time) {
	w.delyClosure.SetExpiryTime(now.Add(w.conf.WaitMainTunnelTimeout))
}

func (w *Worker) ExpiryTime() time.Time {
	return w.delyClosure.ExpiryTime()
}

func (w *Worker) Reset() {
	w.delyClosure.Reset()
}

func (w *Worker) Conn() *net.TCPConn {
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
