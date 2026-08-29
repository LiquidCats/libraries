package netparams

import "strings"

// Net represents which bitcoin network a message belongs to.
type Net uint32
type NetName string

type NetParams struct {
	// Net defines the magic bytes used to identify the network.
	Net Net
	// Address encoding magics
	PubKeyHashAddrID byte // First byte of a P2PKH address
	ScriptHashAddrID byte // First byte of a P2SH address
	Bech32HRPSegwit  string
}

// IsBech32SegwitPrefix returns whether the prefix is a known prefix for segwit
// addresses on any default or registered network.  This is used when decoding
// an address string into a specific address type.
func (p *NetParams) IsBech32SegwitPrefix(prefix string) bool {
	prefix = strings.ToLower(prefix)

	return (p.Bech32HRPSegwit + "1") == prefix
}

var BitcoinMainNet = &NetParams{
	Net:              0xd9b4bef9,
	PubKeyHashAddrID: 0x00,
	ScriptHashAddrID: 0x05,
	Bech32HRPSegwit:  "bc",
}
var BitcoinRegNet = &NetParams{
	Net:              0xdab5bffa,
	PubKeyHashAddrID: 0x6f,
	ScriptHashAddrID: 0xc4,
	Bech32HRPSegwit:  "bcrt",
}
var BitcoinTestNet3 = &NetParams{
	Net:              0x0709110b,
	PubKeyHashAddrID: 0x6f,
	ScriptHashAddrID: 0xc4,
	Bech32HRPSegwit:  "tb",
}
var BitcoinTestNet4 = &NetParams{
	Net:              0x283f161c,
	PubKeyHashAddrID: 0x6f,
	ScriptHashAddrID: 0xc4,
	Bech32HRPSegwit:  "tb",
}
