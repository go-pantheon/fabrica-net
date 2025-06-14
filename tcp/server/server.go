package tcp

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/internal/ip"
	"github.com/go-pantheon/fabrica-net/xcontext"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
)

// Option is a function that configures the server.
type Option func(o *Server)

// WrapperFunc is a function that is called after a connection is established or closed.
type WrapperFunc func(ctx context.Context, uid int64, color string) error

// Bind sets the bind address for the server.
func Bind(bind string) Option {
	return func(s *Server) {
		s.conf.Server.Bind = bind
	}
}

// Referer sets the referer for the server.
func Referer(referer string) Option {
	return func(s *Server) {
		s.referer = referer
	}
}

// Logger sets the logger for the server.
func Logger(logger log.Logger) Option {
	return func(s *Server) {
		s.logger = logger
	}
}

// ReadFilter sets the read filter for the server.
func ReadFilter(m middleware.Middleware) Option {
	return func(s *Server) {
		if s.readFilter == nil {
			s.readFilter = m
			return
		}

		s.readFilter = middleware.Chain(s.readFilter, m)
	}
}

// WriteFilter sets the write filter for the server.
func WriteFilter(m middleware.Middleware) Option {
	return func(s *Server) {
		if s.writeFilter == nil {
			s.writeFilter = m
			return
		}

		s.writeFilter = middleware.Chain(s.writeFilter, m)
	}
}

// AfterConnectFunc sets the after connect function for the server.
func AfterConnectFunc(f WrapperFunc) Option {
	return func(s *Server) {
		s.afterConnectFunc = f
	}
}

// AfterDisconnectFunc sets the after disconnect function for the server.
func AfterDisconnectFunc(f WrapperFunc) Option {
	return func(s *Server) {
		s.afterDisconnectFunc = f
	}
}

var _ transport.Server = (*Server)(nil)

// Server is a TCP server.
type Server struct {
	xsync.Closable

	conf    conf.Config
	logger  log.Logger
	referer string

	workerSize int
	manager    *internal.WorkerManager

	listener net.Listener

	service     xnet.Service
	readFilter  middleware.Middleware
	writeFilter middleware.Middleware

	afterConnectFunc    WrapperFunc
	afterDisconnectFunc WrapperFunc
}

func NewServer(svc xnet.Service, opts ...Option) (*Server, error) {
	s := &Server{
		Closable: xsync.NewClosure(conf.Conf.Server.StopTimeout),
		logger:   log.DefaultLogger,
		conf:     conf.Conf,
		readFilter: middleware.Chain(
			recovery.Recovery(),
		),
		writeFilter: middleware.Chain(
			recovery.Recovery(),
		),
		service: svc,
	}

	for _, o := range opts {
		o(s)
	}

	s.manager = internal.NewWorkerManager(s.conf.Bucket)
	s.workerSize = s.conf.Server.WorkerSize
	s.afterConnectFunc = func(ctx context.Context, uid int64, color string) error {
		return nil
	}
	s.afterDisconnectFunc = func(ctx context.Context, uid int64, color string) error {
		return nil
	}

	return s, nil
}

// Start starts the server.
func (s *Server) Start(ctx context.Context) error {
	var (
		listener *net.TCPListener
		addr     *net.TCPAddr
		err      error
	)

	bind := s.conf.Server.Bind

	if addr, err = net.ResolveTCPAddr("tcp", bind); err != nil {
		err = errors.Wrapf(err, "resolve bind failed. bind=%s", bind)
		return err
	}

	if listener, err = net.ListenTCP("tcp", addr); err != nil {
		err = errors.Wrapf(err, "listen failed. addr=%s", addr.String())
		return err
	}

	xcontext.SetDeadlineWithContext(ctx, listener, "TcpListener")

	s.listener = listener
	widGener := &atomic.Uint64{}

	for i := range s.workerSize {
		aid := i
		xsync.GoSafe(fmt.Sprintf("tcp.Server.acceptLoop.%d", aid), func() error {
			return s.acceptLoop(ctx, widGener)
		})
	}

	log.Infof("[tcp.Server] listening on %s", addr.String())

	return nil
}

func (s *Server) acceptLoop(ctx context.Context, widGener *atomic.Uint64) error {
	for {
		select {
		case <-s.ClosingStart():
			s.WaitClosed()
			return ctx.Err()
		case <-ctx.Done():
			s.WaitClosed()
			return ctx.Err()
		default:
			if err := s.accept(ctx, widGener); err != nil {
				log.Errorf("[tcp.Server] %+v", err)
			}
		}
	}
}

func (s *Server) accept(ctx context.Context, widGener *atomic.Uint64) error {
	conn, err := s.listener.(*net.TCPListener).AcceptTCP()
	if err != nil {
		return errors.Wrapf(err, "accept failed")
	}

	wid := widGener.Add(1)

	xsync.GoSafe(fmt.Sprintf("tcp.Server.serve.%d", wid), func() error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		return s.serve(ctx, conn, wid)
	})

	return nil
}

