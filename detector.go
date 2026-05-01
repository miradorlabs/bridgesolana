package bridgesolana

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// BridgeDetector scans Solana program logs and emits Detection records.
// It is read-only after construction and safe to share across goroutines.
type BridgeDetector struct {
	logSubs   map[[8]byte]*logSubscription
	instrSubs map[instructionKey]*instrSubscription
	progIDs   []string // sorted, deduped; for ProgramIDs().
}

// NewBridgeDetector builds a detector from the embedded bridge configs.
func NewBridgeDetector() (*BridgeDetector, error) {
	cfgs, err := allConfigs()
	if err != nil {
		return nil, fmt.Errorf("failed to load Solana bridge configs: %w", err)
	}

	d := &BridgeDetector{
		logSubs:   make(map[[8]byte]*logSubscription),
		instrSubs: make(map[instructionKey]*instrSubscription),
	}

	progSet := make(map[string]struct{})
	for _, cfg := range cfgs {
		if err := d.addSubscription(cfg, progSet); err != nil {
			return nil, err
		}
	}

	d.progIDs = slices.Sorted(maps.Keys(progSet))

	return d, nil
}

// ProgramIDs returns the unique set of Solana program IDs the detector
// matches against, sorted for stable output. Pass these to
// LogsSubscribeMentions / SubscribeProgramLogs to receive every
// transaction the detector can recognize.
func (d *BridgeDetector) ProgramIDs() []string {
	out := make([]string, len(d.progIDs))
	copy(out, d.progIDs)
	return out
}

// Detect scans logs (the raw "Program ..." strings from a single Solana
// transaction's metadata) and returns one [Detection] per matched bridge
// event. The detector tracks the program invocation stack across log
// lines, so both "Program data:" events and "Program log: Instruction:"
// lines are scoped to the program currently executing.
//
// A [Detection] with a non-empty CorrelationID is fully resolved. A
// [Detection] with a non-nil [Resolution] requires the caller to fetch
// the relevant account or instruction data via RPC and call the package
// helpers to extract the correlation ID.
func (d *BridgeDetector) Detect(logs []string) []Detection {
	if len(d.logSubs) == 0 && len(d.instrSubs) == 0 {
		return nil
	}

	var (
		out      []Detection
		stack    []string
		hasLog   = len(d.logSubs) > 0
		hasInstr = len(d.instrSubs) > 0
	)

	const (
		invokePrefix    = "Program "
		invokeMarker    = " invoke ["
		dataPrefix      = "Program data: "
		instrLogPrefix  = "Program log: Instruction: "
		successSuffix   = " success"
		failedSubstring = " failed"
	)

	for _, line := range logs {
		trimmed := strings.TrimSpace(line)

		// invoke / exit / inside-program are mutually exclusive — keep
		// these as `else if` so the ordering dependency is explicit.
		if strings.HasPrefix(trimmed, invokePrefix) && strings.Contains(trimmed, invokeMarker) {
			stack = append(stack, extractProgramID(trimmed))
		} else if strings.HasPrefix(trimmed, invokePrefix) && (strings.HasSuffix(trimmed, successSuffix) || strings.Contains(trimmed, failedSubstring)) {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		} else if len(stack) > 0 {
			current := stack[len(stack)-1]

			if hasInstr && strings.HasPrefix(trimmed, instrLogPrefix) {
				name := strings.TrimPrefix(trimmed, instrLogPrefix)
				if sub, ok := d.instrSubs[instructionKey{programID: current, instructionName: name}]; ok {
					res := sub.resolution
					// Defensive copy: callers must not mutate detector
					// state through Resolution's correlation slice.
					res.correlation = cloneCorrelation(res.correlation)
					out = append(out, Detection{
						BridgeName:        sub.bridgeName,
						BridgeDescription: sub.bridgeDesc,
						BridgeLegType:     sub.legType,
						Resolution:        &res,
					})
				}
			} else if hasLog && strings.HasPrefix(trimmed, dataPrefix) {
				if det, ok := d.matchLogData(current, strings.TrimPrefix(trimmed, dataPrefix)); ok {
					out = append(out, det)
				}
			}
		}
	}

	return out
}

