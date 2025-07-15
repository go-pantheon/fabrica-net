package frame

import (
	"sync/atomic"
	"time"

	"github.com/go-pantheon/fabrica-net/internal/ringpool"
)

var (
	totalEncodes  atomic.Uint64
	totalDecodes  atomic.Uint64
	totalBytesIn  atomic.Uint64
	totalBytesOut atomic.Uint64
	encodeErrors  atomic.Uint64
	decodeErrors  atomic.Uint64
	poolHits      atomic.Uint64
	poolMisses    atomic.Uint64

	lastStatsReset atomic.Value
)

type CodecStats struct {
	TotalEncodes  uint64    `json:"total_encodes"`
	TotalDecodes  uint64    `json:"total_decodes"`
	TotalBytesIn  uint64    `json:"total_bytes_in"`
	TotalBytesOut uint64    `json:"total_bytes_out"`
	EncodeErrors  uint64    `json:"encode_errors"`
	DecodeErrors  uint64    `json:"decode_errors"`
	PoolHits      uint64    `json:"pool_hits"`
	PoolMisses    uint64    `json:"pool_misses"`
	HitRatio      float64   `json:"hit_ratio"`
	AvgPacketSize float64   `json:"avg_packet_size"`
	LastReset     time.Time `json:"last_reset"`
}

func GetPoolStats() map[int]ringpool.PoolStats {
	if pool == nil {
		return nil
	}

	if multiPool, ok := pool.(*ringpool.MultiSizeRingPool); ok {
		return multiPool.Stats()
	}

	return nil
}

func GetCodecStats() CodecStats {
	encodes := totalEncodes.Load()
	decodes := totalDecodes.Load()
	bytesIn := totalBytesIn.Load()
	bytesOut := totalBytesOut.Load()
	hits := poolHits.Load()
	misses := poolMisses.Load()

	var hitRatio float64
	if total := hits + misses; total > 0 {
		hitRatio = float64(hits) / float64(total)
	}

	var avgPacketSize float64
	if decodes > 0 {
		avgPacketSize = float64(bytesIn) / float64(decodes)
	}

	lastReset := time.Now()
	if val := lastStatsReset.Load(); val != nil {
		lastReset = val.(time.Time)
	}

	return CodecStats{
		TotalEncodes:  encodes,
		TotalDecodes:  decodes,
		TotalBytesIn:  bytesIn,
		TotalBytesOut: bytesOut,
		EncodeErrors:  encodeErrors.Load(),
		DecodeErrors:  decodeErrors.Load(),
		PoolHits:      hits,
		PoolMisses:    misses,
		HitRatio:      hitRatio,
		AvgPacketSize: avgPacketSize,
		LastReset:     lastReset,
	}
}

// ResetCodecStats resets all codec statistics
func ResetCodecStats() {
	totalEncodes.Store(0)
	totalDecodes.Store(0)
	totalBytesIn.Store(0)
	totalBytesOut.Store(0)
	encodeErrors.Store(0)
	decodeErrors.Store(0)
	poolHits.Store(0)
	poolMisses.Store(0)
	lastStatsReset.Store(time.Now())
}
