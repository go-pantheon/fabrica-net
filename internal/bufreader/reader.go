// Package bufreader provides a buffered reader with automatic buffer management
// and memory pooling for efficient reuse of buffers.
package bufreader

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
)

var (
	pool *syncPool
	// ErrBufReaderAlreadyClosed is returned when the reader is already closed
	ErrBufReaderAlreadyClosed = errors.New("bufreader.Reader already closed")
	// ErrBufReaderSize is returned when the reader size is invalid
	ErrBufReaderSize = errors.New("bufreader.Reader size error")
)

func init() {
	minsize := 64

	if err := InitReaderPool([]int{
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
	p, err := newSyncPool(thresholds)
	if err != nil {
		return err
	}

	pool = p

	return nil
}

var _ io.Reader = (*Reader)(nil)

// Reader implements buffered reading with automatic buffer management
// and memory pooling for efficient reuse of buffers.
type Reader struct {
	reader    io.Reader
	bufReader *bufio.Reader
	buf       []byte
	w         int
	r         int
	cleanedUp bool
}

func NewReader(r io.Reader, initialSize int) *Reader {
	return &Reader{
		reader:    r,
		bufReader: bufio.NewReader(r),
		buf:       pool.Alloc(initialSize),
	}
}

func (r *Reader) Read(p []byte) (n int, err error) {
	return r.bufReader.Read(p)
}

func (r *Reader) ReadByte() (n byte, err error) {
	if r.unreadBytes() > 0 {
		n = r.buf[r.r]
		r.r++

		return
	}

	if r.capLeft() == 0 {
		if r.cleanedUp {
			return 0, ErrBufReaderAlreadyClosed
		}

		// both r and w is at final position
		r.r, r.w = 0, 0
	}

	// enough room to Read
	if err = r.readAtLeast(1); err != nil {
		return
	}

	n = r.buf[r.r]
	r.r++

	return
}

func (r *Reader) ReadUint32() (uint32, error) {
	if r.unreadBytes() < 4 {
		if err := r.readAtLeast(4); err != nil {
			return 0, err
		}
	}

	return binary.BigEndian.Uint32(r.buf[r.r : r.r+4]), nil
}

// ReadFull return a slice with exactly n bytes. It's safe to use the result slice before the next call to any Read method.
func (r *Reader) ReadFull(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrBufReaderSize
	}

	// Try to fulfill from existing buffer first
	if unread := r.unreadBytes(); unread >= n {
		result := r.buf[r.r : r.r+n]
		r.r += n

		return result, nil
	}

	// Calculate needed capacity using exponential growth
	needed := n + r.unreadBytes()
	if needed > len(r.buf) {
		if r.cleanedUp {
			return nil, ErrBufReaderAlreadyClosed
		}

		var extraSpace int

		switch {
		case n <= 4096: // 50% extra for small buffers
			extraSpace = n >> 1
		case n <= 65536: // 25% extra for medium buffers
			extraSpace = n >> 2
		default: // 12.5% extra for large buffers
			extraSpace = n >> 3
		}

		newSize := nextPowerOfTwo(needed + extraSpace)
		newBuf := pool.Alloc(newSize)

		// Copy existing data and recycle old buffer
		r.w = copy(newBuf, r.buf[r.r:r.w])
		r.r = 0
		pool.Free(r.buf)
		r.buf = newBuf
	} else {
		// Compact existing buffer
		r.w = copy(r.buf, r.buf[r.r:r.w])
		r.r = 0
	}

	// Read remaining bytes
	if err := r.readAtLeast(n - r.unreadBytes()); err != nil {
		return nil, err
	}

	result := r.buf[r.r : r.r+n]
	r.r += n

	return result, nil
}

func (r *Reader) readAtLeast(bytes int) error {
	if n, err := io.ReadAtLeast(r.reader, r.buf[r.w:], bytes); err != nil {
		return err
	} else {
		r.w += n
		return nil
	}
}

func (r *Reader) unreadBytes() int {
	return r.w - r.r
}

func (r *Reader) capLeft() int {
	return len(r.buf) - r.w
}

func (r *Reader) Close() error {
	if r.cleanedUp {
		return ErrBufReaderAlreadyClosed
	}

	r.cleanedUp = true
	pool.Free(r.buf)
	r.w, r.r = 0, 0
	r.buf = nil

	return nil
}

// Add helper function for buffer growth calculation
func nextPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}

	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32

	return n + 1
}
