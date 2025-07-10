package ringpool

import (
	"sort"
	"sync"
)

// MultiSizeRingPool manages multiple ring pools for different buffer sizes
type MultiSizeRingPool struct {
	pools map[int]*RingBufferPool
	sizes []int // sorted sizes for lookup
	mutex sync.RWMutex
}

// NewMultiSizeRingPool creates a pool manager for multiple buffer sizes
func NewMultiSizeRingPool(sizeCapacityMap map[int]uint64) (*MultiSizeRingPool, error) {
	pools := make(map[int]*RingBufferPool)
	sizes := make([]int, 0, len(sizeCapacityMap))

	for size, capacity := range sizeCapacityMap {
		pool, err := NewRingBufferPool(size, capacity)
		if err != nil {
			return nil, err
		}

		pools[size] = pool

		sizes = append(sizes, size)
	}

	sort.Ints(sizes)

	return &MultiSizeRingPool{
		pools: pools,
		sizes: sizes,
	}, nil
}

// Alloc allocates a buffer from the appropriate sized pool
func (mp *MultiSizeRingPool) Alloc(size int) []byte {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	for _, poolSize := range mp.sizes {
		if poolSize >= size {
			if pool, exists := mp.pools[poolSize]; exists {
				return pool.Alloc(size)
			}
		}
	}

	return make([]byte, size)
}

// Free returns a buffer to the appropriate pool
func (mp *MultiSizeRingPool) Free(buf []byte) {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	size := cap(buf)
	if pool, exists := mp.pools[size]; exists {
		pool.Free(buf)
	}
}

// Stats returns statistics for all pools
func (mp *MultiSizeRingPool) Stats() map[int]PoolStats {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	stats := make(map[int]PoolStats)
	for size, pool := range mp.pools {
		stats[size] = pool.Stats()
	}

	return stats
}
