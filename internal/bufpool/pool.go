package bufpool

import (
	"errors"
	"slices"
	"sort"
	"sync"
)

type Pool interface {
	Alloc(int) []byte
	Free([]byte)
}

var _ Pool = (*SyncPool)(nil)

var (
	// ErrInvalidSize when the input size parameters are invalid
	ErrInvalidSize = errors.New("invalid size parameters")
	// ErrThresholdsRequired is returned when the thresholds are required
	ErrThresholdsRequired = errors.New("thresholds must not be empty")
	// ErrThresholdsNotSorted is returned when the thresholds are not sorted in ascending order
	ErrThresholdsNotSorted = errors.New("thresholds must be sorted in ascending order")
)

// SyncPool is a sync.Pool based slab allocation memory pool
// Simplified version inspired by multipool structure
type SyncPool struct {
	pools      []sync.Pool // different size memory pools
	thresholds []int       // size thresholds for each pool
}

// New creates a sync.Pool based slab allocation memory pool
// thresholds: size thresholds for each pool, must be sorted in ascending order
// For example: []int{256, 1024, 4096} will create 4 pools:
// - Pool 0: size <= 256 bytes
// - Pool 1: size > 256 and <= 1024 bytes
// - Pool 2: size > 1024 and <= 4096 bytes
// - Pool 3: size > 4096 bytes
func New(thresholds []int) (*SyncPool, error) {
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

	return &SyncPool{
		pools:      pools,
		thresholds: slices.Clone(thresholds),
	}, nil
}

// getPoolIndex returns the appropriate pool index for the given size
func (pool *SyncPool) getPoolIndex(size int) int {
	index := sort.Search(len(pool.thresholds), func(i int) bool {
		return pool.thresholds[i] >= size
	})

	if index < len(pool.thresholds) {
		return index
	}

	return len(pool.thresholds) // largest pool
}

// Alloc allocates a []byte from the internal slab class
// if there is no free block, it will create a new one
func (pool *SyncPool) Alloc(size int) []byte {
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
func (pool *SyncPool) Free(mem []byte) {
	size := cap(mem)

	if size < pool.thresholds[0] {
		return
	}

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
func (pool *SyncPool) ClassCount() int {
	return len(pool.pools)
}

// Thresholds returns a copy of the size thresholds
func (pool *SyncPool) Thresholds() []int {
	return slices.Clone(pool.thresholds)
}
