package jsonrpc_test

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	rpc "github.com/LiquidCats/libraries/jsonrpc/v2"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func TestNewRequest(t *testing.T) {
	r := rpc.NewRequest[int]("sum", []any{1, 2})
	if r.Method != "sum" || r.JSONRPC != rpc.Version || r.ID == "" || len(r.Params) != 2 {
		t.Fatalf("unexpected request: %+v", r)
	}
	custom := rpc.NewRequest[int]("sum", nil, rpc.WithRPCVersion[int]("1.0"), rpc.WithRPCid[int](42), rpc.WithRPCid[int](43))
	if custom.ID != 43 || custom.JSONRPC != "1.0" {
		t.Fatalf("options not applied: %+v", custom)
	}
	data, err := json.Marshal(custom)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "params") {
		t.Fatalf("empty params not omitted: %s", data)
	}
}

func TestExecuteHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		var request struct {
			Method  string `json:"method"`
			Params  []int  `json:"params"`
			ID      string `json:"id"`
			JSONRPC string `json:"jsonrpc"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Method != "sum" || request.ID != "test" || request.JSONRPC != "2.0" || len(request.Params) != 2 {
			t.Errorf("request = %+v", request)
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":"test","result":{"total":3}}`)
	}))
	defer server.Close()
	type result struct{ Total int }
	req := rpc.NewRequest[result]("sum", []any{1, 2}, rpc.WithRPCid[result]("test"))
	// Exercise both the shared default client and the injected client.
	for _, options := range [][]rpc.ExecuteOption[result]{nil, {rpc.WithClient[result](server.Client()), rpc.WithClient[result](nil)}} {
		got, err := req.Execute(context.Background(), server.URL, options...)
		if err != nil || got.Total != 3 {
			t.Fatalf("Execute = %+v, %v", got, err)
		}
	}
}

func TestExecuteFailures(t *testing.T) {
	sentinel := errors.New("transport failed")
	cases := []struct {
		name, url, body, want string
		params                []any
		transportErr          error
		cancel                bool
	}{
		{name: "encode", url: "http://example.test", params: []any{make(chan int)}, want: "failed to encode request"},
		{name: "url", url: "://", want: "failed to create request"},
		{name: "transport", url: "http://example.test", transportErr: sentinel, want: "failed to execute request"},
		{name: "cancelled", url: "http://example.test", cancel: true, want: "failed to execute request"},
		{name: "malformed", url: "http://example.test", body: `{"result":`, want: "failed to decode response"},
		{name: "empty", url: "http://example.test", want: "failed to decode response"},
		{name: "wrong result type", url: "http://example.test", body: `{"result":"text"}`, want: "failed to decode response"},
		{name: "trailing JSON", url: "http://example.test", body: `{"result":7}{}`, want: "failed to decode response"},
		{name: "trailing garbage", url: "http://example.test", body: `{"result":7}garbage`, want: "failed to decode response"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := &trackedBody{Reader: strings.NewReader(tc.body)}
			called := false
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				called = true
				_ = r.Body.Close()
				if tc.transportErr != nil {
					return nil, tc.transportErr
				}
				if err := r.Context().Err(); err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header)}, nil
			})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			got, err := rpc.NewRequest[int]("test", tc.params).Execute(ctx, tc.url, rpc.WithClient[int](client))
			if got != 0 || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Execute = %d, %v; want %s", got, err, tc.want)
			}
			if tc.transportErr != nil && !errors.Is(err, sentinel) {
				t.Errorf("lost cause: %v", err)
			}
			if tc.cancel && !errors.Is(err, context.Canceled) {
				t.Errorf("lost cancellation: %v", err)
			}
			if called && tc.transportErr == nil && !tc.cancel && !body.closed {
				t.Error("response body not closed")
			}
		})
	}
}

