package frame

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/go-pantheon/fabrica-net/internal/ringpool"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
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
		panic("failed to initialize tcp ring pool: " + err.Error())
	}

	pool = p
}

func InitTcpRingPool(sizeCapacityMap map[int]uint64) (err error) {
	once.Do(func() {
		pool, err = ringpool.NewMultiSizeRingPool(sizeCapacityMap)
		if err != nil {
			return
		}
	})

	return nil
}

var _ xnet.Codec = (*Codec)(nil)

type Codec struct {
	w *bufio.Writer
	r *bufio.Reader
}

func New(conn net.Conn) xnet.Codec {
	return &Codec{
		w: bufio.NewWriter(conn),
		r: bufio.NewReader(conn),
	}
}

func (c *Codec) Encode(pack xnet.Pack) error {
	if err := binary.Write(c.w, binary.BigEndian, int32(len(pack))); err != nil {
		return errors.Wrap(err, "write pack len failed")
	}

	n, err := c.w.Write(pack)
	if err != nil {
		return errors.Wrapf(err, "write pack failed")
	}

	if n != len(pack) {
		return ErrShortWrite
	}

	if err := c.w.Flush(); err != nil {
		return errors.Wrap(err, "flush writer failed")
	}

	return nil
}

func (c *Codec) Decode() (pack xnet.Pack, free func(), err error) {
	var packLen int32
	if err := binary.Read(c.r, binary.BigEndian, &packLen); err != nil {
		return nil, nil, errors.Wrap(err, "read pack len failed")
	}

	if packLen <= 0 || packLen > xnet.MaxPackSize {
		return nil, nil, ErrInvalidPackLen
	}

	buf := pool.Alloc(int(packLen))
	free = func() {
		pool.Free(buf)
	}

	defer func() {
		if err != nil {
			free()
		}
	}()

	n, err := io.ReadFull(c.r, buf)
	if err != nil {
		return nil, nil, errors.Wrap(err, "read pack failed")
	}

	if n < int(packLen) {
		return nil, nil, ErrShortRead
	}

	return xnet.Pack(buf), free, nil
}
