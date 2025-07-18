package internal

import "net"

type ConnCarrier struct {
	ID    uint64
	Conn  net.Conn
	Codec Codec
}

func NewConnCarrier(id uint64, conn net.Conn, codec Codec) ConnCarrier {
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
