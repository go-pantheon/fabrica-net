package ringpool

import (
	"errors"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	// ErrPoolExhausted is returned when the ring buffer pool is exhausted
	ErrPoolExhausted = errors.New("ring buffer pool exhausted")
	// ErrInvalidSize is returned when the buffer size is invalid
	ErrInvalidSize = errors.New("invalid buffer size")
	// ErrInvalidCapacity is returned when the ring capacity is invalid
	ErrInvalidCapacity = errors.New("ring capacity must be a power of 2")
)

// Pool interface for compatibility with bufpool
type Pool interface {
	Alloc(int) []byte
	Free([]byte)
}

var _ Pool = (*RingBufferPool)(nil)

// RingBufferPool implements a lock-free ring buffer pool for high-performance buffer allocation
type RingBufferPool struct {
	buffers [][]byte
	mask    uint64

	head atomic.Uint64 // next allocation position
	tail atomic.Uint64 // next free position

	// Per-slot state tracking for lock-free coordination (LMAX Disruptor pattern)
	slotStates []atomic.Uint32 // 0 = available, 1 = allocated

	bufferSize int
	capacity   uint64 // total number of buffers (must be power of 2)

	// Buffer address to index mapping for O(1) lookups
	bufferAddrs map[uintptr]uint64

	// Fallback for when ring is exhausted
	fallbackPool sync.Pool

	// Statistics - using atomic.Uint64 for better performance and safety
	allocCount    atomic.Uint64
	freeCount     atomic.Uint64
	fallbackCount atomic.Uint64
}

// NewRingBufferPool creates a new ring buffer pool
// bufferSize: size of each buffer in bytes
// capacity: number of buffers in the ring (must be power of 2)
func NewRingBufferPool(bufferSize int, capacity uint64) (*RingBufferPool, error) {
	if bufferSize <= 0 {
		return nil, ErrInvalidSize
	}

	if capacity == 0 || (capacity&(capacity-1)) != 0 {
		return nil, ErrInvalidCapacity
	}

	pool := &RingBufferPool{
		buffers:     make([][]byte, capacity),
		slotStates:  make([]atomic.Uint32, capacity),
		bufferAddrs: make(map[uintptr]uint64, capacity),
		mask:        capacity - 1,
		bufferSize:  bufferSize,
		capacity:    capacity,
	}

	// Pre-allocate all buffers and map their addresses to indices
	for i := range capacity {
		pool.buffers[i] = make([]byte, bufferSize)
		bufAddr := uintptr(unsafe.Pointer(unsafe.SliceData(pool.buffers[i]))) // #nosec G103
		pool.bufferAddrs[bufAddr] = i
	}

	pool.fallbackPool.New = func() any {
		b := make([]byte, bufferSize)
		return &b
	}

	return pool, nil
}

// Alloc allocates a buffer from the ring pool
// Returns a buffer of the requested size, or falls back to heap allocation
func (p *RingBufferPool) Alloc(size int) []byte {
	p.allocCount.Add(1)

	if size <= 0 {
		return make([]byte, 0)
	}

	if size > p.bufferSize {
		return make([]byte, size)
	}

	for {
		head := p.head.Load()
		tail := p.tail.Load()

		if head == tail+p.capacity {
			// Ring is full, use fallback
			p.fallbackCount.Add(1)

			buf := *p.fallbackPool.Get().(*[]byte)

			return buf[:size]
		}

		if p.head.CompareAndSwap(head, head+1) {
			index := head & p.mask

			if p.slotStates[index].CompareAndSwap(0, 1) {
				buf := p.buffers[index]
				return buf[:size]
			}

			// Slot was not available, handle it gracefully by retrying
			continue
		}
	}
}

// Free returns a buffer to the ring pool
func (p *RingBufferPool) Free(buf []byte) {
	p.freeCount.Add(1)

	if p.isRingBuffer(buf) {
		p.clearBuffer(buf)

		bufAddr := uintptr(unsafe.Pointer(unsafe.SliceData(buf))) // #nosec G103
		if index, exists := p.bufferAddrs[bufAddr]; exists {
			p.slotStates[index].Store(0)
		}

		p.tail.Add(1)

		return
	}

	if cap(buf) == p.bufferSize {
		p.clearBuffer(buf)

		b := buf[:p.bufferSize]
		p.fallbackPool.Put(&b)
	}
}

// clearBuffer securely zeros out buffer contents
func (p *RingBufferPool) clearBuffer(buf []byte) {
	if cap(buf) > 0 {
		fullBuf := buf[:cap(buf)]
		clear(fullBuf)
	}
}

// isRingBuffer checks if a buffer belongs to our pre-allocated ring
func (p *RingBufferPool) isRingBuffer(buf []byte) bool {
	if cap(buf) != p.bufferSize {
		return false
	}

	// Get buffer address safely (works for both zero and non-zero length slices)
	bufAddr := uintptr(unsafe.Pointer(unsafe.SliceData(buf))) // #nosec G103

	_, exists := p.bufferAddrs[bufAddr]
	return exists
}

// Stats returns pool statistics
func (p *RingBufferPool) Stats() PoolStats {
	return PoolStats{
		AllocCount:    p.allocCount.Load(),
		FreeCount:     p.freeCount.Load(),
		FallbackCount: p.fallbackCount.Load(),
		RingSize:      p.capacity,
		BufferSize:    p.bufferSize,
		Available:     p.available(),
	}
}

// available returns the number of available buffers in the ring
func (p *RingBufferPool) available() uint64 {
	head := p.head.Load()
	tail := p.tail.Load()

	return p.capacity - (head - tail)
}

// PoolStats contains pool statistics
type PoolStats struct {
	AllocCount    uint64 // Total allocations
	FreeCount     uint64 // Total frees
	FallbackCount uint64 // Fallback allocations
	RingSize      uint64 // Ring buffer capacity
	BufferSize    int    // Size of each buffer
	Available     uint64 // Currently available buffers
}
