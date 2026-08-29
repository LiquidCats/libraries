package jsonrpc_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	jsonrpc "github.com/LiquidCats/libraries/jsonrpc"
	"github.com/LiquidCats/libraries/jsonrpc/tests/types"
)

var benchmarkResult *types.Block

func BenchmarkExecuteLargeResponse(b *testing.B) {
	fixture, err := os.ReadFile("tests/fixtures/btc-block-without-txs.json")
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}

	req := jsonrpc.NewRequest(
		"getblock",
		[]any{"00000000000000000001246cadf2834cf70fe92404e37c14e071f4b7da61993d", false},
		jsonrpc.WithRPCid[[]any, types.Block]("bench"),
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := server.Client()

	b.ReportAllocs()

	for b.Loop() {
		prepared := req.Prepare(server.URL)

		res, execErr := prepared.Execute(client)
		if execErr != nil {
			b.Fatalf("execute: %v", execErr)
		}
		if res == nil || res.Hash == "" {
			b.Fatal("unexpected empty result")
		}
		benchmarkResult = res
	}
}
