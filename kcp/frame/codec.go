package frame

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/internal/ringpool"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
)

var (
	once sync.Once
	pool ringpool.Pool
)

var (
	MOBAPoolConfig = map[int]uint64{
		32:   16384, // 32B * 16384 = 512KB - Player inputs, heartbeats (2^14)
		64:   8192,  // 64B * 8192 = 512KB - Quick commands, mini updates (2^13)
		128:  8192,  // 128B * 8192 = 1MB - Position sync, state updates (2^13)
		256:  4096,  // 256B * 4096 = 1MB - Skill casts, medium updates (2^12)
		512:  4096,  // 512B * 4096 = 2MB - Game state chunks (2^12)
		1024: 2048,  // 1KB * 2048 = 2MB - Batch updates, events (2^11)
		2048: 1024,  // 2KB * 1024 = 2MB - Map section updates (2^10)
		4096: 512,   // 4KB * 512 = 2MB - Large batch operations (2^9)
		8192: 256,   // 8KB * 256 = 2MB - Asset data, replay chunks (2^8)
	}
)

func init() {
	p, err := ringpool.Default()
	if err != nil {
		panic("failed to initialize kcp ring pool: " + err.Error())
	}

	pool = p
	lastStatsReset.Store(time.Now())
}

// InitMOBARingPool creates optimized pools for MOBA game traffic patterns
func InitMOBARingPool() error {
	return InitKcpRingPool(MOBAPoolConfig)
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

var _ internal.Codec = (*kcpCodec)(nil)

type kcpCodec struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func New(conn net.Conn) internal.Codec {
	return &kcpCodec{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}
}

func (c *kcpCodec) Encode(pack xnet.Pack) error {
	totalEncodes.Add(1)
	totalBytesOut.Add(uint64(len(pack)))

	if err := binary.Write(c.writer, binary.BigEndian, int32(len(pack))); err != nil {
		encodeErrors.Add(1)
		return errors.Wrap(err, "write length failed")
	}

	if _, err := c.writer.Write(pack); err != nil {
		encodeErrors.Add(1)
		return errors.Wrap(err, "write data failed")
	}

	if err := c.writer.Flush(); err != nil {
		encodeErrors.Add(1)
		return errors.Wrap(err, "flush writer failed")
	}

	return nil
}

func (c *kcpCodec) Decode() (pack xnet.Pack, free func(), err error) {
	totalDecodes.Add(1)

	var length int32
	if err := binary.Read(c.reader, binary.BigEndian, &length); err != nil {
		decodeErrors.Add(1)
		return nil, nil, errors.Wrap(err, "read length failed")
	}

	if length <= 0 || length > xnet.MaxPackSize {
		decodeErrors.Add(1)
		return nil, nil, errors.Errorf("invalid pack length: %d", length)
	}

	totalBytesIn.Add(uint64(length))

	buf := pool.Alloc(int(length))

	if cap(buf) == int(length) {
		poolHits.Add(1)
	} else {
		poolMisses.Add(1)
	}

	free = func() {
		pool.Free(buf)
	}

	defer func() {
		if err != nil {
			decodeErrors.Add(1)
			free()
		}
	}()

	if _, err = io.ReadFull(c.reader, buf); err != nil {
		return nil, nil, errors.Wrap(err, "read data failed")
	}

	return xnet.Pack(buf), free, nil
}
