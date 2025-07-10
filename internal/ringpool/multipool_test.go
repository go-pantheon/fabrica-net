package ringpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiSizeRingPool(t *testing.T) {
	t.Parallel()

	sizeCapacityMap := map[int]uint64{
		64:   128,
		512:  64,
		1024: 32,
		4096: 16,
	}

	pool, err := NewMultiSizeRingPool(sizeCapacityMap)
	require.NoError(t, err)

	buf1 := pool.Alloc(32)   // Should use 64-byte pool
	buf2 := pool.Alloc(256)  // Should use 512-byte pool
	buf3 := pool.Alloc(800)  // Should use 1024-byte pool
	buf4 := pool.Alloc(2048) // Should use 4096-byte pool
	buf5 := pool.Alloc(8192) // Should allocate directly

	assert.Equal(t, 32, len(buf1))
	assert.Equal(t, 256, len(buf2))
	assert.Equal(t, 800, len(buf3))
	assert.Equal(t, 2048, len(buf4))
	assert.Equal(t, 8192, len(buf5))

	// Free buffers
	pool.Free(buf1)
	pool.Free(buf2)
	pool.Free(buf3)
	pool.Free(buf4)
	pool.Free(buf5)

	// Check stats
	stats := pool.Stats()
	assert.Equal(t, 4, len(stats)) // 4 pools
}

func BenchmarkMultiSizeRingPool(b *testing.B) {
	sizeCapacityMap := map[int]uint64{
		64:   128,
		512:  64,
		1024: 32,
		4096: 16,
	}

	pool, _ := NewMultiSizeRingPool(sizeCapacityMap)

	sizes := []int{32, 256, 800, 2048}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			size := sizes[i%len(sizes)]
			buf := pool.Alloc(size)
			pool.Free(buf)

			i++
		}
	})
}
