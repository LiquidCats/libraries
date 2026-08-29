package codec

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/LiquidCats/libraries/evm-lib/parser/types"
)

// GnosisMultiSendDecoder decodes Gnosis MultiSend / MultiSendCallOnly:
//
//	multiSend(bytes transactions)   0x8d80ff0a
//
// The `transactions` blob is a tightly packed sequence of entries:
//
//	operation : uint8  (1 byte)
//	to        : address (20 bytes)
//	value     : uint256 (32 bytes)
//	dataLen   : uint256 (32 bytes)
//	data      : bytes   (dataLen bytes)
//
// Every entry whose value > 0 emits a deterministic ETH transfer from the
// MultiSend contract to `to`.
type GnosisMultiSendDecoder struct{}

func NewGnosisMultiSendDecoder() *GnosisMultiSendDecoder { return &GnosisMultiSendDecoder{} }

var selMultiSend = types.Selector{0x8d, 0x80, 0xff, 0x0a}

func (d *GnosisMultiSendDecoder) CanDecode(s types.Selector) bool {
	return s == selMultiSend
}

func (d *GnosisMultiSendDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	if sel != selMultiSend {
		return nil, fmt.Errorf("gnosis_multisend: unsupported selector %s", sel)
	}

	blob, err := ReadDynamicBytes(params, 0)
	if err != nil {
		return nil, fmt.Errorf("gnosis_multisend: read transactions: %w", err)
	}

	out := &types.ParsedInputData{Selector: sel}
	for i := 0; i < len(blob); {
		const entryHeader = 1 + 20 + 32 + 32 // operation + to + value + dataLen
		if i+entryHeader > len(blob) {
			return nil, fmt.Errorf("gnosis_multisend: truncated entry at byte %d", i)
		}
		// operation := blob[i]   // 0=call, 1=delegatecall; we don't filter here
		var addr types.Address
		copy(addr[:], blob[i+1:i+21])
		value := new(big.Int).SetBytes(blob[i+21 : i+53])

		dataLenWord := blob[i+53 : i+85]
		// Reject unreasonable dataLen up front.
		if dataLenWord[0] != 0 || dataLenWord[1] != 0 {
			return nil, fmt.Errorf("gnosis_multisend: unreasonable dataLen at byte %d", i)
		}
		// Compare in uint64 before narrowing: a dataLen above math.MaxInt would
		// wrap on conversion and turn the bounds check into a no-op.
		dataLen := binary.BigEndian.Uint64(dataLenWord[24:32])
		//nolint:gosec // len(blob) >= i+entryHeader per the check above, so this cannot wrap
		remaining := uint64(len(blob) - i - entryHeader)
		if dataLen > remaining {
			return nil, fmt.Errorf("gnosis_multisend: data overflow at byte %d", i)
		}
		//nolint:gosec // dataLen <= remaining <= len(blob), so it fits in int
		dataSize := int(dataLen)

		if value.Sign() > 0 {
			out.Transfers = append(out.Transfers, types.Transfer{
				To:         addr,
				Value:      value,
				Confidence: types.Deterministic,
			})
		}
		i += entryHeader + dataSize
	}

	return out, nil
}
