package tcp

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

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

type Option func(o *Server)

type WrapperFunc func(ctx context.Context, uid int64, color string) error

func Bind(bind string) Option {
	return func(s *Server) {
		s.bind = bind
	}
}

func WithConf(conf conf.Config) Option {
	return func(s *Server) {
		s.conf = conf
	}
}

func Referer(referer string) Option {
	return func(s *Server) {
		s.referer = referer
	}
}

func Logger(logger log.Logger) Option {
	return func(s *Server) {
		s.logger = logger
	}
}

func ReadFilter(m middleware.Middleware) Option {
	return func(s *Server) {
		if s.readFilter == nil {
			s.readFilter = m
			return
		}

		s.readFilter = middleware.Chain(s.readFilter, m)
	}
}

func WriteFilter(m middleware.Middleware) Option {
	return func(s *Server) {
		if s.writeFilter == nil {
			s.writeFilter = m
			return
		}

		s.writeFilter = middleware.Chain(s.writeFilter, m)
	}
}

func AfterConnectFunc(f WrapperFunc) Option {
	return func(s *Server) {
		s.afterConnectFunc = f
	}
}

func AfterDisconnectFunc(f WrapperFunc) Option {
	return func(s *Server) {
		s.afterDisconnectFunc = f
	}
}

const (
	stopTimeout = time.Second * 30
)

var _ transport.Server = (*Server)(nil)

type Server struct {
	xsync.Stoppable

	bind    string
	conf    conf.Config
	logger  log.Logger
	referer string

	workerSize    int
	workerManager *internal.WorkerManager

	listener net.Listener

	service     xnet.Service
	readFilter  middleware.Middleware
	writeFilter middleware.Middleware

	afterConnectFunc    WrapperFunc
	afterDisconnectFunc WrapperFunc
}

func NewServer(bind string, svc xnet.Service, opts ...Option) (*Server, error) {
	s := &Server{
		Stoppable: xsync.NewStopper(stopTimeout),
		logger:    log.DefaultLogger,
		bind:      bind,
		conf:      conf.Default(),
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

	s.workerManager = internal.NewWorkerManager(s.conf.Bucket)
	s.workerSize = s.conf.Server.WorkerSize
	s.afterConnectFunc = func(ctx context.Context, uid int64, color string) error {
		return nil
	}
	s.afterDisconnectFunc = func(ctx context.Context, uid int64, color string) error {
		return nil
	}

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	var (
		listener *net.TCPListener
		addr     *net.TCPAddr
		err      error
	)

	if addr, err = net.ResolveTCPAddr("tcp", s.bind); err != nil {
		err = errors.Wrapf(err, "resolve bind failed. bind=%s", s.bind)
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
		s.GoAndQuickStop(fmt.Sprintf("tcp.Server.acceptLoop.%d", aid), func() error {
			return s.acceptLoop(ctx, widGener)
		}, func() error {
			return s.Stop(ctx)
		})
	}

	log.Infof("[tcp.Server] started. Listening on %s", addr.String())

	return nil
}

func (s *Server) acceptLoop(ctx context.Context, widGener *atomic.Uint64) error {
	for {
		select {
		case <-s.StopTriggered():
			return xsync.ErrStopByTrigger
		case <-ctx.Done():
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

	xsync.Go(fmt.Sprintf("tcp.Server.serve.%d", wid), func() error {
		return s.serve(ctx, conn, wid)
	})

	return nil
}

func (s *Server) serve(ctx context.Context, conn *net.TCPConn, wid uint64) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
		if closeErr := w.Stop(ctx); closeErr != nil {
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

	if err = s.addWorker(ctx, w); err != nil {
		return err
	}

	defer s.delWorker(w.WID())

	if err = s.afterConnectFunc(ctx, w.UID(), w.Color()); err != nil {
		return err
	}

	return w.Run(ctx)
}

func (s *Server) addWorker(ctx context.Context, w *internal.Worker) (err error) {
	if ow := s.workerManager.Put(w); ow != nil {
		err = ow.Stop(ctx)
	}

	return err
}

func (s *Server) delWorker(wid uint64) {
	s.workerManager.Del(wid)
}

func (s *Server) Stop(ctx context.Context) (err error) {
	return s.TurnOff(func() error {
		s.workerManager.Walk(func(w *internal.Worker) (continued bool) {
			if stopErr := w.Stop(ctx); stopErr != nil {
				err = errors.JoinUnsimilar(err, stopErr)
			}

			return true
		})

		log.Infof("[tcp.Server] stopped.")

		return err
	})
}

func (s *Server) Disconnect(ctx context.Context, wid uint64) error {
	if s.OnStopping() {
		return xsync.ErrIsStopped
	}

	w := s.workerManager.Worker(wid)
	if w == nil {
		return errors.New("worker not found")
	}

	s.delWorker(wid)

	if closeErr := w.Stop(ctx); closeErr != nil {
		return closeErr
	}

	return nil
}

func (s *Server) WIDList() []uint64 {
	if s.OnStopping() {
		return nil
	}

	ids := make([]uint64, 0, 65535)

	s.workerManager.Walk(func(w *internal.Worker) bool {
		ids = append(ids, w.WID())
		return true
	})

	return ids
}

func (s *Server) Push(ctx context.Context, uid int64, pack []byte) error {
	if s.OnStopping() {
		return xsync.ErrIsStopped
	}

	if len(pack) == 0 {
		return errors.New("push msg len <= 0")
	}

	w := s.workerManager.GetByUID(uid)
	if w == nil {
		return errors.New("worker not found")
	}

	return w.Push(ctx, pack)
}

func (s *Server) BatchPush(ctx context.Context, uids []int64, pack []byte) (err error) {
	if s.OnStopping() {
		return xsync.ErrIsStopped
	}

	if len(pack) == 0 {
		return errors.New("push group msg len <= 0")
	}

	workers := s.workerManager.GetByUIDs(uids)
	for _, w := range workers {
		if pusherr := w.Push(ctx, pack); pusherr != nil {
			err = errors.JoinUnsimilar(err, pusherr)
		}
	}

	return nil
}

func (s *Server) Broadcast(ctx context.Context, pack []byte) (err error) {
	if s.OnStopping() {
		return xsync.ErrIsStopped
	}

	if len(pack) == 0 {
		return errors.New("broadcast msg len <= 0")
	}

	s.workerManager.Walk(func(w *internal.Worker) bool {
		if pusherr := w.Push(ctx, pack); pusherr != nil {
			err = errors.JoinUnsimilar(err, pusherr)
		}

		return true
	})

	return nil
}

func (s *Server) Endpoint() (string, error) {
	addr, err := ip.Extract(s.bind, s.listener)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("tcp://%s", addr), nil
}
