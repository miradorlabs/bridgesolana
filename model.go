package bridgesolana

// BridgeDetails captures normalized data extracted from a Solana bridge program log.
type BridgeDetails struct {
	BridgeLegType     LegType
	CorrelationID     string
	BridgeName        string
	BridgeDescription string
}

// InstructionDetection captures a detected instruction-mode bridge event that needs RPC resolution.
type InstructionDetection struct {
	Subscription *BridgeSubscription
	ProgramID    string // The program ID that was executing when the instruction was detected.
}

// BridgeSubscription describes a Solana program + event discriminator to watch for bridge events.
type BridgeSubscription struct {
	ProgramID         string
	Discriminator     [8]byte // Anchor discriminator for log-mode detection.
	BridgeName        string
	BridgeDescription string
	BridgeLegType     LegType
	EventName         string
	Correlation       []correlationField

	// Instruction-mode fields.
	DetectionMode         string  // "log" or "instruction".
	InstructionName       string  // Instruction name to match (e.g. "SendMessage").
	AccountDiscriminator  [8]byte // 8-byte discriminator for finding the right account (source legs).
	HasAccountDisc        bool    // Whether AccountDiscriminator was set from config.
	MessageVersion        int     // 0 = v1, 1 = v2.
	MessageProgramID      string  // Program that holds the MessageSent account or instruction data.
	DataHeaderSize        int     // Destination: bytes to skip in instruction data before Borsh Vec.
	AccountDataHeaderSize int     // Source: bytes to skip in account data before Borsh Vec.
}

// correlationField describes a single field to extract from decoded event data.
// Used for both JSON config decoding and runtime extraction.
type correlationField struct {
	Offset int    `json:"offset"` // Byte offset within decoded event data (after discriminator).
	Size   int    `json:"size"`   // Number of bytes to extract.
	Type   string `json:"type"`   // "uint64_le", "uint32_le", "bytes32", "keccak256", etc.
	Field  string `json:"field"`  // Human-readable name (e.g. "nonce").
}

// instructionKey indexes instruction-mode subscriptions by (programID, instructionName).
type instructionKey struct {
	ProgramID       string
	InstructionName string
}