func (s *Server) serve(ctx context.Context, conn *net.TCPConn, wid uint64) error {
	if err := conn.SetKeepAlive(s.conf.Server.KeepAlive); err != nil {
		return errors.Wrapf(err, "SetKeepAlive failed v=%v	", s.conf.Server.KeepAlive)
	}

	if err := conn.SetReadBuffer(s.conf.Server.ReadBufSize); err != nil {
		return errors.Wrapf(err, "SetReadBuffer failed v=%d", s.conf.Server.ReadBufSize)
	}

	if err := conn.SetWriteBuffer(s.conf.Server.WriteBufSize); err != nil {
		return errors.Wrapf(err, "SetWriteBuffer failed v=%d", s.conf.Server.WriteBufSize)
	}

	return s.work(ctx, conn, wid)
}

func (s *Server) work(ctx context.Context, conn *net.TCPConn, wid uint64) (err error) {
	w := internal.NewWorker(wid, conn, s.logger, s.conf.Worker, s.referer, s.readFilter, s.writeFilter, s.service)

	defer func() {
		if closeErr := w.Close(ctx); closeErr != nil {
			err = errors.Join(err, closeErr)
		}

		if aferr := s.afterDisconnectFunc(ctx, w.UID(), w.Color()); aferr != nil {
			err = errors.Join(err, aferr)
		}

		if err != nil {
			err = errors.WithMessagef(err, "wid=%d uid=%d color=%s status=%d remote-addr=%s local-addr=%s",
				w.WID(), w.UID(), w.Color(), w.Status(),
				xcontext.RemoteAddr(conn), xcontext.LocalAddr(conn))
		}
	}()

	if err = w.Start(ctx); err != nil {
		return err
	}

	s.addWorker(w)
	defer s.delWorker(w)

	if err = s.afterConnectFunc(ctx, w.UID(), w.Color()); err != nil {
		return err
	}

	return w.Run(ctx)
}

func (s *Server) addWorker(w *internal.Worker) {
	if ow := s.manager.Put(w); ow != nil {
		log.Errorf("[tcp.Server] addWorker failed, worker is replaced. wid=%d uid=%d color=%s",
			ow.WID(), ow.UID(), ow.Color())
		ow.TriggerClose()
	}
}

func (s *Server) delWorker(w *internal.Worker) {
	s.manager.Del(w)
}

func (s *Server) Stop(ctx context.Context) (err error) {
	if doCloseErr := s.DoClose(func() {
		s.manager.Walk(func(w *internal.Worker) (continued bool) {
			w.TriggerClose()
			return true
		})

		s.manager.Walk(func(w *internal.Worker) (continued bool) {
			w.WaitClosed()
			return true
		})
	}); doCloseErr != nil {
		err = errors.Join(err, doCloseErr)
	}

	log.Info("[tcp.Server] TCP server is closed")

	return err
}

func (s *Server) Disconnect(ctx context.Context, wid uint64) error {
	w := s.manager.Worker(wid)
	if w == nil {
		return errors.New("worker not found")
	}

	w.TriggerClose()
	w.WaitClosed()

	return nil
}

func (s *Server) WIDList() []uint64 {
	ids := make([]uint64, 0, 65535)

	s.manager.Walk(func(w *internal.Worker) bool {
		ids = append(ids, w.WID())
		return true
	})

	return ids
}

func (s *Server) Push(ctx context.Context, uid int64, pack []byte) error {
	if len(pack) == 0 {
		return errors.New("push msg len <= 0")
	}

	w := s.manager.GetByUID(uid)
	if w == nil {
		return errors.New("worker not found")
	}

	return w.Push(ctx, pack)
}

func (s *Server) BatchPush(ctx context.Context, uids []int64, pack []byte) (err error) {
	if len(pack) == 0 {
		return errors.New("push group msg len <= 0")
	}

	workers := s.manager.GetByUIDs(uids)
	for _, w := range workers {
		if pusherr := w.Push(ctx, pack); pusherr != nil {
			err = errors.JoinUnsimilar(err, pusherr)
		}
	}

	return nil
}

func (s *Server) Broadcast(ctx context.Context, pack []byte) (err error) {
	if len(pack) == 0 {
		return errors.New("broadcast msg len <= 0")
	}

	s.manager.Walk(func(w *internal.Worker) bool {
		if pusherr := w.Push(ctx, pack); pusherr != nil {
			err = errors.JoinUnsimilar(err, pusherr)
		}

		return true
	})

	return nil
}

func (s *Server) Endpoint() (string, error) {
	addr, err := ip.Extract(s.conf.Server.Bind, s.listener)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("tcp://%s", addr), nil
}
