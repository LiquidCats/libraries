# utxo-lib

Bitcoin network parameters: magic bytes and address prefixes.

```sh
go get github.com/LiquidCats/libraries/utxo-lib
```

```go
params := netparams.BitcoinMainNet
params.IsBech32SegwitPrefix("bc1") // true
```

Available networks: `BitcoinMainNet`, `BitcoinTestNet3`, `BitcoinTestNet4`,
`BitcoinRegNet`. Each `NetParams` carries `Net` (magic bytes),
`PubKeyHashAddrID`, `ScriptHashAddrID` and `Bech32HRPSegwit`.
