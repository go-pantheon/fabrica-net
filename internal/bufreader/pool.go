package bufreader

import (
	"errors"
	"slices"
	"sync"
)

type Pool interface {
	Alloc(int) []byte
	Free([]byte)
}

var _ Pool = (*syncPool)(nil)

var (
	// ErrInvalidSize when the input size parameters are invalid
	ErrInvalidSize = errors.New("invalid size parameters")
	// ErrThresholdsRequired is returned when the thresholds are required
	ErrThresholdsRequired = errors.New("thresholds must not be empty")
	// ErrThresholdsNotSorted is returned when the thresholds are not sorted in ascending order
	ErrThresholdsNotSorted = errors.New("thresholds must be sorted in ascending order")
)

// syncPool is a sync.Pool based slab allocation memory pool
// Simplified version inspired by multipool structure
type syncPool struct {
	pools      []sync.Pool // different size memory pools
	thresholds []int       // size thresholds for each pool
}

// newSyncPool creates a sync.Pool based slab allocation memory pool
// thresholds: size thresholds for each pool, must be sorted in ascending order
// For example: []int{256, 1024, 4096} will create 4 pools:
// - Pool 0: size <= 256 bytes
// - Pool 1: size > 256 and <= 1024 bytes
// - Pool 2: size > 1024 and <= 4096 bytes
// - Pool 3: size > 4096 bytes
func newSyncPool(thresholds []int) (*syncPool, error) {
	if len(thresholds) == 0 {
		return nil, ErrThresholdsRequired
	}

	// Validate thresholds are sorted
	for i := 1; i < len(thresholds); i++ {
		if thresholds[i] <= thresholds[i-1] {
			return nil, ErrThresholdsNotSorted
		}
	}

	// Create one more pool for sizes larger than max threshold
	poolCount := len(thresholds) + 1
	pools := make([]sync.Pool, poolCount)

	// Initialize each pool
	for i := range poolCount {
		poolIndex := i
		pools[i].New = func() any {
			var size int

			if poolIndex < len(thresholds) {
				size = thresholds[poolIndex]
			} else {
				// For the largest pool, use double of max threshold as default size
				size = thresholds[len(thresholds)-1] * 2
			}

			buf := make([]byte, size)

			return &buf
		}
	}

	return &syncPool{
		pools:      pools,
		thresholds: slices.Clone(thresholds),
	}, nil
}

// getPoolIndex returns the appropriate pool index for the given size
func (pool *syncPool) getPoolIndex(size int) int {
	for i, threshold := range pool.thresholds {
		if size <= threshold {
			return i
		}
	}

	return len(pool.thresholds) // largest pool
}

// Alloc allocates a []byte from the internal slab class
// if there is no free block, it will create a new one
func (pool *syncPool) Alloc(size int) []byte {
	if size <= 0 {
		return make([]byte, 0)
	}

	poolIndex := pool.getPoolIndex(size)
	mem := pool.pools[poolIndex].Get().(*[]byte)

	// If the pooled buffer is too small, create a new one
	if cap(*mem) < size {
		return make([]byte, size)
	}

	return (*mem)[:size]
}

// Free frees the []byte allocated from Pool.Alloc
func (pool *syncPool) Free(mem []byte) {
	if len(mem) == 0 {
		return
	}

	size := cap(mem)
	poolIndex := pool.getPoolIndex(size)

	// Reset the slice to avoid memory leaks
	mem = mem[:cap(mem)]
	// Zero out sensitive data
	for i := range mem {
		mem[i] = 0
	}

	pool.pools[poolIndex].Put(&mem)
}

// ClassCount returns the number of memory pool classes
func (pool *syncPool) ClassCount() int {
	return len(pool.pools)
}

// Thresholds returns a copy of the size thresholds
func (pool *syncPool) Thresholds() []int {
	return slices.Clone(pool.thresholds)
}
