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
	requestTimeout   = 30 * time.Second
	headerTimeout    = 10 * time.Second

	// Keep a modest idle pool; active connection limits do not bound HTTP/2 streams.
	maxIdleConns        = 100
	maxIdleConnsPerHost = 16
	maxConnsPerHost     = 512

	// tlsSessionCacheSize lowers handshake CPU when many connections exist.
	tlsSessionCacheSize = 100
)

var defaultHTTPTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: dialKeepAlive,
	}).DialContext,
	ForceAttemptHTTP2: true,

	MaxIdleConns:        maxIdleConns,
	MaxIdleConnsPerHost: maxIdleConnsPerHost,
	MaxConnsPerHost:     maxConnsPerHost,

	IdleConnTimeout:       idleConnTimeout,
	TLSHandshakeTimeout:   tlsHandshakeTime,
	ExpectContinueTimeout: expectContinue,
	ResponseHeaderTimeout: headerTimeout,

	// Compression trades CPU for bandwidth. Execute bounds decompressed bytes.
	DisableCompression: false,

	TLSClientConfig: &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ClientSessionCache: tls.NewLRUClientSessionCache(tlsSessionCacheSize),
	},
}

var defaultHTTPClient = &http.Client{
	Transport: defaultHTTPTransport,
	Timeout:   requestTimeout,
}
