package internal

import "net"

type ConnWrapper struct {
	ID    uint64
	Conn  net.Conn
	Codec Codec
}

func NewConnWrapper(id uint64, conn net.Conn, codec Codec) ConnWrapper {
	return ConnWrapper{
		ID:    id,
		Conn:  conn,
		Codec: codec,
	}
}

func (c ConnWrapper) Close() error {
	if c.Codec == nil {
		return nil
	}

	return c.Conn.Close()
}
