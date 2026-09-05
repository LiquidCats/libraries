# JSON-RPC client for Go

A generic HTTP client using Sonic for JSON encoding and decoding.

```sh
go get github.com/LiquidCats/libraries/jsonrpc/v2
```

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/LiquidCats/libraries/jsonrpc/v2"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    request := jsonrpc.NewRequest[string]("exampleMethod", []any{123})
    result, err := request.Execute(ctx, "https://your.rpc",
        jsonrpc.WithHeader[string]("Authorization", "Bearer token"),
    )
    if err != nil {
        log.Fatal(err)
    }
    log.Print(result)
}
```

`NewRequest[Result]` accepts `WithRPCid[Result]` and `WithRPCVersion[Result]`. The default version is `2.0`. `Execute` accepts:

- `WithHeader[Result](key, value)`: appends a header value. Content-Type defaults to `application/json` unless supplied.
- `WithClient[Result](client)`: uses a custom HTTP client, including its timeout and transport settings. Nil leaves the current client unchanged.
- `WithMaxResponseBytes[Result](limit)`: overrides the 64-MiB default limit on the decompressed response, including trailing whitespace. Limits must be positive and less than `math.MaxInt64`.

The default client has a 30-second overall HTTP timeout, a 10-second response-header timeout, and 5-second dial/TLS timeouts. A custom client replaces these defaults; configure its timeout or pass a context deadline. The HTTP timeout covers network/body reading, not arbitrary custom JSON unmarshaling code. For unusually large or slow responses, supply an appropriately configured client and response-size limit.

Non-2xx final HTTP statuses return errors without decoding the body. For successful HTTP statuses, a JSON-RPC error is returned as `*jsonrpc.RPCError`; use `errors.As` to inspect its code and message. Transport/read errors preserve their causes for `errors.Is`. Failed execution returns the zero result. Responses must contain exactly one JSON value; malformed data and trailing JSON are rejected. Response version/ID matching and full envelope validation are not currently enforced.

Request bytes belong to each execution and remain valid for asynchronous transport reads and redirect replays. Responses within the size limit are read to EOF before decoding, allowing HTTP/1 connection reuse even after JSON decode errors. Oversized responses and HTTP error bodies are closed immediately; those paths can sacrifice connection reuse to avoid further untrusted reads. Response limits bound input bytes, not decoded Go object sizes or aggregate memory across concurrent calls.

The default transport keeps at most 100 idle connections total, 16 per host, uses standard transport I/O buffer sizes, and enables HTTP/2 and automatic gzip handling. Compression trades CPU for bandwidth. Decoded strings are copied so small results do not retain an entire large response. Caller-controlled custom unmarshaling can have its own allocation/retention behavior. Request fields and parameters must not be mutated during execution.

Tests use the external `jsonrpc_test` package; benchmarks live in `request_bench_test.go`.

```sh
go test -race -vet=all ./...
go test -run '^$' -bench . -benchmem
```

See [REVIEW.md](REVIEW.md) for findings and measured performance tradeoffs.
