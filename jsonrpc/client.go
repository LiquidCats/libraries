package jsonrpc

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

const (
	dialTimeout      = 5 * time.Second
	dialKeepAlive    = 30 * time.Second
	tlsHandshakeTime = 5 * time.Second
	idleConnTimeout  = 90 * time.Second
	expectContinue   = 250 * time.Millisecond

	// Large response tuning: allow many idle conns but cap concurrency.
	maxIdleConns        = 4_096
	maxIdleConnsPerHost = 1_024
	// maxConnsPerHost bounds memory while handling many large bodies.
	maxConnsPerHost = 512

	// ioBufferSize raises the per-connection IO buffers from the 4KB default
	// to cut syscalls on large bodies. 64KB is a common page multiple.
	ioBufferSize = 64 << 10

	// tlsSessionCacheSize lowers handshake CPU when many connections exist.
	tlsSessionCacheSize = 4096
)

var defaultHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: dialKeepAlive,
		}).DialContext,
		ForceAttemptHTTP2: true,

		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConnsPerHost,
		MaxConnsPerHost:     maxConnsPerHost,

		// Keep generous idle timeout for reuse; large responses take longer
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTime,
		ExpectContinueTimeout: expectContinue,

		// Enable compression to save bandwidth for multi-MB JSON; CPU tradeoff
		DisableCompression: false,

		ReadBufferSize:  ioBufferSize,
		WriteBufferSize: ioBufferSize,

		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			ClientSessionCache: tls.NewLRUClientSessionCache(tlsSessionCacheSize),
		},

		// HTTP/2: raise concurrent streams per connection for multiplexing large responses
		// (Go picks defaults; env GODEBUG may tune; leaving default to avoid incompat issues)
	},
	Timeout: 0,
}
