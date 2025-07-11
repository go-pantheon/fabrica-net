package internal

import (
	"sync"
	"testing"

	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest
func TestNewWorkerManager(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bucketSize int
		wantSize   int
	}{
		{
			name:       "normal size",
			bucketSize: 16,
			wantSize:   16,
		},
		{
			name:       "small size",
			bucketSize: 1,
			wantSize:   1,
		},
		{
			name:       "large size",
			bucketSize: 1024,
			wantSize:   1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := conf.Bucket{
				BucketSize: tt.bucketSize,
			}

			bs := newWorkerManager(c)
			assert.Equal(t, tt.wantSize, len(bs.buckets))
		})
	}
}

func TestWorkerManager_BasicOperations(t *testing.T) {
	t.Parallel()

	c := conf.Bucket{
		BucketSize: 16,
	}
	bs := newWorkerManager(c)

	// Test Put and Get
	w1 := newTestWorker(1, 100)
	w2 := newTestWorker(2, 200)

	// Test Put
	old := bs.Put(w1)
	assert.Nil(t, old)
	old = bs.Put(w2)
	assert.Nil(t, old)

	// Test Get
	got := bs.Worker(w1.WID())
	assert.Equal(t, w1, got)
	got = bs.Worker(w2.WID())
	assert.Equal(t, w2, got)

	// Test Del
	bs.Del(w1.WID())
	got = bs.Worker(w1.WID())
	assert.Nil(t, got)
}

//nolint:paralleltest
func TestWorkerManager_UIDOperations(t *testing.T) {
	c := conf.Bucket{
		BucketSize: 16,
	}
	bs := newWorkerManager(c)

	w1 := newTestWorker(1, 100)
	w2 := newTestWorker(2, 200)

	bs.Put(w1)
	bs.Put(w2)

	// Test GetByUID
	got := bs.GetByUID(w1.UID())
	assert.Equal(t, w1, got)
	got = bs.GetByUID(w2.UID())
	assert.Equal(t, w2, got)
	got = bs.GetByUID(999) // non-existent UID
	assert.Nil(t, got)

	// Test GetByUIDs
	workers := bs.GetByUIDs([]int64{w1.UID(), w2.UID(), 999})
	assert.Equal(t, 2, len(workers))
	assert.Contains(t, workers, w1.UID())
	assert.Contains(t, workers, w2.UID())
}

//nolint:paralleltest
func TestWorkerManager_ConcurrentOperations(t *testing.T) {
	c := conf.Bucket{
		BucketSize: 16,
	}
	bs := newWorkerManager(c)

	// Create multiple workers
	workers := make([]*Worker, 100)

	for i := range 100 {
		workers[i] = newTestWorker(uint64(i), int64(i))
	}

	// Concurrent Put operations
	done := make(chan bool)

	for i := range 100 {
		go func(w *Worker) {
			bs.Put(w)
			done <- true
		}(workers[i])
	}

	// Wait for all goroutines to complete
	for range 100 {
		<-done
	}

	// Verify all workers are present
	for _, w := range workers {
		got := bs.Worker(w.WID())
		assert.Equal(t, w, got)
	}
}

func TestWorkerManager_Walk(t *testing.T) {
	t.Parallel()

	c := conf.Bucket{
		BucketSize: 16,
	}
	bs := newWorkerManager(c)

	// Add some workers
	workers := make([]*Worker, 5)

	for i := range 5 {
		workers[i] = newTestWorker(uint64(i), int64(i))
		bs.Put(workers[i])
	}

	// Test Walk
	count := 0

	bs.Walk(func(w *Worker) bool {
		count++
		return true
	})

	assert.Equal(t, 5, count)

	// Test Walk with early termination
	count = 0

	bs.Walk(func(w *Worker) bool {
		count++
		return count < 3
	})

	assert.Equal(t, 3, count)
}

func BenchmarkWorkerManager_Operations(b *testing.B) {
	c := conf.Bucket{
		BucketSize: 16,
	}
	bs := newWorkerManager(c)

	// Prepare test data
	workers := make([]*Worker, 1000)

	for i := range 1000 {
		workers[i] = newTestWorker(uint64(i), int64(i))
		bs.Put(workers[i])
	}

	b.Run("Put", func(b *testing.B) {
		b.ResetTimer()

		for i := range b.N {
			w := newTestWorker(uint64(i), int64(i))
			bs.Put(w)
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()

		for i := range b.N {
			bs.Worker(workers[i%1000].WID())
		}
	})

	b.Run("GetByUID", func(b *testing.B) {
		b.ResetTimer()

		for i := range b.N {
			bs.GetByUID(workers[i%1000].UID())
		}
	})

	b.Run("Walk", func(b *testing.B) {
		b.ResetTimer()

		for range b.N {
			bs.Walk(func(w *Worker) bool {
				return true
			})
		}
	})
}

func BenchmarkWorkerManager_Concurrent(b *testing.B) {
	bs := newWorkerManager(conf.Default().Bucket)

	// Prepare test data
	workers := make([]*Worker, 1000)

	for i := range 1000 {
		workers[i] = newTestWorker(uint64(i), int64(i))
		bs.Put(workers[i])
	}

	b.Run("ConcurrentPut", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				w := newTestWorker(uint64(b.N), int64(b.N)) //nolint:gosec
				bs.Put(w)
			}
		})
	})

	b.Run("ConcurrentGet", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				bs.Worker(workers[b.N%1000].WID()) //nolint:gosec
			}
		})
	})

	b.Run("ConcurrentGetByUID", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				bs.GetByUID(workers[b.N%1000].UID()) //nolint:gosec
			}
		})
	})

	b.Run("ConcurrentWalk", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				bs.Walk(func(w *Worker) bool { return true })
			}
		})
	})
}

