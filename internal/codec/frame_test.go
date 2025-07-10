package codec

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
			writer := bufio.NewWriter(&buf)

			err := Encode(writer, xnet.Pack(tc.data))
			require.NoError(t, err)

			// Decode
			reader := bytes.NewReader(buf.Bytes())
			decoded, free, err := Decode(reader)
			require.NoError(t, err)

			defer free()

			// Verify
			assert.Equal(t, tc.data, []byte(decoded))
		})
	}
}

//nolint:paralleltest
func TestDecodeInvalidPacket(t *testing.T) {
	// Test with invalid packet length
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	// Write invalid length (too large)
	invalidLen := xnet.MaxPackSize + 100
	_, err := writer.Write([]byte{
		byte(invalidLen >> 24),
		byte(invalidLen >> 16),
		byte(invalidLen >> 8),
		byte(invalidLen),
	})
	require.NoError(t, err)
	err = writer.Flush()
	require.NoError(t, err)

	reader := bytes.NewReader(buf.Bytes())
	_, _, err = Decode(reader)
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
		writer := bufio.NewWriter(&buf)

		for pb.Next() {
			buf.Reset()
			writer.Reset(&buf)

			// Encode
			err := Encode(writer, xnet.Pack(data))
			if err != nil {
				b.Fatal(err)
			}

			// Decode
			reader := bytes.NewReader(buf.Bytes())

			decoded, free, err := Decode(reader)
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

func TestInitRingPool(t *testing.T) {
	t.Parallel()
	// Test custom configuration
	customConfig := map[int]uint64{
		32:  512,
		64:  256,
		128: 128,
	}

	err := InitRingPool(customConfig)
	require.NoError(t, err)

	// Test allocation with custom config
	testData := make([]byte, 50) // Should use 64-byte pool

	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	err = Encode(writer, xnet.Pack(testData))
	require.NoError(t, err)

	reader := bytes.NewReader(buf.Bytes())
	decoded, free, err := Decode(reader)
	require.NoError(t, err)

	defer free()

	assert.Equal(t, len(testData), len(decoded))
}
