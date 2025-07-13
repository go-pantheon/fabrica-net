package frame

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/go-pantheon/fabrica-net/codec"
	"github.com/go-pantheon/fabrica-net/internal/ringpool"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
)

var (
	once sync.Once
	pool ringpool.Pool
)

func init() {
	sizeCapacityMap := map[int]uint64{
		64:   8192 * 2, // 64bytes * 8192 * 2 = 1MB
		128:  8192,     // 128bytes * 8192 = 1MB
		256:  4096,     // 256bytes * 4096 = 1MB
		512:  2048,     // 512bytes * 2048 = 1MB
		1024: 1024,     // 1kb * 1024 = 1MB
		4096: 256,      // 4kb * 256 = 1MB
	}

	p, err := ringpool.NewMultiSizeRingPool(sizeCapacityMap)
	if err != nil {
		panic("failed to initialize kcp ring pool: " + err.Error())
	}

	pool = p
}

func InitKcpRingPool(sizeCapacityMap map[int]uint64) (err error) {
	once.Do(func() {
		pool, err = ringpool.NewMultiSizeRingPool(sizeCapacityMap)
		if err != nil {
			return
		}
	})

	return nil
}

var _ codec.Codec = (*kcpCodec)(nil)

type kcpCodec struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func New(conn net.Conn) codec.Codec {
	return &kcpCodec{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}
}

func (c *kcpCodec) Encode(pack xnet.Pack) error {
	if err := binary.Write(c.writer, binary.BigEndian, int32(len(pack))); err != nil {
		return errors.Wrap(err, "write length failed")
	}

	if _, err := c.writer.Write(pack); err != nil {
		return errors.Wrap(err, "write data failed")
	}

	if err := c.writer.Flush(); err != nil {
		return errors.Wrap(err, "flush writer failed")
	}

	return nil
}

func (c *kcpCodec) Decode() (pack xnet.Pack, free func(), err error) {
	var length int32

	if err := binary.Read(c.reader, binary.BigEndian, &length); err != nil {
		return nil, nil, errors.Wrap(err, "read length failed")
	}

	if length <= 0 || length > xnet.MaxPackSize {
		return nil, nil, errors.Errorf("invalid pack length: %d", length)
	}

	buf := pool.Alloc(int(length))
	free = func() {
		pool.Free(buf)
	}

	if _, err := io.ReadFull(c.reader, buf); err != nil {
		free()
		return nil, nil, errors.Wrap(err, "read data failed")
	}

	return xnet.Pack(buf), free, nil
}
