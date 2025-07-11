package internal

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/codec"
	"github.com/go-pantheon/fabrica-net/internal/util"
	server "github.com/go-pantheon/fabrica-net/server"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/go-pantheon/fabrica-util/xsync"
)

const (
	stopTimeout = time.Second * 30
)

var _ xnet.Server = (*BaseServer)(nil)

type BaseServer struct {
	xsync.Stoppable
	*server.Options

	workerSize    int
	workerManager *WorkerManager

	listener     Listener
	newCodecFunc codec.NewCodecFunc

	service xnet.Service
}

func NewBaseServer(listener Listener, svc xnet.Service, newCodecFunc codec.NewCodecFunc, options *server.Options) (*BaseServer, error) {
	if options == nil {
		options = server.NewOptions()
	}

	s := &BaseServer{
		Stoppable:    xsync.NewStopper(stopTimeout),
		Options:      options,
		service:      svc,
		listener:     listener,
		newCodecFunc: newCodecFunc,
	}

	s.workerManager = newWorkerManager(s.Conf().Bucket)
	s.workerSize = s.Conf().Server.WorkerSize

	if err := s.listener.Start(context.Background()); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *BaseServer) Start(ctx context.Context) error {
	for i := range s.workerSize {
		aid := i
		s.GoAndStop(fmt.Sprintf("BaseServer.acceptLoop-%d", aid), func() error {
			return s.acceptLoop(ctx)
		}, func() error {
			return s.Stop(ctx)
		})
	}

	return nil
}

func (s *BaseServer) acceptLoop(ctx context.Context) error {
	for {
		select {
		case <-s.StopTriggered():
			return xsync.ErrStopByTrigger
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := s.accept(ctx); err != nil {
				log.Errorf("[BaseServer] %+v", err)
			}
		}
	}
}

func (s *BaseServer) accept(ctx context.Context) error {
	conn, wid, err := s.listener.Accept(ctx, nil)
	if err != nil {
		return errors.Wrapf(err, "accept failed")
	}

	xsync.Go(fmt.Sprintf("BaseServer.serve.%d", wid), func() error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		return s.work(ctx, wid, conn)
	})

	return nil
}

func (s *BaseServer) work(ctx context.Context, wid uint64, conn net.Conn) (err error) {
	codec, err := s.newCodecFunc(conn)
	if err != nil {
		return err
	}

	w := newWorker(wid, conn, s.Conf().Worker, s.Referer(), codec,
		s.ReadFilter(), s.WriteFilter(), s.service)

	defer func() {
		if closeErr := w.Stop(ctx); closeErr != nil {
			err = errors.Join(err, closeErr)
		}

		if aferr := s.AfterDisconnect()(server.EmptyInspectorFunc)(ctx, w); aferr != nil {
			err = errors.Join(err, aferr)
		}

		if err != nil {
			err = errors.WithMessagef(err, "wid=%d uid=%d color=%s status=%d remote-addr=%s local-addr=%s",
				w.WID(), w.UID(), w.Color(), w.Status(),
				util.RemoteAddr(conn), util.LocalAddr(conn))
		}
	}()

	if err := w.Start(ctx); err != nil {
		return err
	}

	if err := s.addWorker(ctx, w); err != nil {
		return err
	}

	defer s.delWorker(w.WID())

	if err := s.AfterConnect()(server.EmptyInspectorFunc)(ctx, w); err != nil {
		return err
	}

	return w.Run(ctx)
}

func (s *BaseServer) addWorker(ctx context.Context, w *Worker) (err error) {
	if ow := s.workerManager.Put(w); ow != nil {
		err = ow.Stop(ctx)
	}

	return err
}

func (s *BaseServer) delWorker(wid uint64) {
	s.workerManager.Del(wid)
}

func (s *BaseServer) Stop(ctx context.Context) (err error) {
	return s.TurnOff(func() error {
		wg := sync.WaitGroup{}

		s.workerManager.Walk(func(w *Worker) (continued bool) {
			wg.Add(1)

			xsync.Go(fmt.Sprintf("BaseServer.stopWorker.%d", w.WID()), func() error {
				defer wg.Done()
				return w.Stop(ctx)
			})

			return true
		})

		wg.Wait()

		if s.listener != nil {
			if stopErr := s.listener.Stop(ctx); stopErr != nil {
				err = errors.Join(err, stopErr)
			}
		}

		log.Infof("[BaseServer] stopped.")

		return err
	})
}

func (s *BaseServer) Disconnect(ctx context.Context, wid uint64) error {
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

func (s *BaseServer) WIDList() []uint64 {
	if s.OnStopping() {
		return nil
	}

	ids := make([]uint64, 0, 65535)

	s.workerManager.Walk(func(w *Worker) bool {
		ids = append(ids, w.WID())
		return true
	})

	return ids
}

func (s *BaseServer) Push(ctx context.Context, uid int64, pack xnet.Pack) error {
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

func (s *BaseServer) Multicast(ctx context.Context, uids []int64, pack xnet.Pack) (err error) {
	if s.OnStopping() {
		return xsync.ErrIsStopped
	}

	if len(pack) == 0 {
		return errors.New("multicast msg len <= 0")
	}

	workers := s.workerManager.GetByUIDs(uids)
	for _, w := range workers {
		if pusherr := w.Push(ctx, pack); pusherr != nil {
			err = errors.JoinUnsimilar(err, pusherr)
		}
	}

	return nil
}

func (s *BaseServer) Broadcast(ctx context.Context, pack xnet.Pack) (err error) {
	if s.OnStopping() {
		return xsync.ErrIsStopped
	}

	if len(pack) == 0 {
		return errors.New("broadcast msg len <= 0")
	}

	s.workerManager.Walk(func(w *Worker) bool {
		if pusherr := w.Push(ctx, pack); pusherr != nil {
			err = errors.JoinUnsimilar(err, pusherr)
		}

		return true
	})

	return nil
}

func (s *BaseServer) Endpoint() (*url.URL, error) {
	endpointStr, err := s.listener.Endpoint()
	if err != nil {
		return nil, err
	}

	return url.Parse(endpointStr)
}