func TestExecuteConcurrent(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		var request struct {
			Params []int `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return nil, err
		}
		data, err := json.Marshal(map[string]any{"result": request.Params[0]})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, err
	})}
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := rpc.NewRequest[int]("echo", []any{i}).Execute(context.Background(), "http://example.test", rpc.WithClient[int](client))
			if err != nil || got != i {
				t.Errorf("Execute = %d, %v; want %d", got, err, i)
			}
		}()
	}
	wg.Wait()
}

func TestExecuteRemoteErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"RPC error", `{"error":{"code":-32601,"message":"missing"}}`, "jsonrpc error", 200},
		{"HTTP error", `{"result":7}`, "500 Internal Server Error", 500},
		{"HTTP HTML error", `<html>unauthorized</html>`, "401 Unauthorized", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &trackedBody{Reader: strings.NewReader(tc.body)}
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				defer r.Body.Close()
				if got := r.Header.Values("X-Test"); !reflect.DeepEqual(got, []string{"one", "two"}) {
					t.Errorf("X-Test = %v", got)
				}
				if got := r.Header.Get("Content-Type"); got != "application/custom+json" {
					t.Errorf("Content-Type = %q", got)
				}
				return &http.Response{StatusCode: tc.status, Body: body, Header: make(http.Header)}, nil
			})}
			got, err := rpc.NewRequest[int]("test", nil).Execute(context.Background(), "http://example.test", rpc.WithClient[int](client), rpc.WithHeader[int]("X-Test", "one"), rpc.WithHeader[int]("X-Test", "two"), rpc.WithHeader[int]("Content-Type", "application/custom+json"))
			if got != 0 || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Execute = %d, %v; want %s", got, err, tc.want)
			}
			if tc.status == http.StatusOK {
				var rpcErr *rpc.RPCError
				if !errors.As(err, &rpcErr) || rpcErr.Code != -32601 || rpcErr.Message != "missing" {
					t.Fatalf("lost RPC error: %v", err)
				}
			}
			if !body.closed {
				t.Error("response body not closed")
			}
		})
	}
}

func TestRPCError(t *testing.T) {
	err := &rpc.RPCError{Code: -32601, Message: "Method not found"}
	if got, want := err.Error(), "jsonrpc error: code=-32601, message=Method not found"; got != want {
		t.Fatalf("Error = %q, want %q", got, want)
	}
	var response rpc.RPCResponse[int]
	if e := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`), &response); e != nil {
		t.Fatal(e)
	}
	if response.Error == nil || *response.Error != *err || response.JSONRPC != "2.0" {
		t.Fatalf("response = %+v", response)
	}
}

func TestExecuteRequestBodyOutlivesTransportError(t *testing.T) {
	sentinel := errors.New("connection failed")
	var requests []*http.Request
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		// RoundTripper is allowed to close the request asynchronously after returning.
		requests = append(requests, r)
		return nil, sentinel
	})}
	t.Cleanup(func() {
		for _, r := range requests {
			_ = r.Body.Close()
		}
	})
	for i := range 16 {
		_, err := rpc.NewRequest[int]("echo", []any{i}).Execute(t.Context(), "http://example.test", rpc.WithClient[int](client))
		if !errors.Is(err, sentinel) {
			t.Fatalf("error = %v", err)
		}
	}
	for i, r := range requests {
		replay, err := r.GetBody()
		if err != nil {
			t.Fatal(err)
		}
		for _, body := range []io.ReadCloser{r.Body, replay} {
			var request struct {
				Params []int `json:"params"`
			}
			err = json.NewDecoder(body).Decode(&request)
			_ = body.Close()
			if err != nil || !reflect.DeepEqual(request.Params, []int{i}) {
				t.Fatalf("request %d corrupted: %+v, %v", i, request, err)
			}
		}
	}
}

