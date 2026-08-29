# CLAUDE.md

Guidance for Claude Code working in this repo. See [README.md](README.md) for
what each library does and how it is used.

## Layout

Seven independent Go modules, one per directory — `db`, `evm-lib`, `graceful`,
`jsonrpc`, `observer`, `utxo-lib`, `workers`. There is no root module and no
`go.work` (it is gitignored on purpose).

Consequences:

- Run `go` and `golangci-lint` from inside a module directory. At the repo root
  they find nothing.
- Modules do not import each other. Keep it that way unless asked — a
  cross-module import means the consumer needs a `require` plus a released tag
  of the dependency, which is why they are separate modules.
- Every module is on `go 1.27.0`. They are pinned per module rather than
  centrally, so a bump means editing all seven `go.mod` files (`make tidy`
  afterwards) — and raising any of them past `GO_IMAGE` in the Makefile breaks
  every target until that is bumped too.
- Releases are tagged `<module>/vX.Y.Z`. A bare `vX.Y.Z` tag does not publish
  anything.

## Checks

Use the root `Makefile`. It runs everything in pinned containers
(`golang:1.27-bookworm`, `golangci/golangci-lint:v2.13.2`), loops over every
module, and passes the shared root `.golangci.yaml` explicitly — module dirs
have no config of their own, so a bare `golangci-lint run` inside one would
silently fall back to defaults.

```sh
make                      # test + lint, everything
make test MODULES=graceful
make lint MODULES=graceful
make lint-fix             # autofix; `make fmt` for formatters only
make shell                # interactive container, same sandbox
```

`MODULES` defaults to every directory containing a `go.mod`, so a new module is
picked up automatically here — but not in CI, see below.

Do not fall back to running `go` or `golangci-lint` on the host when a target
fails; the host toolchain is a different version and will disagree with CI.
Things worth knowing before debugging a failure:

- Containers run with `--network=none`. `make deps` (auto-run via the
  `.make/deps.stamp` prerequisite whenever a `go.mod`/`go.sum` changes) and
  `make tidy` are the only targets with network. A `dial tcp ... network is
  unreachable` error means the module cache is cold — `make deps`, do not add
  network to other targets.
- `GOTOOLCHAIN=local`, so a module requiring a Go version newer than `GO_IMAGE`
  fails outright instead of downloading a toolchain. Bump `GO_IMAGE` in the
  Makefile when raising a `go` directive.
- `GOFLAGS` is left at the default `-mod=readonly`; only `make tidy` may rewrite
  manifests.
- Caches live in `~/.cache/liquidcats-libraries`, not the host GOPATH.
  `make clean` removes them.

Both targets must be clean — they are the two CI steps. `graceful` and `workers` tests
exercise real timing and take ~7s and ~15s; that is normal, not a hang.
`db`, `evm-lib` and `utxo-lib` currently have no tests, so `go test` only builds
and vets them.

CI is `.github/workflows/ci.yml`, two jobs: `test` runs each module against
every Go version in `matrix.go` (currently just 1.27), and `lint` runs once per
module using the version from its own `go.mod`. **Adding a module means adding
it to both `matrix.module` lists** — nothing discovers directories
automatically.

`GOTOOLCHAIN: local` is set workflow-wide, so a module cannot silently upgrade
past the matrix version. If an older version is ever added back to `matrix.go`,
every module whose `go` directive is newer than it needs an `exclude` for that
pair, or that job fails outright instead of being skipped.

## Conventions

- Options are variadic functional options (`WithPort`, `WithMaxWorkerCount[T]`),
  applied over a defaults struct in the constructor. Follow that shape when
  adding configuration rather than growing the argument list or exporting a
  config struct.
- Constructors validate and return sentinel errors (see `workers/errors.go`).
  Do not panic on bad configuration.
- `graceful` wraps errors with `rotisserie/eris` and logs via `rs/zerolog`;
  `workers` and `observer` use stdlib `errors` and no logger; `db` uses
  `fmt.Errorf` with `%w`. Match the module you are editing instead of unifying
  them.
- `db` stores the `pgx.Tx` on the context. Type assertions pulling it back out
  must use the comma-ok form and return `ErrNoTransaction` — a bare assertion
  panics on any context that did not come from `Begin`.
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
