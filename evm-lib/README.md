# evm-lib

Decodes raw EVM transaction calldata into value transfers, without a node or
ABIs.

```sh
go get github.com/LiquidCats/libraries/evm-lib
```

Each codec claims a set of 4-byte selectors; the parser walks them in order and
uses the first that matches.

```go
p := parser.New(
	codecs.NewWETHDecoder(),
	codecs.NewUniswapV2RouterDecoder(),
	codecs.NewUniswapV3PaymentsDecoder(),
	codecs.NewAaveWETHGatewayDecoder(),
	codecs.NewDisperseDecoder(),
	codecs.NewGnosisSafeDecoder(),
	codecs.NewGnosisMultiSendDecoder(),
	codecs.NewHeuristicDecoder(), // last: guesses from calldata shape
)

out, err := p.Parse(types.RawInputData("a9059cbb...")) // hex, no 0x prefix
```

- Every `Transfer` carries a `Confidence`, since a heuristic match is weaker than
  a known selector.
- `From` is zero when calldata alone cannot identify the source — `Parse` has no
  caller/callee context. That is expected, not a bug.
- Decoder order matters: `HeuristicDecoder` must stay last.
- `NewMulticallDecoder` and `NewMulticall3ValueDecoder` take a `SubParser` so
  batched inner calls are decoded recursively; pass the `*Parser` itself.
- `codecs.NewChain` composes decoders into one. `codecs/abi.go` holds the ABI
  word readers (`ReadAddress`, `ReadUint256`, `ReadDynamicBytes`, …) for writing
  your own codec.
