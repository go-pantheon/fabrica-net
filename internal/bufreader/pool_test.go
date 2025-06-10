package bufreader

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyncPool(t *testing.T) {
	t.Parallel()

	// Test valid thresholds
	thresholds := []int{256, 1024, 4096}
	pool, err := newSyncPool(thresholds)
	require.NoError(t, err)
	assert.Equal(t, pool.ClassCount(), 4) // 3 thresholds + 1 overflow pool

	// Test empty thresholds
	_, err = newSyncPool([]int{})
	assert.ErrorIs(t, err, ErrThresholdsRequired)

	// Test unsorted thresholds
	_, err = newSyncPool([]int{1024, 256, 4096})
	assert.ErrorIs(t, err, ErrThresholdsNotSorted)
}

func TestPoolIndex(t *testing.T) {
	t.Parallel()

	thresholds := []int{256, 1024, 4096}
	pool, _ := newSyncPool(thresholds)

	tests := []struct {
		size     int
		expected int
	}{
		{100, 0},  // <= 256
		{256, 0},  // <= 256
		{512, 1},  // > 256, <= 1024
		{1024, 1}, // <= 1024
		{2048, 2}, // > 1024, <= 4096
		{4096, 2}, // <= 4096
		{8192, 3}, // > 4096 (overflow pool)
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("size %d", test.size), func(t *testing.T) {
			t.Parallel()

			result := pool.getPoolIndex(test.size)
			assert.Equal(t, result, test.expected)
		})
	}
}

func TestSimpleAllocFree(t *testing.T) {
	t.Parallel()

	thresholds := []int{256, 1024, 4096}
	pool, err := newSyncPool(thresholds)
	require.NoError(t, err)

	// Test allocation of different sizes
	buf1 := pool.Alloc(100)
	assert.Equal(t, len(buf1), 100)
	assert.GreaterOrEqual(t, cap(buf1), 100)

	buf2 := pool.Alloc(512)
	assert.Equal(t, len(buf2), 512)

	// Test zero size allocation
	buf3 := pool.Alloc(0)
	assert.Equal(t, len(buf3), 0)

	// Modify buffers to test data clearing
	for i := range buf1 {
		buf1[i] = byte(i % 256)
	}

	for i := range buf2 {
		buf2[i] = 0xFF
	}

	// Free buffers
	pool.Free(buf1)
	pool.Free(buf2)
	pool.Free(buf3) // Should handle empty slice gracefully

	// Allocate again to verify data is cleared
	buf4 := pool.Alloc(100)
	for _, b := range buf4 {
		assert.Equal(t, b, byte(0))
	}
}

func TestSimpleConcurrent(t *testing.T) {
	t.Parallel()

	thresholds := []int{256, 1024, 4096}
	pool, err := newSyncPool(thresholds)
	require.NoError(t, err)

	const (
		goroutines = 10
		iterations = 100
	)

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()

			for j := range iterations {
				buf := pool.Alloc(256 + (id*j)%1000)
				require.Greater(t, len(buf), 0)

				for i := range buf {
					require.Equal(t, buf[i], byte(0))
				}

				buf[0] = byte(id)
				assert.Equal(t, buf[0], byte(id))

				pool.Free(buf)
			}
		}(i)
	}

	wg.Wait()
}

func BenchmarkSimplePool(b *testing.B) {
	thresholds := []int{256, 1024, 4096, 16384}
	pool, _ := newSyncPool(thresholds)

	b.ResetTimer()

	for range b.N {
		buf := pool.Alloc(512)
		pool.Free(buf)
	}
}