func BenchmarkMap_Comparison(b *testing.B) {
	workers := make([]*Worker, 0, 20_000)

	for i := range 20_000 {
		workers = append(workers, newTestWorker(uint64(i), int64(i)))
	}

	benchmarkMutexMap(b, workers)
	benchmarkSyncMap(b, workers)
	benchmarkBuckets(b, workers)
}

func benchmarkMutexMap(b *testing.B, workers []*Worker) {
	type mutexMap struct {
		mu   sync.RWMutex
		m    map[uint64]*Worker
		uids *sync.Map
	}

	m := &mutexMap{
		m:    make(map[uint64]*Worker, 512*128),
		uids: &sync.Map{},
	}

	for i := range len(workers) {
		m.m[uint64(i)] = workers[i]
		m.uids.Store(int64(i), uint64(i))
	}

	b.Run("MutexMapPut", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				w := newTestWorker(uint64(b.N), int64(b.N)) //nolint:gosec

				m.mu.Lock()
				m.m[w.WID()] = w
				m.uids.Store(w.UID(), w.WID())
				m.mu.Unlock()
			}
		})
	})

	b.Run("MutexMapGet", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				m.mu.RLock()
				_ = m.m[workers[b.N%len(workers)].WID()]
				m.uids.Load(int64(b.N))
				m.mu.RUnlock()
			}
		})
	})

	b.Run("MutexMapGetByUID", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				m.mu.RLock()
				if wid, ok := m.uids.Load(b.N % len(workers)); ok {
					_ = m.m[wid.(uint64)]
				}
				m.mu.RUnlock()
			}
		})
	})

	b.Run("MutexMapWalk", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				m.mu.RLock()
				for _, w := range m.m {
					_ = w
				}
				m.mu.RUnlock()
			}
		})
	})
}

func benchmarkSyncMap(b *testing.B, workers []*Worker) {
	type syncMap struct {
		m    *sync.Map
		uids *sync.Map
	}

	sm := &syncMap{
		m:    &sync.Map{},
		uids: &sync.Map{},
	}

	for i := range len(workers) {
		sm.m.Store(workers[i].WID(), workers[i])
		sm.uids.Store(workers[i].UID(), workers[i].WID())
	}

	b.Run("SyncMapPut", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				w := newTestWorker(uint64(b.N), int64(b.N))
				sm.m.Store(w.WID(), w)
				sm.uids.Store(w.UID(), w.WID())
			}
		})
	})

	b.Run("SyncMapGet", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				a, _ := sm.m.Load(workers[b.N%len(workers)].WID())
				_ = a.(*Worker)

				sm.uids.Load(int64(b.N))
			}
		})
	})

	b.Run("SyncMapGetByUID", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				wid, ok := sm.uids.Load(b.N % len(workers))
				if ok {
					a, _ := sm.m.Load(wid.(uint64))
					_ = a.(*Worker)
				}
			}
		})
	})

	b.Run("SyncMapWalk", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				sm.m.Range(func(key, value any) bool {
					_ = value.(*Worker)
					return true
				})
			}
		})
	})
}

func benchmarkBuckets(b *testing.B, workers []*Worker) {
	c := conf.Bucket{
		BucketSize: 128,
	}
	buckets := newWorkerManager(c)

	for i := range len(workers) {
		buckets.Put(workers[i])
	}

	b.Run("BucketsPut", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buckets.Put(newTestWorker(uint64(b.N), int64(b.N)))
			}
		})
	})

	b.Run("BucketsGet", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buckets.Worker(workers[b.N%len(workers)].WID())
			}
		})
	})

	b.Run("BucketsGetByUID", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buckets.GetByUID(workers[b.N%len(workers)].UID())
			}
		})
	})

	b.Run("BucketsWalk", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buckets.Walk(func(w *Worker) bool { return true })
			}
		})
	})
}