func TestExecuteResponseLimits(t *testing.T) {
	const response = `{"result":7}`
	for _, tc := range []struct {
		name          string
		limit, length int64
		body          string
		wantError     bool
	}{
		{"exact", int64(len(response)), -1, response, false},
		{"under", 128, int64(len(response)), response, false},
		{"unknown length overflow", 4, -1, response, true},
		{"known length overflow", 4, int64(len(response)), response, true},
		{"trailing whitespace overflow", int64(len(response)), -1, response + " ", true},
		{"zero limit", 0, -1, response, true},
		{"negative limit", -1, -1, response, true},
		{"overflowing limit", math.MaxInt64, -1, response, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.body)
			body := &trackedBody{Reader: reader}
			called := false
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				called = true
				_ = r.Body.Close()
				return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: tc.length}, nil
			})}
			got, err := rpc.NewRequest[int]("test", nil).Execute(t.Context(), "http://example.test", rpc.WithClient[int](client), rpc.WithMaxResponseBytes[int](tc.limit))
			if (err != nil) != tc.wantError || (err == nil && got != 7) || (err != nil && got != 0) {
				t.Fatalf("Execute = %d, %v", got, err)
			}
			if called && !body.closed {
				t.Error("body not closed")
			}
			switch {
			case tc.limit <= 0 || tc.limit == math.MaxInt64:
				if called {
					t.Error("invalid limit should fail before transport")
				}
			case tc.length > tc.limit:
				if reader.Len() != len(tc.body) {
					t.Error("oversized content length should fail before reading")
				}
			case int64(len(tc.body)-reader.Len()) > tc.limit+1:
				t.Error("read beyond limit+1")
			}
		})
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestExecuteDefaultResponseLimit(t *testing.T) {
	body := &trackedBody{Reader: errorReader{errors.New("oversized body must not be read")}}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		_ = r.Body.Close()
		return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: rpc.DefaultMaxResponseBytes + 1}, nil
	})}
	got, err := rpc.NewRequest[int]("test", nil).Execute(t.Context(), "http://example.test", rpc.WithClient[int](client))
	if got != 0 || err == nil || !strings.Contains(err.Error(), "response exceeds") || !body.closed {
		t.Fatalf("Execute = %d, %v; closed=%v", got, err, body.closed)
	}
}

func TestExecuteReadError(t *testing.T) {
	sentinel := errors.New("body interrupted")
	body := &trackedBody{Reader: io.MultiReader(strings.NewReader(`{"result":7}`), errorReader{sentinel})}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		_ = r.Body.Close()
		return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
	})}
	got, err := rpc.NewRequest[int]("test", nil).Execute(t.Context(), "http://example.test", rpc.WithClient[int](client))
	if got != 0 || !errors.Is(err, sentinel) || !body.closed {
		t.Fatalf("Execute = %d, %v; closed=%v", got, err, body.closed)
	}
}

func TestExecuteCompressedResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		writer := gzip.NewWriter(w)
		_, _ = io.WriteString(writer, `{"result":"`+strings.Repeat("x", 4096)+`"}`)
		_ = writer.Close()
	}))
	defer server.Close()
	got, err := rpc.NewRequest[string]("test", nil).Execute(t.Context(), server.URL, rpc.WithMaxResponseBytes[string](1024))
	if got != "" || err == nil || !strings.Contains(err.Error(), "exceeds 1024 bytes") {
		t.Fatalf("Execute = %q, %v", got, err)
	}
}

func TestExecuteConnectionReuse(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "malformed"}[malformed], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if malformed {
					_, _ = io.WriteString(w, "invalid")
				} else {
					_, _ = io.WriteString(w, `{"result":7}`)
				}
				_, _ = io.WriteString(w, strings.Repeat(" ", 64<<10))
			}))
			defer server.Close()
			request := rpc.NewRequest[int]("test", nil)
			for i := range 2 {
				reused := false
				ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused }})
				_, err := request.Execute(ctx, server.URL, rpc.WithClient[int](server.Client()))
				if (err != nil) != malformed {
					t.Fatalf("error = %v", err)
				}
				if i == 1 && !reused {
					t.Error("connection not reused")
				}
			}
		})
	}
}

func TestExecuteDeadline(t *testing.T) {
	for _, flushHeaders := range []bool{false, true} {
		t.Run(map[bool]string{false: "headers", true: "body"}[flushHeaders], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if flushHeaders {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			_, err := rpc.NewRequest[int]("test", nil).Execute(ctx, server.URL)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
