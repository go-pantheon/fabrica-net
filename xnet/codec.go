package xnet

import (
	"net"
)

type NewCodecFunc func(conn net.Conn) (Codec, error)

type Codec interface {
	Encode(pack Pack) error
	Decode() (pack Pack, free func(), err error)
}
