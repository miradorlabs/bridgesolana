package bridgesolana

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"
)

// Detection modes used in bridge configs.
const (
	detectionModeLog         = "log"
	detectionModeInstruction = "instruction"
)

//go:embed config/*.json
var bridgeConfigFS embed.FS

var (
	loadOnce sync.Once
	loadErr  error
	configs  []*bridgeConfig
)

// bridgeConfig is the JSON-serialized shape of a Solana bridge definition.
type bridgeConfig struct {
	BridgeName        string        `json:"bridgeName"`
	BridgeDescription string        `json:"bridgeDescription"`
	BridgeProgram     bridgeProgram `json:"bridgeProgram"`
	BridgeEvent       bridgeEvent   `json:"bridgeEvent"`
}

// bridgeProgram describes the Solana program to subscribe to.
type bridgeProgram struct {
	ProgramID string `json:"programId"`
}

// bridgeEvent defines the event or instruction to detect.
type bridgeEvent struct {
	Type        string             `json:"type"`        // "source" or "destination".
	Name        string             `json:"name"`        // Event name (e.g. "MessageReceived") or instruction name (e.g. "SendMessage").
	AnchorType  string             `json:"anchorType"`  // "event" or "account" (for discriminator computation); optional in instruction mode.
	Correlation []CorrelationField `json:"correlation"` // Fields to extract for the correlation ID.

	// Instruction-mode fields.
	DetectionMode         string `json:"detectionMode,omitempty"`         // "log" (default) or "instruction".
	MessageVersion        int    `json:"messageVersion,omitempty"`        // 0 = v1 (uint64 nonce), 1 = v2 (bytes32 nonce).
	AccountDiscriminator  string `json:"accountDiscriminator,omitempty"`  // Hex string of 8-byte discriminator for finding the source account.
	MessageProgramID      string `json:"messageProgramId,omitempty"`      // Program holding the MessageSent account or instruction data.
	DataHeaderSize        int    `json:"dataHeaderSize,omitempty"`        // Destination: bytes before Vec in instruction data.
	AccountDataHeaderSize int    `json:"accountDataHeaderSize,omitempty"` // Source: bytes before Vec in account data.
}

// allConfigs returns all embedded Solana bridge definitions.
func allConfigs() ([]*bridgeConfig, error) {
	if err := ensureLoaded(); err != nil {
		return nil, err
	}
	out := make([]*bridgeConfig, len(configs))
	copy(out, configs)
	return out, nil
}

// computeAnchorDiscriminator computes the 8-byte Anchor discriminator for a given type and name.
// For events: SHA256("event:<name>")[:8]. For accounts: SHA256("account:<name>")[:8].
func computeAnchorDiscriminator(anchorType, name string) [8]byte {
	prefix := anchorType + ":" + name
	hash := sha256.Sum256([]byte(prefix))
	var disc [8]byte
	copy(disc[:], hash[:8])
	return disc
}

func ensureLoaded() error {
	loadOnce.Do(func() {
		configs, loadErr = loadBridgeConfigs()
	})
	return loadErr
}

func loadBridgeConfigs() ([]*bridgeConfig, error) {
	entries, err := bridgeConfigFS.ReadDir("config")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded Solana bridge config directory: %w", err)
	}

	var out []*bridgeConfig

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := path.Join("config", entry.Name())
		raw, err := bridgeConfigFS.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		var fileConfigs []*bridgeConfig
		if err := json.Unmarshal(raw, &fileConfigs); err != nil {
			return nil, fmt.Errorf("failed to decode %s: %w", filePath, err)
		}

		for idx, cfg := range fileConfigs {
			if err := validateBridgeConfig(filePath, idx, cfg); err != nil {
				return nil, err
			}
			out = append(out, cfg)
		}
	}

	return out, nil
}

