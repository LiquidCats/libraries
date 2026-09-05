package jsonrpc

import "net/http"

// DefaultMaxResponseBytes limits the decompressed HTTP response to 64 MiB.
const DefaultMaxResponseBytes int64 = 64 << 20

type RequestOption[Resp any] func(*rpcRequest[Resp])

func WithRPCVersion[Resp any](version string) RequestOption[Resp] {
	return func(req *rpcRequest[Resp]) {
		req.JSONRPC = version
	}
}

func WithRPCid[Resp any](id any) RequestOption[Resp] {
	return func(req *rpcRequest[Resp]) {
		req.ID = id
	}
}

type executeConfig struct {
	client           *http.Client
	headers          http.Header
	maxResponseBytes int64
}

type ExecuteOption[Result any] func(*executeConfig)

// WithClient replaces the default client, including its timeout policy.
// A nil client leaves the current client unchanged.
func WithClient[Result any](client *http.Client) ExecuteOption[Result] {
	return func(cfg *executeConfig) {
		if client != nil {
			cfg.client = client
		}
	}
}

// WithMaxResponseBytes sets the maximum decompressed response size.
// Execute rejects non-positive limits and math.MaxInt64.
func WithMaxResponseBytes[Result any](limit int64) ExecuteOption[Result] {
	return func(cfg *executeConfig) {
		cfg.maxResponseBytes = limit
	}
}

func WithHeader[Result any](key, value string) ExecuteOption[Result] {
	return func(cfg *executeConfig) {
		cfg.headers.Add(key, value)
	}
}
