package codec

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/go-pantheon/fabrica-net/internal/ringpool"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
)

var (
	pool ringpool.Pool

	ErrShortRead      = errors.New("short read")
	ErrShortWrite     = errors.New("short write")
	ErrInvalidPackLen = errors.New("invalid pack len")
)

func init() {
	sizeCapacityMap := map[int]uint64{
		4:    65536 * 32, // 4bytes * 65536 * 32 = 16MB
		64:   65536 * 8,  // 64bytes * 65536 * 8 = 32MB
		128:  32768 * 8,  // 128bytes * 32768 * 8 = 32MB
		256:  16384 * 8,  // 256bytes * 16384 * 8 = 32MB
		512:  8192 * 8,   // 512bytes * 8192 * 8 = 32MB
		1024: 4096 * 8,   // 1kb * 4096 * 8 = 32MB
		4096: 1024 * 8,   // 4kb * 1024 * 8 = 32MB
	}
	// Total memory: 224MB for ring buffers
	// Total capacity: 1M buffers for burst handling

	p, err := ringpool.NewMultiSizeRingPool(sizeCapacityMap)
	if err != nil {
		panic("failed to initialize ring pool: " + err.Error())
	}

	pool = p
}

func InitRingPool(sizeCapacityMap map[int]uint64) (err error) {
	pool, err = ringpool.NewMultiSizeRingPool(sizeCapacityMap)
	if err != nil {
		return err
	}

	return nil
}

func Encode(w *bufio.Writer, pack xnet.Pack) error {
	if err := binary.Write(w, binary.BigEndian, xnet.PackLenSize+int32(len(pack))); err != nil {
		return errors.Wrap(err, "write pack len failed")
	}

	n, err := w.Write(pack)
	if err != nil {
		return errors.Wrapf(err, "write pack failed")
	}

	if n != len(pack) {
		return ErrShortWrite
	}

	if err := w.Flush(); err != nil {
		return errors.Wrap(err, "flush writer failed")
	}

	return nil
}

func Decode(r io.Reader) (pack xnet.Pack, free func(), err error) {
	totalLen, err := readInt32(r)
	if err != nil {
		return nil, nil, err
	}

	if totalLen < xnet.PackLenSize || totalLen > xnet.MaxPackSize {
		return nil, nil, ErrInvalidPackLen
	}

	return readPack(r, totalLen-xnet.PackLenSize)
}

func readInt32(r io.Reader) (int32, error) {
	buf := pool.Alloc(int(xnet.PackLenSize))
	defer pool.Free(buf)

	n, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, errors.Wrap(err, "read int32 failed")
	}

	if n != int(xnet.PackLenSize) {
		return 0, ErrShortRead
	}

	return int32(binary.BigEndian.Uint32(buf)), nil
}

func readPack(r io.Reader, packLen int32) (pack xnet.Pack, free func(), err error) {
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
