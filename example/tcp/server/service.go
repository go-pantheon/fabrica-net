package main

import (
	"context"
	"encoding/json"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/example/tcp/message"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
)

var _ xnet.Service = (*Service)(nil)

type Service struct {
}

func newService() *Service {
	return &Service{}
}

func (s *Service) Auth(ctx context.Context, in xnet.Pack) (out xnet.Pack, ss xnet.Session, err error) {
	p := &message.Packet{}

	if err = json.Unmarshal(in, p); err != nil {
		return nil, nil, errors.Wrap(err, "packet unmarshal failed")
	}

	if p.Mod != message.ModAuth {
		return nil, nil, errors.New("invalid auth mod")
	}

	log.Infof("[recv] auth %s", p)

	ss = xnet.NewSession(p.Obj, "", 0)

	p.Data = []byte("auth success")

	out, err = json.Marshal(p)
	if err != nil {
		return nil, nil, errors.Wrap(err, "packet marshal failed")
	}

	log.Infof("[send] auth %s", p)

	return
}

func (s *Service) Handle(ctx context.Context, ss xnet.Session, tm xnet.TunnelManager, in xnet.Pack) (err error) {
	p := &message.Packet{}

	if err = json.Unmarshal(in, p); err != nil {
		return errors.Wrap(err, "packet unmarshal failed")
	}

	t, err := tm.Tunnel(ctx, p.Mod, p.Obj)
	if err != nil {
		return errors.Wrap(err, "tunnel not found")
	}

	return t.Forward(ctx, p)
}

func (s *Service) TunnelType(mod int32) (t int32, initCapacity int, err error) {
	return message.ModEcho, 1, nil
}

func (s *Service) CreateAppTunnel(ctx context.Context, ss xnet.Session, tp int32, rid int64, w xnet.Worker) (t xnet.AppTunnel, err error) {
	return newEchoTunnel(ss), nil
}

func (s *Service) OnConnected(ctx context.Context, ss xnet.Session) (err error) {
	log.Debugf("on connected. uid=%d sid=%d color=%s status=%d ip=%s", ss.UID(), ss.SID(), ss.Color(), ss.Status(), ss.ClientIP())
	return nil
}

func (s *Service) OnDisconnect(ctx context.Context, ss xnet.Session) (err error) {
	log.Debugf("on disconnect. uid=%d sid=%d color=%s status=%d ip=%s", ss.UID(), ss.SID(), ss.Color(), ss.Status(), ss.ClientIP())
	return nil
}

func (s *Service) Tick(ctx context.Context, ss xnet.Session) (err error) {
	return nil
}
