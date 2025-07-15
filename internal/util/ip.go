// Package ip provides utilities for IP address operations, including
// detection of internal IPs, extracting addresses from connections,
// and client IP identification in request contexts.
package util

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/go-kratos/kratos/v2/transport"
)

var (
	// ErrInvalidHostPort is returned when the host:port string cannot be parsed.
	ErrInvalidHostPort = errors.New("invalid host:port format")

	// ErrNoPrivateIPFound is returned when no private IP could be identified.
	ErrNoPrivateIPFound = errors.New("no private IP found")

	// ErrInterfaceLookup is returned when network interfaces cannot be enumerated.
	ErrInterfaceLookup = errors.New("failed to lookup network interfaces")

	// ErrInvalidIPFormat is returned when an IP address has invalid format.
	ErrInvalidIPFormat = errors.New("invalid IP format")
	// ErrIPComponentOutOfRange is returned when an IP component is out of valid range.
	ErrIPComponentOutOfRange = errors.New("IP component out of range")
)

func RemoteAddr(conn net.Conn) string {
	if conn == nil {
		return ""
	}

	addr := conn.RemoteAddr()
	if addr == nil {
		return ""
	}

	return addr.String()
}

func LocalAddr(conn net.Conn) string {
	if conn == nil {
		return ""
	}

	addr := conn.LocalAddr()
	if addr == nil {
		return ""
	}

	return addr.String()
}

// InternalIP returns the first detected internal IPv4 address.
// Returns an empty string if no internal IP could be determined.
func InternalIP() string {
	inters, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, inter := range inters {
		// Skip interfaces that are down or loopback
		if inter.Flags&net.FlagUp == 0 || strings.HasPrefix(inter.Name, "lo") {
			continue
		}

		addrs, err := inter.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() {
				continue
			}

			// Check for IPv4 address
			if ipv4 := ipNet.IP.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}

	return ""
}

// Extract returns a private address and port from the given hostPort string.
// If a listener is provided, its actual port will be used.
// Returns an error if no suitable address could be determined.
func Extract(hostPort string) (string, error) {
	addr, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidHostPort, err)
	}

	// If a specific address is provided and it's not a placeholder, use it
	if addr != "" && addr != "0.0.0.0" && addr != "[::]" && addr != "::" {
		return net.JoinHostPort(addr, port), nil
	}

	// Otherwise, find a private IP address
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInterfaceLookup, err)
	}

	for _, iface := range ifaces {
		// Skip interfaces that are down or loopback
		if iface.Flags&net.FlagUp == 0 || (iface.Flags&net.FlagLoopback != 0) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, rawAddr := range addrs {
			var ip net.IP
			switch addr := rawAddr.(type) {
			case *net.IPAddr:
				ip = addr.IP
			case *net.IPNet:
				ip = addr.IP
			default:
				continue
			}

			if isPrivateIP(ip.String()) {
				return net.JoinHostPort(ip.String(), port), nil
			}
		}
	}

	return "", ErrNoPrivateIPFound
}

// isPrivateIP determines if an IP address is within private address ranges.
// Supports both IPv4 and IPv6 addresses.
func isPrivateIP(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}

	if ip4 := ip.To4(); ip4 != nil {
		// IPv4 private ranges:
		// 10.0.0.0/8
		// 172.16.0.0/12
		// 192.168.0.0/16
		return ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1]&0xf0 == 16) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}

	// IPv6 private range: FC00::/7
	return len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc
}

// GetClientIP extracts the client IP address from a request context.
// Attempts to use X-Forwarded-For or X-Real-IP headers if present.
// Returns an empty string if no client IP could be determined.
func GetClientIP(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}

	// Try X-Forwarded-For header first
	if v := tr.RequestHeader().Get("X-Forwarded-For"); v != "" {
		// X-Forwarded-For can contain multiple IPs, use the first one
		parts := strings.Split(v, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Fall back to X-Real-IP header
	if v := tr.RequestHeader().Get("X-Real-IP"); v != "" {
		return v
	}

	return ""
}
