package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/chirs/stordiag/internal/driver"
)

func ProbeNetwork(ctx context.Context, drv driver.Driver) *LayerReport {
	r := &LayerReport{Layer: LayerNetwork}

	// POSIX is local — skip network layer
	if drv.Type() == "posix" {
		r.Add("network", "N/A", 0, "local filesystem — no network", "")
		return r
	}

	host, port := parseEndpoint(drv)
	if host == "" {
		r.Error("resolve_endpoint", 0, fmt.Errorf("cannot parse endpoint from %s", drv))
		return r
	}

	// --- DNS ---
	start := time.Now()
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	dnsDur := time.Since(start)
	if err != nil {
		r.Error("dns_lookup", dnsDur, err)
	} else {
		r.Add("dns_lookup", "OK", dnsDur, fmt.Sprintf("%v", addrs), "")
	}

	// --- TCP connect ---
	if len(addrs) == 0 {
		r.Error("tcp_connect", 0, fmt.Errorf("no addresses to connect"))
		return r
	}

	target := net.JoinHostPort(addrs[0], port)
	start = time.Now()
	conn, err := net.DialTimeout("tcp", target, 5*time.Second)
	tcpDur := time.Since(start)
	if err != nil {
		r.Error("tcp_connect", tcpDur, fmt.Errorf("dial %s: %w", target, err))
		return r
	}
	r.Add("tcp_connect", "OK", tcpDur, target, "")
	defer conn.Close()

	// --- TLS handshake ---
	tlsUsed := false
	if _, ok := conn.(*tls.Conn); ok {
		tlsUsed = true
	} else if isSecureEndpoint(drv) {
		start = time.Now()
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, InsecureSkipVerify: false})
		err := tlsConn.HandshakeContext(ctx)
		tlsDur := time.Since(start)
		if err != nil {
			r.Error("tls_handshake", tlsDur, err)
		} else {
			r.Add("tls_handshake", "OK", tlsDur,
				fmt.Sprintf("version=%s cipher=%s", tlsVersion(tlsConn), tls.CipherSuiteName(tlsConn.ConnectionState().CipherSuite)),
				"")
			tlsUsed = true
		}
	}

	_ = tlsUsed

	// --- Round-trip time (RTT) ---
	// For POSIX drivers, network info is N/A
	if drv.Type() == "posix" {
		r.Add("note", "OK", 0, "localhost — network stats N/A", "")
	}

	return r
}

func parseEndpoint(drv driver.Driver) (host, port string) {
	s := drv.String()
	if drv.Type() == "posix" {
		return "", ""
	}
	// format: s3://bucket@host:port or host:port/bucket
	s = strings.TrimPrefix(s, "s3://")
	if idx := strings.Index(s, "@"); idx > 0 {
		s = s[idx+1:]
	}
	if idx := strings.Index(s, "/"); idx > 0 {
		s = s[:idx]
	}
	if h, p, err := net.SplitHostPort(s); err == nil {
		return h, p
	}
	// assume default S3 port
	if strings.Contains(s, ".") || s != "" {
		return s, "443"
	}
	return "", ""
}

func isSecureEndpoint(drv driver.Driver) bool {
	if s, ok := drv.(interface{ Endpoint() string }); ok {
		ep := s.Endpoint()
		return strings.HasPrefix(ep, "https://") || strings.Contains(ep, ":443")
	}
	return false
}

func tlsVersion(c *tls.Conn) string {
	v := c.ConnectionState().Version
	switch v {
	case tls.VersionTLS13:
		return "TLS1.3"
	case tls.VersionTLS12:
		return "TLS1.2"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}