func validateBridgeConfig(filename string, idx int, cfg *bridgeConfig) error {
	if cfg == nil {
		return fmt.Errorf("nil bridge config in %s index %d", filename, idx)
	}
	if err := validateRequiredStrings(filename, idx, cfg); err != nil {
		return err
	}
	if err := validateDetectionMode(filename, idx, cfg); err != nil {
		return err
	}
	return validateCorrelationFields(filename, idx, cfg.BridgeEvent.Correlation)
}

func validateRequiredStrings(filename string, idx int, cfg *bridgeConfig) error {
	required := []struct {
		field string
		value *string
	}{
		{"bridgeName", &cfg.BridgeName},
		{"bridgeDescription", &cfg.BridgeDescription},
	}
	for _, r := range required {
		*r.value = strings.TrimSpace(*r.value)
		if *r.value == "" {
			return fmt.Errorf("bridge config %s[%d] missing %s", filename, idx, r.field)
		}
	}
	if strings.TrimSpace(cfg.BridgeProgram.ProgramID) == "" {
		return fmt.Errorf("bridge config %s[%d] missing bridgeProgram.programId", filename, idx)
	}
	if cfg.BridgeEvent.Type == "" {
		return fmt.Errorf("bridge config %s[%d] missing bridgeEvent.type", filename, idx)
	}
	if cfg.BridgeEvent.Name == "" {
		return fmt.Errorf("bridge config %s[%d] missing bridgeEvent.name", filename, idx)
	}
	return nil
}

func validateDetectionMode(filename string, idx int, cfg *bridgeConfig) error {
	if cfg.BridgeEvent.DetectionMode == "" {
		cfg.BridgeEvent.DetectionMode = detectionModeLog
	}
	switch cfg.BridgeEvent.DetectionMode {
	case detectionModeLog:
		if cfg.BridgeEvent.AnchorType == "" {
			return fmt.Errorf("bridge config %s[%d] missing bridgeEvent.anchorType for log mode", filename, idx)
		}
		return nil
	case detectionModeInstruction:
		return validateInstructionMode(filename, idx, &cfg.BridgeEvent)
	default:
		return fmt.Errorf("bridge config %s[%d] invalid bridgeEvent.detectionMode %q (must be %q or %q)",
			filename, idx, cfg.BridgeEvent.DetectionMode, detectionModeLog, detectionModeInstruction)
	}
}

func validateInstructionMode(filename string, idx int, ev *bridgeEvent) error {
	if ev.MessageProgramID == "" {
		return fmt.Errorf("bridge config %s[%d] missing bridgeEvent.messageProgramId for instruction mode", filename, idx)
	}
	// Default destination dataHeaderSize: 8 bytes (anchor discriminator).
	if ev.Type == "destination" && ev.DataHeaderSize == 0 {
		ev.DataHeaderSize = 8
	}
	// Default source accountDataHeaderSize based on messageVersion.
	if ev.Type == "source" && ev.AccountDataHeaderSize == 0 {
		switch ev.MessageVersion {
		case 0:
			ev.AccountDataHeaderSize = 40 // disc(8) + rent_payer(32).
		case 1:
			ev.AccountDataHeaderSize = 48 // disc(8) + rent_payer(32) + created_at(8).
		}
	}
	return nil
}

func validateCorrelationFields(filename string, idx int, fields []CorrelationField) error {
	if len(fields) == 0 {
		return fmt.Errorf("bridge config %s[%d] missing correlation fields", filename, idx)
	}
	for i, field := range fields {
		if field.Offset < 0 {
			return fmt.Errorf("bridge config %s[%d] correlation field %d offset must be >= 0", filename, idx, i)
		}
		typ := strings.ToLower(strings.TrimSpace(field.Type))
		if typ == "" {
			return fmt.Errorf("bridge config %s[%d] correlation field %d missing type", filename, idx, i)
		}
		// keccak256 hashes from offset to end; size == 0 is valid.
		if typ != "keccak256" && field.Size <= 0 {
			return fmt.Errorf("bridge config %s[%d] correlation field %d size must be > 0", filename, idx, i)
		}
	}
	return nil
}
