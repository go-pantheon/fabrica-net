package ringpool

func Default() (Pool, error) {
	sizeCapacityMap := map[int]uint64{
		64:   8192 * 2, // 64bytes * 8192 * 2 = 1MB
		128:  8192,     // 128bytes * 8192 = 1MB
		256:  4096,     // 256bytes * 4096 = 1MB
		512:  2048,     // 512bytes * 2048 = 1MB
		1024: 1024,     // 1kb * 1024 = 1MB
		4096: 256,      // 4kb * 256 = 1MB
	}

	return NewMultiSizeRingPool(sizeCapacityMap)
}
