package ringpool

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRingBufferPool(t *testing.T) {
	t.Parallel()

	// Test valid parameters
	pool, err := NewRingBufferPool(1024, 256)
	require.NoError(t, err)
	assert.NotNil(t, pool)
	assert.Equal(t, 1024, pool.bufferSize)
	assert.Equal(t, uint64(256), pool.capacity)

	// Test invalid buffer size
	_, err = NewRingBufferPool(0, 256)
	assert.ErrorIs(t, err, ErrInvalidSize)

	// Test invalid capacity (not power of 2)
	_, err = NewRingBufferPool(1024, 255)
	assert.ErrorIs(t, err, ErrInvalidCapacity)

	// Test zero capacity
	_, err = NewRingBufferPool(1024, 0)
	assert.ErrorIs(t, err, ErrInvalidCapacity)
}

func TestRingBufferPoolBasicAllocation(t *testing.T) {
	t.Parallel()

	pool, err := NewRingBufferPool(1024, 4)
	require.NoError(t, err)

	// Test normal allocation
	buf1 := pool.Alloc(512)
	assert.Equal(t, 512, len(buf1))
	assert.Equal(t, 1024, cap(buf1))

	// Test zero size allocation
	buf2 := pool.Alloc(0)
	assert.Equal(t, 0, len(buf2))

	// Test oversized allocation
	buf3 := pool.Alloc(2048)
	assert.Equal(t, 2048, len(buf3))
	assert.Equal(t, 2048, cap(buf3))

	// Free buffers
	pool.Free(buf1)
	pool.Free(buf2)
	pool.Free(buf3)
}

func TestRingBufferPoolExhaustion(t *testing.T) {
	t.Parallel()

	pool, err := NewRingBufferPool(1024, 4)
	require.NoError(t, err)

	// Allocate all buffers from ring
	buffers := make([][]byte, 0, 6)

	for range 6 {
		buf := pool.Alloc(512)
		buffers = append(buffers, buf)
	}

	stats := pool.Stats()
	assert.Equal(t, uint64(6), stats.AllocCount)
	assert.True(t, stats.FallbackCount > 0) // Some allocations should use fallback

	// Free all buffers
	for _, buf := range buffers {
		pool.Free(buf)
	}

	finalStats := pool.Stats()
	assert.Equal(t, uint64(6), finalStats.FreeCount)
}

func TestRingBufferPoolConcurrent(t *testing.T) {
	t.Parallel()

	pool, err := NewRingBufferPool(1024, 256)
	require.NoError(t, err)

	const (
		goroutines = 20
		iterations = 100
	)

	var wg sync.WaitGroup

	bufChan := make(chan []byte, goroutines*iterations)
	errors := make(chan error, goroutines)

	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()

			for j := range iterations {
				buf := pool.Alloc(512)
				if len(buf) != 512 {
					errors <- assert.AnError
					return
				}

				buf[0] = byte(id + j)

				pool.Free(buf)
			}
		}(i)
	}

	consumeCloseChan := make(chan struct{})
	go func() {
		for buf := range bufChan {
			assert.True(t, buf[0] > 0)
			pool.Free(buf)
		}

		close(consumeCloseChan)
	}()

	wg.Wait()
	close(errors)
	close(bufChan)
	<-consumeCloseChan

	// Check for errors
	for err := range errors {
		require.NoError(t, err)
	}

	stats := pool.Stats()
	assert.Equal(t, uint64(goroutines*iterations), stats.AllocCount)
	assert.Equal(t, uint64(goroutines*iterations), stats.FreeCount)
}

func TestRingBufferPoolStats(t *testing.T) {
	t.Parallel()

	pool, err := NewRingBufferPool(1024, 8)
	require.NoError(t, err)

	// Initial stats
	stats := pool.Stats()
	assert.Equal(t, uint64(0), stats.AllocCount)
	assert.Equal(t, uint64(0), stats.FreeCount)
	assert.Equal(t, uint64(0), stats.FallbackCount)
	assert.Equal(t, uint64(8), stats.Available)

	// Allocate some buffers
	buf1 := pool.Alloc(512)
	buf2 := pool.Alloc(1024)

	stats = pool.Stats()
	assert.Equal(t, uint64(2), stats.AllocCount)
	assert.Equal(t, uint64(6), stats.Available)

	// Free one buffer
	pool.Free(buf1)

	stats = pool.Stats()
	assert.Equal(t, uint64(1), stats.FreeCount)
	assert.Equal(t, uint64(7), stats.Available)

	// Clean up
	pool.Free(buf2)
}

func BenchmarkRingBufferPool(b *testing.B) {
	pool, _ := NewRingBufferPool(1024, 256)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Alloc(512)
			pool.Free(buf)
		}
	})
}

func BenchmarkRingBufferPoolVsStdAlloc(b *testing.B) {
	pool, _ := NewRingBufferPool(1024, 256)

	b.Run("RingPool", func(b *testing.B) {
		b.ResetTimer()

		for range b.N {
			buf := pool.Alloc(512)
			pool.Free(buf)
		}
	})

	b.Run("StdAlloc", func(b *testing.B) {
		b.ResetTimer()

		for range b.N {
			buf := make([]byte, 512)
			_ = buf
		}
	})
}

func BenchmarkConcurrentRingBufferPool(b *testing.B) {
	pool, _ := NewRingBufferPool(1024, 1024)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Alloc(512)
			// Simulate some work
			for i := range 10 {
				if i < len(buf) {
					buf[i] = byte(i)
				}
			}

			pool.Free(buf)
		}
	})
}

func TestRingBufferPoolMemoryReuse(t *testing.T) {
	t.Parallel()

	pool, err := NewRingBufferPool(1024, 4)
	require.NoError(t, err)

	// Allocate buffer and get its address
	buf1 := pool.Alloc(512)
	addr1 := &buf1[0]

	// Free and reallocate
	pool.Free(buf1)
	buf2 := pool.Alloc(512)
	addr2 := &buf2[0]

	// Should reuse the same memory
	assert.Equal(t, addr1, addr2)
}

func TestRingBufferPoolUnderLoad(t *testing.T) {
	t.Parallel()

	pool, err := NewRingBufferPool(1024, 64)
	require.NoError(t, err)

	// Simulate high load
	done := make(chan struct{})
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(done)
	}()

	var (
		totalAllocs atomic.Uint64
		wg          sync.WaitGroup
	)

	// Multiple goroutines allocating/freeing rapidly
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			var localAllocs uint64

			for {
				select {
				case <-done:
					totalAllocs.Add(localAllocs)
					return
				default:
					buf := pool.Alloc(512)
					localAllocs++

					pool.Free(buf)
				}
			}
		}()
	}

	wg.Wait()

	stats := pool.Stats()

	t.Logf("Total allocations: %d", totalAllocs.Load())
	t.Logf("Pool stats: %+v", stats)

	// Should have processed many allocations
	assert.Greater(t, totalAllocs.Load(), uint64(1000))
	assert.Equal(t, stats.AllocCount, stats.FreeCount)
}