func (d *BridgeDetector) addSubscription(cfg *bridgeConfig, progSet map[string]struct{}) error {
	legType, err := parseLegType(cfg.BridgeEvent.Type)
	if err != nil {
		return fmt.Errorf("bridge %s: %w", cfg.BridgeName, err)
	}

	progSet[cfg.BridgeProgram.ProgramID] = struct{}{}
	if cfg.BridgeEvent.MessageProgramID != "" {
		progSet[cfg.BridgeEvent.MessageProgramID] = struct{}{}
	}

	switch cfg.BridgeEvent.DetectionMode {
	case detectionModeInstruction:
		// The instruction log is emitted by the program that *runs* the
		// instruction, which is the messageProgramID for CPI'd legs.
		emittingProg := cfg.BridgeEvent.MessageProgramID
		if emittingProg == "" {
			emittingProg = cfg.BridgeProgram.ProgramID
		}

		var accountDisc [8]byte
		if cfg.BridgeEvent.AccountDiscriminator != "" {
			b, err := hex.DecodeString(cfg.BridgeEvent.AccountDiscriminator)
			if err != nil {
				return fmt.Errorf("bridge %s: invalid accountDiscriminator hex %q: %w", cfg.BridgeName, cfg.BridgeEvent.AccountDiscriminator, err)
			}
			if len(b) != 8 {
				return fmt.Errorf("bridge %s: accountDiscriminator must be 8 bytes, got %d", cfg.BridgeName, len(b))
			}
			copy(accountDisc[:], b)
		}

		key := instructionKey{programID: emittingProg, instructionName: cfg.BridgeEvent.Name}
		d.instrSubs[key] = &instrSubscription{
			bridgeName: cfg.BridgeName,
			bridgeDesc: cfg.BridgeDescription,
			legType:    legType,
			resolution: Resolution{
				MessageProgramID:     cfg.BridgeEvent.MessageProgramID,
				AccountDiscriminator: accountDisc,
				messageVersion:       cfg.BridgeEvent.MessageVersion,
				correlation:          cloneCorrelation(cfg.BridgeEvent.Correlation),
			},
		}

	default: // log mode.
		disc := computeAnchorDiscriminator(cfg.BridgeEvent.AnchorType, cfg.BridgeEvent.Name)
		d.logSubs[disc] = &logSubscription{
			programID:   cfg.BridgeProgram.ProgramID,
			bridgeName:  cfg.BridgeName,
			bridgeDesc:  cfg.BridgeDescription,
			legType:     legType,
			correlation: cloneCorrelation(cfg.BridgeEvent.Correlation),
		}
	}

	return nil
}

func (d *BridgeDetector) matchLogData(currentProgram, b64 string) (Detection, bool) {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(decoded) < 8 {
		return Detection{}, false
	}
	var disc [8]byte
	copy(disc[:], decoded[:8])
	sub, ok := d.logSubs[disc]
	if !ok || sub.programID != currentProgram {
		return Detection{}, false
	}
	correlationID, err := extractCorrelationFields(decoded, sub.correlation)
	if err != nil {
		return Detection{}, false
	}
	return Detection{
		BridgeName:        sub.bridgeName,
		BridgeDescription: sub.bridgeDesc,
		BridgeLegType:     sub.legType,
		CorrelationID:     correlationID,
	}, true
}

type logSubscription struct {
	programID   string
	bridgeName  string
	bridgeDesc  string
	legType     LegType
	correlation []correlationField
}

type instrSubscription struct {
	bridgeName string
	bridgeDesc string
	legType    LegType
	resolution Resolution
}

type instructionKey struct {
	programID       string
	instructionName string
}

func parseLegType(s string) (LegType, error) {
	switch strings.ToLower(s) {
	case "source":
		return LegTypeSource, nil
	case "destination":
		return LegTypeDestination, nil
	default:
		return "", fmt.Errorf("unsupported event type %q", s)
	}
}

func cloneCorrelation(in []correlationField) []correlationField {
	out := make([]correlationField, len(in))
	copy(out, in)
	return out
}

// extractProgramID parses the program ID from a log line like
// "Program <ID> invoke [1]" or "Program <ID> success".
func extractProgramID(line string) string {
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}
