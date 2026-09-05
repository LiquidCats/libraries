package jsonrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
)

const Version = "2.0"

type rpcRequest[Result any] struct {
	Method  string `json:"method"`
	Params  []any  `json:"params,omitempty"`
	ID      any    `json:"id"`
	JSONRPC string `json:"jsonrpc"`
}

type Request[Result any] struct {
	rpcRequest[Result]
}

func NewRequest[Result any](
	method string,
	params []any,
	opts ...RequestOption[Result],
) *Request[Result] {
	req := rpcRequest[Result]{
		ID:      strconv.FormatInt(time.Now().UnixNano(), 10),
		Method:  method,
		JSONRPC: Version,
		Params:  params,
	}

	for _, opt := range opts {
		opt(&req)
	}

	return &Request[Result]{
		rpcRequest: req,
	}
}

// Copy decoded strings so a small result cannot retain a large response body.
var responseJSON = sonic.Config{CopyString: true}.Froze()

func (r Request[Result]) Execute(ctx context.Context, url string, opts ...ExecuteOption[Result]) (Result, error) {
	var zero Result

	cfg := executeConfig{
		client:           defaultHTTPClient,
		headers:          make(http.Header),
		maxResponseBytes: DefaultMaxResponseBytes,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.maxResponseBytes <= 0 || cfg.maxResponseBytes == math.MaxInt64 {
		return zero, fmt.Errorf("max response bytes must be between 1 and %d", int64(math.MaxInt64-1))
	}

	// The transport may still own the body after Do returns an error.
	// Give each request its own bytes, including redirect/retry replays.
	data, err := sonic.Marshal(r.rpcRequest)
	if err != nil {
		return zero, fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return zero, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header = cfg.headers
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := cfg.client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Do not read an untrusted error body just to report its status.
		return zero, fmt.Errorf("unexpected HTTP status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	if resp.ContentLength > cfg.maxResponseBytes {
		return zero, fmt.Errorf("response exceeds %d bytes", cfg.maxResponseBytes)
	}

	// Read through EOF for connection reuse and reject trailing JSON/data.
	// Limit the decompressed body too, including chunked/unknown-length bodies.
	var body bytes.Buffer
	if resp.ContentLength > 0 && resp.ContentLength <= cfg.maxResponseBytes-bytes.MinRead {
		// Reserve room for ReadFrom's final EOF probe to avoid another growth.
		body.Grow(int(resp.ContentLength) + bytes.MinRead)
	}
	if _, err = body.ReadFrom(io.LimitReader(resp.Body, cfg.maxResponseBytes+1)); err != nil {
		return zero, fmt.Errorf("failed to read response: %w", err)
	}
	if int64(body.Len()) > cfg.maxResponseBytes {
		return zero, fmt.Errorf("response exceeds %d bytes", cfg.maxResponseBytes)
	}

	var rpcResp RPCResponse[Result]
	err = responseJSON.Unmarshal(body.Bytes(), &rpcResp)
	if err != nil {
		return zero, fmt.Errorf("failed to decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return zero, rpcResp.Error
	}

	return rpcResp.Result, nil
}
