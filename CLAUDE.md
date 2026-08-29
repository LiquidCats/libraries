# CLAUDE.md

Guidance for Claude Code working in this repo. See [README.md](README.md) for
what each library does and how it is used.

## Layout

Five independent Go modules, one per directory — `evm-lib`, `graceful`,
`observer`, `utxo-lib`, `workers`. There is no root module and no `go.work`
(it is gitignored on purpose).

Consequences:

- Run `go` and `golangci-lint` from inside a module directory. At the repo root
  they find nothing.
- Modules do not import each other. Keep it that way unless asked — a
  cross-module import means the consumer needs a `require` plus a released tag
  of the dependency, which is why they are separate modules.
- Each module pins its own Go version (currently 1.25.0 through 1.27.0). Check
  the target module's `go.mod` before reaching for a recent language or stdlib
  feature; what compiles in `evm-lib` may not compile in `observer`.
- Releases are tagged `<module>/vX.Y.Z`. A bare `vX.Y.Z` tag does not publish
  anything.

## Checks

Per module:

```sh
cd <module>
go test -race -vet=all ./...
golangci-lint run ./...
```

Both must be clean — they are the two CI steps. `graceful` and `workers` tests
exercise real timing and take ~7s and ~15s; that is normal, not a hang.
`evm-lib` and `utxo-lib` currently have no tests, so `go test` only builds and
vets them.

CI is `.github/workflows/ci.yml`: one matrix job per module, Go version read
from that module's `go.mod`. **Adding a module means adding it to the
`matrix.module` list** — nothing discovers directories automatically.

## Conventions

- Options are variadic functional options (`WithPort`, `WithMaxWorkerCount[T]`),
  applied over a defaults struct in the constructor. Follow that shape when
  adding configuration rather than growing the argument list or exporting a
  config struct.
- Constructors validate and return sentinel errors (see `workers/errors.go`).
  Do not panic on bad configuration.
- `graceful` wraps errors with `rotisserie/eris` and logs via `rs/zerolog`;
  `workers` and `observer` use stdlib `errors` and no logger. Match the module
  you are editing instead of unifying them.
- Concurrency is `golang.org/x/sync/errgroup` throughout: the first error
  cancels the context and unwinds everything.
- `graceful`, `workers` and `observer` have no dependency on any chain code.
  Keep chain-specific logic in `evm-lib` / `utxo-lib`.

## evm-lib specifics

`parser.Parse` takes bare hex calldata with **no `0x` prefix** and no
caller/callee context, so `Transfer.From` is legitimately zero for many codecs —
that is not a bug to fix locally. Decoder order matters: `HeuristicDecoder`
guesses from calldata shape and must stay last. Use the readers in
`codecs/abi.go` for ABI word access rather than slicing bytes by hand.