//nolint:paralleltest
func TestWorkerManager_ConcurrencySafety(t *testing.T) {
	c := conf.Bucket{
		BucketSize: 16,
	}
	bs := newWorkerManager(c)

	const (
		numWorkers    = 1000
		numGoroutines = 100
		numOperations = 1000
	)

	workers := make([]*Worker, numWorkers)
	for i := range numWorkers {
		workers[i] = newTestWorker(uint64(i), int64(i))
	}

	//nolint:paralleltest
	t.Run("ConcurrentPut", func(t *testing.T) {
		testConcurrentPut(t, bs, workers, numGoroutines, numOperations)
	})

	//nolint:paralleltest
	t.Run("ConcurrentGet", func(t *testing.T) {
		testConcurrentGet(t, bs, workers, numGoroutines, numOperations)
	})

	//nolint:paralleltest
	t.Run("ConcurrentGetByUID", func(t *testing.T) {
		testConcurrentGetByUID(t, bs, workers, numGoroutines, numOperations)
	})

	//nolint:paralleltest
	t.Run("ConcurrentPutAndGet", func(t *testing.T) {
		testConcurrentPutAndGet(t, bs, workers, numGoroutines, numOperations)
	})

	//nolint:paralleltest
	t.Run("ConcurrentDel", func(t *testing.T) {
		testConcurrentDel(t, bs, workers, numGoroutines, numOperations)
	})

	//nolint:paralleltest
	t.Run("ConcurrentWalk", func(t *testing.T) {
		testConcurrentWalk(t, bs, workers, numGoroutines, numWorkers)
	})
}

func testConcurrentPut(t *testing.T, bs *WorkerManager, workers []*Worker, numGoroutines, numOperations int) {
	var wg sync.WaitGroup

	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(routineID int) {
			defer wg.Done()

			for j := range numOperations {
				workerID := (routineID*numOperations + j) % len(workers)
				bs.Put(workers[workerID])
			}
		}(i)
	}

	wg.Wait()

	for _, w := range workers {
		got := bs.Worker(w.WID())
		assert.Equal(t, w, got, "Worker not found after concurrent Put")
	}
}

func testConcurrentGet(t *testing.T, bs *WorkerManager, workers []*Worker, numGoroutines, numOperations int) {
	var wg sync.WaitGroup

	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(routineID int) {
			defer wg.Done()

			for j := range numOperations {
				workerID := (routineID*numOperations + j) % len(workers)
				got := bs.Worker(workers[workerID].WID())
				assert.Equal(t, workers[workerID], got, "Incorrect worker retrieved during concurrent Get")
			}
		}(i)
	}

	wg.Wait()
}

func testConcurrentGetByUID(t *testing.T, bs *WorkerManager, workers []*Worker, numGoroutines, numOperations int) {
	var wg sync.WaitGroup

	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(routineID int) {
			defer wg.Done()

			for j := range numOperations {
				workerID := (routineID*numOperations + j) % len(workers)
				got := bs.GetByUID(workers[workerID].UID())
				assert.Equal(t, workers[workerID], got, "Incorrect worker retrieved during concurrent GetByUID")
			}
		}(i)
	}

	wg.Wait()
}

func testConcurrentPutAndGet(t *testing.T, bs *WorkerManager, workers []*Worker, numGoroutines, numOperations int) {
	var wg sync.WaitGroup

	wg.Add(numGoroutines * 2)

	for i := range numGoroutines {
		go func(routineID int) {
			defer wg.Done()

			for j := range numOperations {
				workerID := (routineID*numOperations + j) % len(workers)
				bs.Put(workers[workerID])
			}
		}(i)
	}

	for i := range numGoroutines {
		go func(routineID int) {
			defer wg.Done()

			for j := range numOperations {
				workerID := (routineID*numOperations + j) % len(workers)
				if got := bs.Worker(workers[workerID].WID()); got != nil {
					assert.Equal(t, workers[workerID], got, "Incorrect worker retrieved during concurrent Put and Get")
				}
			}
		}(i)
	}

	wg.Wait()
}

func testConcurrentDel(t *testing.T, bs *WorkerManager, workers []*Worker, numGoroutines, numOperations int) {
	var wg sync.WaitGroup

	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(routineID int) {
			defer wg.Done()

			for j := range numOperations {
				workerID := uint64((routineID*numOperations + j) % len(workers))
				bs.Del(workerID)
			}
		}(i)
	}

	wg.Wait()

	for _, w := range workers {
		got := bs.Worker(w.WID())
		assert.Nil(t, got, "Worker still exists after concurrent Del")
	}
}

func testConcurrentWalk(t *testing.T, bs *WorkerManager, workers []*Worker, numGoroutines, numWorkers int) {
	for _, w := range workers[:numWorkers/2] {
		bs.Put(w)
	}

	var wg sync.WaitGroup

	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()

			count := 0

			bs.Walk(func(w *Worker) bool {
				count++
				return true
			})

			assert.Equal(t, numWorkers/2, count, "Incorrect count during concurrent Walk")
		}()
	}

	wg.Wait()
}

func newTestWorker(id uint64, uid int64) *Worker {
	return &Worker{
		id:      id,
		session: xnet.NewSession(uid, "", 0),
	}
}
