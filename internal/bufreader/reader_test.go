package bufreader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReader(t *testing.T) {
	t.Parallel()

	r := NewReader(bytes.NewReader(nil), 1024)
	assert.NotNil(t, r)
}

func TestReadByte(t *testing.T) {
	t.Parallel()

	t.Run("basic read", func(t *testing.T) {
		t.Parallel()

		data := []byte{0x01, 0x02}
		br := NewReader(bytes.NewReader(data), 2)

		b, err := br.ReadByte()
		require.NoError(t, err)
		assert.Equal(t, b, byte(0x01))

		b, err = br.ReadByte()
		require.NoError(t, err)
		assert.Equal(t, b, byte(0x02))

		_, err = br.ReadByte()
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("buffer expansion", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 2048)
		br := NewReader(bytes.NewReader(data), 1024)

		for range 2048 {
			_, err := br.ReadByte()
			assert.Nil(t, err)
		}
	})

	t.Run("closed reader", func(t *testing.T) {
		t.Parallel()

		br := NewReader(bytes.NewReader(nil), 1)
		err := br.Close()
		require.NoError(t, err)

		_, err = br.ReadByte()
		assert.ErrorIs(t, err, ErrBufReaderAlreadyClosed)
	})
}

func TestReadFull(t *testing.T) {
	t.Parallel()

	t.Run("exact buffer size", func(t *testing.T) {
		t.Parallel()

		data := bytes.Repeat([]byte{0xaa}, 1024)
		br := NewReader(bytes.NewReader(data), 1024)

		result, err := br.ReadFull(1024)
		require.NoError(t, err)
		assert.Equal(t, result, data)
	})

	t.Run("multiple reads with buffer growth", func(t *testing.T) {
		t.Parallel()

		data := bytes.Repeat([]byte{0xbb}, 4096)
		br := NewReader(bytes.NewReader(data), 1024)

		// First read: 1024 bytes (exact buffer size)
		ret1, err := br.ReadFull(1024)
		require.NoError(t, err)
		assert.Equal(t, len(ret1), 1024)

		// Second read: 2048 bytes (needs buffer expansion)
		ret2, err := br.ReadFull(2048)
		require.NoError(t, err)
		assert.Equal(t, len(ret2), 2048)

		// Third read: remaining 1024 bytes
		ret3, err := br.ReadFull(1024)
		require.NoError(t, err)
		assert.Equal(t, len(ret3), 1024)
	})

	t.Run("invalid size", func(t *testing.T) {
		t.Parallel()

		br := NewReader(bytes.NewReader(nil), 1)
		_, err := br.ReadFull(-1)
		assert.ErrorIs(t, err, ErrBufReaderSize)
	})

	t.Run("partial read then EOF", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 500)
		br := NewReader(bytes.NewReader(data), 1000)

		_, err := br.ReadFull(1000)
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("read after close", func(t *testing.T) {
		t.Parallel()

		br := NewReader(bytes.NewReader(nil), 1)
		err := br.Close()
		require.NoError(t, err)

		_, err = br.ReadFull(1)
		assert.ErrorIs(t, err, ErrBufReaderAlreadyClosed)
	})
}

func TestEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty reader", func(t *testing.T) {
		t.Parallel()

		br := NewReader(bytes.NewReader(nil), 1)
		_, err := br.ReadByte()
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("buffer compaction", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 3000)
		br := NewReader(bytes.NewReader(data), 1024)

		// Partial read to create buffer fragmentation
		_, err := br.ReadFull(500)
		assert.Nil(t, err)

		// Read remaining in buffer
		_, err = br.ReadFull(524) // 1024 - 500 = 524 (but needs to read more)
		assert.Nil(t, err)
	})

	t.Run("exact power of two", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 2048)
		br := NewReader(bytes.NewReader(data), 1024)

		ret, err := br.ReadFull(2048)
		require.NoError(t, err)
		assert.Equal(t, len(ret), 2048)
	})
}

func TestClose(t *testing.T) {
	t.Parallel()

	t.Run("double close", func(t *testing.T) {
		t.Parallel()

		br := NewReader(bytes.NewReader(nil), 1)
		err := br.Close()
		require.NoError(t, err)

		err = br.Close()
		assert.ErrorIs(t, err, ErrBufReaderAlreadyClosed)
	})

	t.Run("close with remaining buffer", func(t *testing.T) {
		t.Parallel()

		data := []byte{0x01, 0x02}
		br := NewReader(bytes.NewReader(data), 2)
		br.ReadByte()

		err := br.Close()
		assert.NoError(t, err)
	})
}

