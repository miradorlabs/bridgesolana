package bridgesolana

// LegType identifies whether a detected bridge event is the source leg
// or the destination leg of a cross-chain transfer.
type LegType string

const (
	LegTypeSource      LegType = "source"
	LegTypeDestination LegType = "destination"
)

// Detection is a single bridge event extracted from a Solana
// transaction's program logs.
//
// CorrelationID is populated when the detector could extract it directly
// from the logs (Anchor "Program data:" events). Otherwise Resolution is
// non-nil and the caller must fetch the relevant on-chain data via RPC
// and pass it to [Resolution.Resolve] to obtain the correlation ID.
type Detection struct {
	BridgeName        string
	BridgeDescription string
	BridgeLegType     LegType
	CorrelationID     string
	Resolution        *Resolution
}

// Resolution describes how to obtain the correlation ID for an
// instruction-mode detection that did not carry it in-band. The caller
// uses MessageProgramID (and AccountDiscriminator on source legs) to
// locate the on-chain bytes, then hands them to [Resolution.Resolve].
type Resolution struct {
	// MessageProgramID is the program that holds the on-chain data
	// containing the correlation ID — a MessageSent account for source
	// legs, the ReceiveMessage instruction data for destination legs.
	MessageProgramID string

	// AccountDiscriminator is the 8-byte Anchor account discriminator
	// used to identify the relevant account among the transaction's
	// accounts. Set for source legs only; zero for destination legs,
	// which dispatches Resolve to the instruction-data parser.
	AccountDiscriminator [8]byte

	accountHeaderSize        int
	instructionHeaderSize    int
	instructionDiscriminator [8]byte
	correlation              []correlationField
}

// correlationField describes a single field to extract from decoded
// event or message data and contribute to the correlation ID string.
type correlationField struct {
	// Offset is the byte offset into the decoded data where the field
	// begins.
	Offset int `json:"offset"`
	// Size is the field's length in bytes. Required for fixed-size
	// types (uintN, bytes32, pubkey); 0 is valid for "keccak256",
	// which hashes from Offset to the end of the data.
	Size int `json:"size"`
	// Type names the extraction strategy: "uint64_le", "uint32_le",
	// "uint64", "uint32", "bytes32", "pubkey", or "keccak256". Unknown
	// values fall back to a generic big-endian integer of Size bytes.
	Type string `json:"type"`
	// Field is a human-readable label used only in error messages
	// (e.g. "nonce"). It is not part of the resulting correlation ID.
	Field string `json:"field"`
}
