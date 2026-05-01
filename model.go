package bridgesolana

// LegType identifies whether a detected bridge event is the source leg
// or the destination leg of a cross-chain transfer.
type LegType string

const (
	LegTypeSource      LegType = "source"
	LegTypeDestination LegType = "destination"
)

// Detection is a single bridge event extracted from a Solana transaction's
// program logs.
//
// CorrelationID is populated when the detector could extract it directly
// from the logs (Anchor "Program data:" events). Otherwise Resolution is
// non-nil and the caller must fetch the relevant account or instruction
// data via RPC and feed it into the package-level helpers
// (ParseMessageSentAccount / ParseReceiveMessageInstructionData /
// ExtractCorrelationFields) to produce the correlation ID.
type Detection struct {
	BridgeName        string
	BridgeDescription string
	BridgeLegType     LegType
	CorrelationID     string
	Resolution        *Resolution
}

// Resolution describes how to obtain the correlation ID for an
// instruction-mode detection that did not carry it in-band.
type Resolution struct {
	// MessageProgramID is the program that holds the on-chain data
	// containing the correlation ID — a MessageSent account for source
	// legs, the ReceiveMessage instruction data for destination legs.
	MessageProgramID string

	// AccountDiscriminator is the 8-byte Anchor account discriminator
	// used to identify the relevant account among the transaction's
	// accounts. Set for source legs only; the zero value indicates a
	// destination-leg resolution.
	AccountDiscriminator [8]byte

	// MessageVersion selects the account layout for source legs:
	// 0 = CCTP V1, 1 = CCTP V2. Pass to ParseMessageSentAccount.
	MessageVersion int

	// Correlation is the field-extraction spec to feed into
	// ExtractCorrelationFields once the message bytes have been parsed.
	Correlation []CorrelationField
}

// CorrelationField describes a single field to extract from decoded
// event or message data and contribute to the correlation ID string.
type CorrelationField struct {
	Offset int    `json:"offset"`
	Size   int    `json:"size"`
	Type   string `json:"type"`
	Field  string `json:"field"`
}
