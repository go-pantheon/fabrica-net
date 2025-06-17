package codec

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/go-pantheon/fabrica-net/internal/bufpool"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
)

var (
	pool *bufpool.SyncPool

	ErrShortRead      = errors.New("short read")
	ErrShortWrite     = errors.New("short write")
	ErrInvalidPackLen = errors.New("invalid pack len")
)

func init() {
	minsize := 64

	if err := InitReaderPool([]int{
		4,
		minsize,
		minsize * 2,
		minsize * 4,
		minsize * 8,
		minsize * 16,
		minsize * 24,
		minsize * 32,
		minsize * 48,
		minsize * 64,
		minsize * 96,
		minsize * 128,
		minsize * 192,
		minsize * 256,
		minsize * 384,
		minsize * 512,
		minsize * 768,
	}); err != nil {
		panic("failed to initialize slab pool: " + err.Error())
	}
}

func InitReaderPool(thresholds []int) error {
	p, err := bufpool.New(thresholds)
	if err != nil {
		return err
	}

	pool = p

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