//nolint:paralleltest
func TestPoolConcurrency(t *testing.T) {
	var (
		goroutines = 1000
		iterations = 100
		dataSizes  = []int{1024, 4096, 16 << 10} // 1024, 4KB, 16KB
	)

	err := InitReaderPool([]int{512, 1024, 64 << 10})
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	for _, dataSize := range dataSizes {
		for i := range goroutines {
			wg.Add(1)

			go func(idx int) {
				defer wg.Done()

				for range iterations {
					data := make([]byte, dataSize)
					cc := rand.NewChaCha8([32]byte{byte(idx)})
					_, err := cc.Read(data)
					require.NoError(t, err)

					br := NewReader(bytes.NewReader(data), 1024)
					defer func() {
						err := br.Close()
						require.NoError(t, err)
					}()

					if rand.IntN(2) == 0 {
						if idx%2 == 0 {
							for range 10 {
								_, err := br.ReadFull(8)
								require.NoError(t, err)
							}
						} else {
							_, err := br.ReadFull(dataSize)
							require.NoError(t, err)
						}
					} else {
						result, err := br.ReadFull(dataSize)
						require.NoError(t, err)
						assert.Equal(t, result, data)
					}
				}
			}(i)
		}
	}

	wg.Wait()
}

func TestReadLoopAccuracy(t *testing.T) {
	t.Parallel()

	t.Run("small chunks", func(t *testing.T) {
		t.Parallel()

		const totalSize = 1 << 20 // 1MB
		data := make([]byte, totalSize)

		cc := rand.NewChaCha8([32]byte{})
		_, err := cc.Read(data) // 生成随机测试数据
		require.NoError(t, err)

		br := NewReader(bytes.NewReader(data), 1024)
		defer func() {
			err := br.Close()
			require.NoError(t, err)
		}()

		var readBuf bytes.Buffer

		remaining := totalSize
		for remaining > 0 {
			readSize := 512 + rand.IntN(512)
			if readSize > remaining {
				readSize = remaining
			}

			chunk, err := br.ReadFull(readSize)
			require.NoError(t, err)

			readBuf.Write(chunk)

			remaining -= readSize
		}

		assert.Equal(t, readBuf.Bytes(), data)
	})

	t.Run("large chunks with buffer growth", func(t *testing.T) {
		t.Parallel()

		const totalSize = 16 << 20 // 16MB

		data := make([]byte, totalSize)
		cc := rand.NewChaCha8([32]byte{})
		_, err := cc.Read(data)
		require.NoError(t, err)

		br := NewReader(bytes.NewReader(data), 1024)
		defer func() {
			err := br.Close()
			require.NoError(t, err)
		}()

		var readBuf bytes.Buffer

		remaining := totalSize
		for remaining > 0 {
			readSize := 4096 + rand.IntN(4096)
			if readSize > remaining {
				readSize = remaining
			}

			chunk, err := br.ReadFull(readSize)
			require.NoError(t, err)

			readBuf.Write(chunk)

			remaining -= readSize
		}

		assert.Equal(t, readBuf.Bytes(), data)
	})
}

func BenchmarkConcurrentReadFull(b *testing.B) {
	err := InitReaderPool([]int{1024, 65536, 128})
	require.NoError(b, err)

	const (
		goroutines    = 8
		chunkSize     = 4096
		totalDataSize = 64 << 20 // 64MB
	)

	data := make([]byte, totalDataSize)
	cc := rand.NewChaCha8([32]byte{})

	_, err = cc.Read(data)
	require.NoError(b, err)

	b.ResetTimer()
	b.SetParallelism(goroutines)
	b.RunParallel(func(pb *testing.PB) {
		br := NewReader(bytes.NewReader(data), chunkSize)

		for pb.Next() {
			if _, err := br.ReadFull(chunkSize); err != nil {
				if errors.Is(err, io.EOF) {
					br = NewReader(bytes.NewReader(data), chunkSize)
					continue
				}

				b.Fatal(err)
			}
		}
	})
}

func BenchmarkReadByte(b *testing.B) {
	data := make([]byte, 1<<20) // 1MB
	br := NewReader(bytes.NewReader(data), 4096)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		br.ReadByte()

		if i%4096 == 0 {
			br = NewReader(bytes.NewReader(data), 4096)
		}
	}
}

func BenchmarkReadFull(b *testing.B) {
	err := InitReaderPool([]int{1024, 131072, 128})
	require.NoError(b, err)

	sizes := []int{128, 256, 512, 512 + 128, 1024, 1024 + 128, 2048, 2048 + 128, 4096, 8192, 8192 + 1024, 16384, 32768, 65536, 65536 + 1024}
	data := make([]byte, 131072)

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()

			br := NewReader(bytes.NewReader(data), size)
			// Calculate how many reads we can do before needing to reset
			readsBeforeReset := len(data) / size

			for i := 0; i < b.N; i++ {
				if i%readsBeforeReset == 0 {
					br = NewReader(bytes.NewReader(data), size)
				}

				_, err := br.ReadFull(size)
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkReadFullAllocations(b *testing.B) {
	err := InitReaderPool([]int{1024, 65536, 128})
	require.NoError(b, err)

	data := make([]byte, 1<<20)
	br := NewReader(bytes.NewReader(data), 1024)

	b.ReportAllocs()
	b.ResetTimer()

	readsBeforeReset := len(data) / 1024

	for i := 0; i < b.N; i++ {
		if i%readsBeforeReset == 0 {
			br = NewReader(bytes.NewReader(data), 1024)
		}

		_, err := br.ReadFull(1024)
		assert.Nil(b, err)
	}
}
