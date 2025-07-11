package frame

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeWithRingPool(t *testing.T) {
	t.Parallel()

	// Test data of various sizes to exercise different ring pool sizes
	testCases := []struct {
		name string
		data []byte
	}{
		{"small", make([]byte, 32)},   // Uses 64-byte ring
		{"medium", make([]byte, 100)}, // Uses 128-byte ring
		{"large", make([]byte, 200)},  // Uses 256-byte ring
		{"xlarge", make([]byte, 400)}, // Uses 512-byte ring
		{"huge", make([]byte, 800)},   // Uses 1024-byte ring
		{"giant", make([]byte, 2000)}, // Uses 4096-byte ring
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Fill test data with pattern
			for i := range tc.data {
				tc.data[i] = byte(i % 256)
			}

			// Encode
			var buf bytes.Buffer

			codec := &Codec{
				w: bufio.NewWriter(&buf),
				r: bufio.NewReader(&buf),
			}

			err := codec.Encode(xnet.Pack(tc.data))
			require.NoError(t, err)

			// Decode
			decoded, free, err := codec.Decode()
			require.NoError(t, err)

			defer free()

			// Verify
			assert.Equal(t, tc.data, []byte(decoded))
		})
	}
}

//nolint:paralleltest
func TestDecodeInvalidPacket(t *testing.T) {
	var buf bytes.Buffer

	c := &Codec{
		w: bufio.NewWriter(&buf),
		r: bufio.NewReader(&buf),
	}

	// Write invalid length (too large)
	invalidLen := xnet.MaxPackSize + 1000
	_, err := c.w.Write([]byte{
		byte(invalidLen >> 24),
		byte(invalidLen >> 16),
		byte(invalidLen >> 8),
		byte(invalidLen),
	})
	require.NoError(t, err)
	err = c.w.Flush()
	require.NoError(t, err)

	_, _, err = c.Decode()
	assert.ErrorIs(t, err, ErrInvalidPackLen)
}

func BenchmarkEncodeDecodeWithRingPool(b *testing.B) {
	data := make([]byte, 512) // Common gaming packet size
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var buf bytes.Buffer

		c := &Codec{
			w: bufio.NewWriter(&buf),
			r: bufio.NewReader(&buf),
		}

		for pb.Next() {
			buf.Reset()
			c.w.Reset(&buf)

			// Encode
			err := c.Encode(xnet.Pack(data))
			if err != nil {
				b.Fatal(err)
			}

			// Decode
			decoded, free, err := c.Decode()
			if err != nil {
				b.Fatal(err)
			}

			// Verify length
			if len(decoded) != len(data) {
				b.Fatal("length mismatch")
			}

			free()
		}
	})
}

func TestInitTcpRingPool(t *testing.T) {
	t.Parallel()
	// Test custom configuration
	customConfig := map[int]uint64{
		32:  512,
		64:  256,
		128: 128,
	}

	err := InitTcpRingPool(customConfig)
	require.NoError(t, err)

	// Test allocation with custom config
	testData := make([]byte, 50) // Should use 64-byte pool

	var buf bytes.Buffer

	c := &Codec{
		w: bufio.NewWriter(&buf),
		r: bufio.NewReader(&buf),
	}

	err = c.Encode(xnet.Pack(testData))
	require.NoError(t, err)

	decoded, free, err := c.Decode()
	require.NoError(t, err)

	defer free()

	assert.Equal(t, len(testData), len(decoded))
}
