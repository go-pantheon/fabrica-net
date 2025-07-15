package util

import (
	"context"
	"net"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
)

// TestInternalIP test internal ip detection
func TestInternalIP(t *testing.T) {
	t.Parallel()

	ip := InternalIP()

	// IP should not be empty, but this depends on the test environment
	// so only verify that the returned IP is valid
	if ip != "" {
		parsedIP := net.ParseIP(ip)
		assert.NotNil(t, parsedIP, "InternalIP should return a valid IP address")
		assert.NotNil(t, parsedIP.To4(), "InternalIP should return an IPv4 address")
		t.Logf("Internal IP detected: %s", ip)
	} else {
		t.Log("No internal IP detected, this might be normal in some environments")
	}
}

// TestExtract test ip address and port extraction
func TestExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		hostPort string
		want     string
		wantErr  bool
		errType  error
	}{
		{
			name:     "Invalid host:port format",
			hostPort: "invalid",
			want:     "",
			wantErr:  true,
			errType:  ErrInvalidHostPort,
		},
		{
			name:     "Specific IP with port",
			hostPort: "192.168.1.1:8080",
			want:     "192.168.1.1:8080",
			wantErr:  false,
		},
		{
			name:     "Wildcard IP with port",
			hostPort: "0.0.0.0:8080",
			wantErr:  false,
		},
		{
			name:     "IPv6 wildcard with port",
			hostPort: "[::]:8080",
			wantErr:  false,
		},
		{
			name:     "With listener overriding port",
			hostPort: "127.0.0.1:0",
			// here we only check if an error is returned
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Extract(tt.hostPort)
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				assert.NoError(t, err)

				if tt.want != "" {
					assert.Equal(t, tt.want, got)
				} else {
					// if no expected result is specified, at least ensure the result is not empty
					assert.NotEmpty(t, got)

					// check the result is a valid host:port format
					host, port, err := net.SplitHostPort(got)
					assert.NoError(t, err)
					assert.NotEmpty(t, host)
					assert.NotEmpty(t, port)
				}
			}
		})
	}
}

// TestIsPrivateIP test if ip is private
func TestIsPrivateIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
		want bool
	}{
		{"Invalid IP", "invalid-ip", false},
		{"Empty string", "", false},
		{"Loopback IPv4", "127.0.0.1", false}, // loopback address is not a private address
		{"Public IPv4", "8.8.8.8", false},
		{"Private IPv4 (10.x.x.x)", "10.0.0.1", true},
		{"Private IPv4 (172.16.x.x)", "172.16.0.1", true},
		{"Private IPv4 (172.31.x.x)", "172.31.255.255", true},
		{"Private IPv4 (192.168.x.x)", "192.168.1.1", true},
		{"Edge case (172.15.x.x)", "172.15.0.1", false},
		{"Edge case (172.32.x.x)", "172.32.0.1", false},
		{"IPv6 Loopback", "::1", false},
		{"IPv6 Private", "fc00::1", true},
		{"IPv6 Public", "2001:db8::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isPrivateIP(tt.addr)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestGetClientIP test ip extraction from context
func TestGetClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name:    "nil context",
			headers: nil,
			want:    "",
		},
		{
			name:    "no headers",
			headers: map[string]string{},
			want:    "",
		},
		{
			name: "X-Forwarded-For single IP",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
			},
			want: "192.168.1.1",
		},
		{
			name: "X-Forwarded-For multiple IPs",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1, 10.0.0.1, 172.16.0.1",
			},
			want: "192.168.1.1",
		},
		{
			name: "X-Real-IP",
			headers: map[string]string{
				"X-Real-IP": "10.0.0.1",
			},
			want: "10.0.0.1",
		},
		{
			name: "X-Forwarded-For takes precedence over X-Real-IP",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
				"X-Real-IP":       "10.0.0.1",
			},
			want: "192.168.1.1",
		},
		{
			name: "X-Forwarded-For with spaces",
			headers: map[string]string{
				"X-Forwarded-For": " 192.168.1.1 ",
			},
			want: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var ctx context.Context
			if tt.name == "nil context" {
				ctx = nil
			} else {
				ctx = mockServerContext(tt.headers)
			}

			got := GetClientIP(ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

var _ transport.Header = (*mockTransport)(nil)

// mockTransport implement transport.Transporter interface for testing
type mockTransport struct {
	headers map[string]string
}

func (m *mockTransport) Add(key, value string) {
	if m.headers == nil {
		m.headers = make(map[string]string)
	}

	m.headers[key] = value
}

func (m *mockTransport) Kind() transport.Kind {
	return transport.KindHTTP
}

func (m *mockTransport) Endpoint() string {
	return "mock-endpoint"
}

func (m *mockTransport) Operation() string {
	return "mock-operation"
}

func (m *mockTransport) RequestHeader() transport.Header {
	return m
}

func (m *mockTransport) ReplyHeader() transport.Header {
	return m
}

func (m *mockTransport) Get(key string) string {
	return m.headers[key]
}

func (m *mockTransport) Set(key, value string) {
	if m.headers == nil {
		m.headers = make(map[string]string)
	}

	m.headers[key] = value
}

func (m *mockTransport) Keys() []string {
	keys := make([]string, 0, len(m.headers))
	for k := range m.headers {
		keys = append(keys, k)
	}

	return keys
}

// Values implement transport.Header interface
func (m *mockTransport) Values(key string) []string {
	if value, ok := m.headers[key]; ok {
		return []string{value}
	}

	return nil
}

// mockServerContext create context with mock transport
func mockServerContext(headers map[string]string) context.Context {
	mt := &mockTransport{headers: headers}
	ctx := context.Background()

	return transport.NewServerContext(ctx, mt)
}
