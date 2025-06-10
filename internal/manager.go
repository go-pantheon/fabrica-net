package internal

import (
	"sync"
	"sync/atomic"

	"github.com/go-pantheon/fabrica-net/conf"
)

var (
	// uidWidMap is a map of user IDs to worker IDs. It put before buckets and delete after buckets for fast search by uid.
	uidWidMap = &sync.Map{}
)

type WorkerManager struct {
	buckets    []*sync.Map
	size       *atomic.Int64
	shardCount uint64
}

func NewWorkerManager(c conf.Bucket) *WorkerManager {
	m := &WorkerManager{
		buckets:    make([]*sync.Map, uint64(c.BucketSize)),
		size:       &atomic.Int64{},
		shardCount: uint64(c.BucketSize),
	}

	for i := range m.shardCount {
		m.buckets[i] = &sync.Map{}
	}

	return m
}

func (m *WorkerManager) Worker(wid uint64) *Worker {
	if w, ok := m.getBucket(wid).Load(wid); ok {
		return w.(*Worker)
	}

	return nil
}

func (m *WorkerManager) Put(w *Worker) (old *Worker) {
	ow, loaded := m.getBucket(w.WID()).LoadOrStore(w.WID(), w)
	if loaded {
		return ow.(*Worker)
	}

	uidWidMap.Store(w.UID(), w.WID())
	m.size.Add(1)

	return nil
}

func (m *WorkerManager) Del(w *Worker) {
	uidWidMap.Delete(w.UID())
	m.getBucket(w.WID()).Delete(w.WID())
	m.size.Add(-1)
}

func (m *WorkerManager) Walk(f func(w *Worker) bool) {
	continued := true

	for _, b := range m.buckets {
		b.Range(func(key, value any) bool {
			v, ok := value.(*Worker)
			if !ok {
				return true
			}

			continued = f(v)

			return continued
		})

		if !continued {
			break
		}
	}
}

func (m *WorkerManager) GetByUID(uid int64) *Worker {
	wid, ok := uidWidMap.Load(uid)
	if !ok {
		return nil
	}

	if a, ok := m.getBucket(wid.(uint64)).Load(wid.(uint64)); ok {
		return a.(*Worker)
	}

	return nil
}

func (m *WorkerManager) GetByUIDs(uids []int64) map[int64]*Worker {
	wids := make([]uint64, 0, len(uids))

	for _, uid := range uids {
		if wid, ok := uidWidMap.Load(uid); ok {
			wids = append(wids, wid.(uint64))
		}
	}

	bucketKeys := make([][]uint64, m.shardCount)
	result := make(map[int64]*Worker, len(wids))

	for _, wid := range wids {
		key := getBucketKey(wid, m.shardCount)
		bucketKeys[key] = append(bucketKeys[key], wid)
	}

	for key, wids := range bucketKeys {
		if len(wids) == 0 {
			continue
		}

		bucket := m.buckets[key]
		for _, wid := range wids {
			if w, ok := bucket.Load(wid); ok {
				worker := w.(*Worker)
				result[worker.UID()] = worker
			}
		}
	}

	return result
}

func (m *WorkerManager) getBucket(wid uint64) *sync.Map {
	return m.buckets[getBucketKey(wid, m.shardCount)]
}

func getBucketKey(wid uint64, shardCount uint64) uint64 {
	return wyhash(wid) & (shardCount - 1)
}

// wyhash generates a 64-bit hash for the given 64-bit key using wyhash algorithm.
func wyhash(key uint64) uint64 {
	x := key
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33

	return x
}
