package frame

import (
	"encoding/binary"
	"io"
	"sync"

	"github.com/go-pantheon/fabrica-net/codec"
	"github.com/go-pantheon/fabrica-net/internal/ringpool"
	"github.com/go-pantheon/fabrica-net/websocket/wsconn"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/gorilla/websocket"
)

var (
	once sync.Once
	pool ringpool.Pool

	ErrShortRead      = errors.New("short read")
	ErrShortWrite     = errors.New("short write")
	ErrInvalidPackLen = errors.New("invalid pack len")
)

func init() {
	p, err := ringpool.Default()
	if err != nil {
		panic("failed to initialize websocket ring pool: " + err.Error())
	}

	pool = p
}

func InitWebSocketRingPool(sizeCapacityMap map[int]uint64) (err error) {
	once.Do(func() {
		pool, err = ringpool.NewMultiSizeRingPool(sizeCapacityMap)
		if err != nil {
			return
		}
	})

	return nil
}

var _ codec.Codec = (*Codec)(nil)

type Codec struct {
	conn *wsconn.WebSocketConn
}

func New(conn *wsconn.WebSocketConn) codec.Codec {
	return &Codec{
		conn: conn,
	}
}

func (c *Codec) Encode(pack xnet.Pack) (err error) {
	w, err := c.conn.NextWriter(wsconn.MessageType)
	if err != nil {
		return errors.Wrap(err, "create next writer failed")
	}

	defer func() {
		if closeErr := w.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	packLen := int32(len(pack))
	if err := binary.Write(w, binary.BigEndian, packLen); err != nil {
		return errors.Wrap(err, "write pack len failed")
	}

	n, err := w.Write(pack)
	if err != nil {
		return errors.Wrap(err, "write pack failed")
	}

	if n != len(pack) {
		return ErrShortWrite
	}

	return nil
}

func (c *Codec) Decode() (pack xnet.Pack, free func(), err error) {
	mt, r, err := c.conn.NextReader()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
			return nil, nil, errors.Wrap(err, "unexpected close error")
		}

		return nil, nil, errors.Wrap(err, "connection closed")
	}

	if mt != wsconn.MessageType {
		return nil, nil, wsconn.ErrInvalidFrameType
	}

	var packLen int32
	if err := binary.Read(r, binary.BigEndian, &packLen); err != nil {
		return nil, nil, errors.Wrap(err, "read pack len failed")
	}

	if packLen <= 0 || packLen > xnet.MaxPackSize {
		return nil, nil, ErrInvalidPackLen
	}

	buf := pool.Alloc(int(packLen))
	free = func() {
		pool.Free(buf)
	}

	n, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, nil, errors.Wrap(err, "read pack failed")
	}

	if n < int(packLen) {
		return nil, nil, ErrShortRead
	}

	return xnet.Pack(buf), free, nil
}
