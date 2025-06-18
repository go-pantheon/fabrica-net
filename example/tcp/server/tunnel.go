package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/example/tcp/message"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/xsync"
)

var _ xnet.AppTunnel = (*EchoTunnel)(nil)

type EchoTunnel struct {
	xsync.Stoppable
	xnet.AppBaseTunnel

	tp  int32
	log *log.Helper
	ss  xnet.Session

	scChan chan xnet.TunnelMessage
}

func newEchoTunnel(ss xnet.Session) *EchoTunnel {
	return &EchoTunnel{
		Stoppable: xsync.NewStopper(time.Second * 10),
		log:       log.NewHelper(log.With(log.DefaultLogger, "module", "app.tunnel")),
		tp:        message.ModEcho,
		ss:        ss,
		scChan:    make(chan xnet.TunnelMessage, 1024),
	}
}

func (t *EchoTunnel) CSHandle(msg xnet.TunnelMessage) error {
	t.log.Infof("[recv] echo %s", msg)

	sc := message.NewTunnelMessage(msg.GetMod(), msg.GetSeq(), msg.GetObj(), msg.GetIndex(), msg.GetData(), msg.GetDataVersion())
	sc.Data = []byte("Hello Bob!")

	t.scChan <- sc

	return nil
}

func (t *EchoTunnel) SCHandle() (xnet.TunnelMessage, error) {
	msg := <-t.scChan
	t.log.Infof("[send] echo %s", msg)

	return msg, nil
}

func (t *EchoTunnel) PacketToTunnelMsg(packet xnet.PacketMessage) (to xnet.TunnelMessage) {
	return message.NewTunnelMessage(packet.GetMod(), packet.GetSeq(), packet.GetObj(), packet.GetIndex(), packet.GetData(), packet.GetDataVersion())
}

func (t *EchoTunnel) TunnelMsgToPack(ctx context.Context, msg xnet.TunnelMessage) (xnet.Pack, error) {
	p := message.NewPacket(msg.GetMod(), msg.GetSeq(), msg.GetObj(), msg.GetIndex(), msg.GetData(), msg.GetDataVersion())

	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (t *EchoTunnel) OnStop(ctx context.Context, erreason error) (err error) {
	t.log.Infof("tunnel closed. %+v", erreason)

	return nil
}

func (t *EchoTunnel) Type() int32 {
	return t.tp
}

func (t *EchoTunnel) Log() *log.Helper {
	return t.log
}

func (t *EchoTunnel) UID() int64 {
	return t.ss.UID()
}

func (t *EchoTunnel) Session() xnet.Session {
	return t.ss
}

func (t *EchoTunnel) Color() string {
	return t.ss.Color()
}

func (t *EchoTunnel) OID() int64 {
	return t.ss.UID()
}
