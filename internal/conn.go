package internal

import (
	"net"

	"github.com/go-pantheon/fabrica-net/xnet"
)

type ConnCarrier struct {
	ID    uint64
	Conn  net.Conn
	Codec xnet.Codec
}

func NewConnCarrier(id uint64, conn net.Conn, codec xnet.Codec) ConnCarrier {
	return ConnCarrier{
		ID:    id,
		Conn:  conn,
		Codec: codec,
	}
}

func (c ConnCarrier) Close() error {
	if c.Codec == nil {
		return nil
	}

	return c.Conn.Close()
}
