package jsonrpc_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	rpc "github.com/LiquidCats/libraries/jsonrpc/v2"
)

var benchmarkRequest *rpc.Request[int]

func BenchmarkNewRequest(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchmarkRequest = rpc.NewRequest[int]("sum", []any{1, 2}, rpc.WithRPCid[int]("bench"))
	}
}

// In-memory transport isolates JSON and client overhead from network latency.
func BenchmarkExecute(b *testing.B) {
	for _, size := range []struct {
		name        string
		bytes       int
		knownLength bool
	}{
		{"small", 64, false},
		{"64KiB", 64 << 10, false},
		{"1MiB", 1 << 20, false},
		{"1MiB_content_length", 1 << 20, true},
	} {
		b.Run(size.name, func(b *testing.B) {
			payload := strings.Repeat("x", size.bytes)
			response := `{"jsonrpc":"2.0","id":"bench","result":"` + payload + `"}`
			contentLength := int64(-1)
			if size.knownLength {
				contentLength = int64(len(response))
			}
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				_, err := io.Copy(io.Discard, r.Body)
				_ = r.Body.Close()
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header), ContentLength: contentLength}, err
			})}
			request := rpc.NewRequest[string]("echo", []any{payload}, rpc.WithRPCid[string]("bench"))
			b.ReportAllocs()
			b.SetBytes(int64(len(response)))
			b.ResetTimer()
			for b.Loop() {
				got, err := request.Execute(context.Background(), "http://example.test", rpc.WithClient[string](client))
				if err != nil || len(got) != size.bytes {
					b.Fatalf("result length=%d, error=%v", len(got), err)
				}
			}
		})
	}
}
