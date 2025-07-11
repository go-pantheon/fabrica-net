package codec

import (
	"net"

	"github.com/go-pantheon/fabrica-net/xnet"
)

type NewCodecFunc func(conn net.Conn) (Codec, error)

type Codec interface {
	Encode(pack xnet.Pack) error
	Decode() (pack xnet.Pack, free func(), err error)
}
