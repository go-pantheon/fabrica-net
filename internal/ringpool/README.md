# Ring Buffer Pool

A high-performance, lock-free ring buffer pool implementation designed for scenarios requiring millions of concurrent connections with minimal latency and memory overhead.

## Features

- **Lock-free operations** using atomic operations for maximum performance
- **Zero-allocation fast path** for common buffer sizes
- **Pre-allocated memory** reduces GC pressure
- **Fallback mechanism** for when ring is exhausted
- **Multi-size support** with automatic size selection
- **Comprehensive statistics** for monitoring and optimization

## Design Principles

### 1. Lock-Free Ring Buffer
- Uses atomic head/tail pointers for concurrent access
- Ring capacity must be power of 2 for efficient bitwise operations
- Pre-allocated buffers eliminate allocation overhead

### 2. Memory Efficiency
- Fixed-size buffers reduce memory fragmentation
- Reuses memory immediately without GC intervention
- Minimal metadata overhead per buffer

### 3. Performance Optimization
- O(1) allocation and deallocation
- CPU cache-friendly sequential access patterns
- Automatic fallback to sync.Pool when ring is exhausted

## Usage

### Single Size Pool

```go
import "github.com/go-pantheon/fabrica-net/internal/ringpool"

// Create pool for 1KB buffers with 256 slots
pool, err := ringpool.NewRingBufferPool(1024, 256)
if err != nil {
    panic(err)
}

// Allocate buffer
buf := pool.Alloc(512)  // Returns 512-byte slice from 1KB buffer

// Use buffer
copy(buf, data)

// Free buffer
pool.Free(buf)
```

### Multi-Size Pool

```go
// Configure different buffer sizes for different packet types
sizeCapacityMap := map[int]uint64{
    64:   1024,  // Small packets (heartbeat, ping)
    512:  512,   // Medium packets (commands, small data)
    1024: 256,   // Large packets (player state)
    4096: 128,   // Very large packets (world data)
}

pool, err := ringpool.NewMultiSizeRingPool(sizeCapacityMap)
if err != nil {
    panic(err)
}

// Automatically selects appropriate sized pool
smallBuf := pool.Alloc(32)    // Uses 64-byte pool
mediumBuf := pool.Alloc(400)  // Uses 512-byte pool
largeBuf := pool.Alloc(800)   // Uses 1024-byte pool
```

## Performance Characteristics

### Memory Usage
- **Ring Buffer**: `capacity × bufferSize` bytes pre-allocated
- **Metadata**: ~64 bytes per pool instance
- **No fragmentation**: Fixed-size allocations

### Allocation Performance
- **Fast path**: ~5-10ns per allocation (ring buffer)
- **Fallback**: ~100-200ns per allocation (sync.Pool)
- **Zero allocation**: No heap allocations for common cases

### Concurrency
- **Lock-free**: Multiple goroutines can allocate/free simultaneously
- **Scalable**: Performance doesn't degrade with goroutine count
- **Memory ordering**: Proper memory barriers for correctness

## Configuration Guidelines

### Buffer Sizes
Choose buffer sizes based on your application's packet distribution:

```go
// Gaming server typical configuration
sizeCapacityMap := map[int]uint64{
    64:   2048,  // 128KB total - heartbeat, ping, small commands
    512:  1024,  // 512KB total - game actions, player updates
    1024: 512,   // 512KB total - world state, large messages
    4096: 256,   // 1MB total - map data, asset chunks
}
```

### Capacity Sizing
- **High frequency, small packets**: Large capacity (1024-4096)
- **Low frequency, large packets**: Small capacity (64-256)
- **Memory constraint**: Balance between performance and memory usage

### Ring Size Selection
Ring capacity should be:
- **Power of 2** (required for bitwise operations)
- **2-4x peak concurrent allocations** (prevent exhaustion)
- **Consider allocation/free patterns** (burst vs steady)

## Statistics and Monitoring

```go
stats := pool.Stats()
fmt.Printf("Allocations: %d\n", stats.AllocCount)
fmt.Printf("Frees: %d\n", stats.FreeCount)
fmt.Printf("Fallback uses: %d\n", stats.FallbackCount)
fmt.Printf("Available buffers: %d\n", stats.Available)
fmt.Printf("Ring utilization: %.2f%%\n",
    float64(stats.RingSize-stats.Available)/float64(stats.RingSize)*100)
```

## Best Practices

1. **Profile your workload** to determine optimal buffer sizes
2. **Monitor statistics** to tune ring capacities
3. **Pre-warm pools** during application startup
4. **Use multi-size pools** for varied packet sizes
5. **Size rings generously** to avoid fallback overhead
6. **Consider memory constraints** when sizing rings

## Integration with Fabrica-Net

This pool can be integrated into the codec layer as a drop-in replacement:

```go
// In codec/frame.go
func init() {
    // Replace sync.Pool with RingBufferPool
    sizeCapacityMap := map[int]uint64{
        64:   1024,
        512:  512,
        1024: 256,
        4096: 128,
    }

    pool, err := ringpool.NewMultiSizeRingPool(sizeCapacityMap)
    if err != nil {
        panic(err)
    }

    // Use pool in Decode/Encode functions
}
```

This provides significant performance improvements for high-throughput networking scenarios while maintaining API compatibility.
