package bridgesolana

import (
	"encoding/base64"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// BridgeDetector detects bridge events from Solana program logs.
// It supports two detection modes:
//   - "log" mode: scans for "Program data:" log lines, decodes base64,
//     matches 8-byte Anchor discriminators.
//   - "instruction" mode: scans for "Program log: Instruction: <Name>" lines
//     within target program invocations.
type BridgeDetector struct {
	// Log-mode: subscriptions keyed by discriminator for O(1) lookup.
	subscriptions map[[8]byte]*BridgeSubscription
	// Instruction-mode: subscriptions keyed by {programID, instructionName}.
	instructionSubscriptions map[instructionKey]*BridgeSubscription
	programIDs               map[string]struct{} // set of all program IDs to match (both modes).
}

// NewBridgeDetector creates a detector loaded with the embedded Solana
// bridge configurations.
func NewBridgeDetector() (*BridgeDetector, error) {
	resolver := NewResolver(zap.NewNop())
	subs, err := resolver.Subscriptions()
	if err != nil {
		return nil, fmt.Errorf("failed to load Solana bridge configs: %w", err)
	}

	subscriptions := make(map[[8]byte]*BridgeSubscription)
	instrSubs := make(map[instructionKey]*BridgeSubscription)
	programIDs := make(map[string]struct{})

	for _, sub := range subs {
		programIDs[sub.ProgramID] = struct{}{}
		if sub.MessageProgramID != "" {
			programIDs[sub.MessageProgramID] = struct{}{}
		}

		switch sub.DetectionMode {
		case detectionModeInstruction:
			// Key by the program that actually emits the instruction log.
			// For source legs (e.g. DepositForBurn → SendMessage), the
			// SendMessage instruction is emitted by the MessageTransmitter
			// (messageProgramID), not the TokenMessengerMinter. For
			// destination legs the emitting program may match the
			// messageProgramID.
			instrProgramID := sub.MessageProgramID
			if instrProgramID == "" {
				instrProgramID = sub.ProgramID
			}
			key := instructionKey{
				ProgramID:       instrProgramID,
				InstructionName: sub.InstructionName,
			}
			instrSubs[key] = sub
		default: // "log" mode.
			subscriptions[sub.Discriminator] = sub
		}
	}

	return &BridgeDetector{
		subscriptions:            subscriptions,
		instructionSubscriptions: instrSubs,
		programIDs:               programIDs,
	}, nil
}

// DetectFromLogs scans program logs for bridge events using log-mode detection.
// Logs are the raw log strings from a Solana transaction (e.g. from
// LogsSubscribeMentions). Returns all detected bridge details.
func (d *BridgeDetector) DetectFromLogs(logs []string) []*BridgeDetails {
	if len(d.subscriptions) == 0 {
		return nil
	}

	var results []*BridgeDetails

	// Track which program is currently executing.
	inTargetProgram := false

	for _, line := range logs {
		trimmed := strings.TrimSpace(line)

		// Top-level program invocation.
		if strings.HasPrefix(trimmed, "Program ") && strings.HasSuffix(trimmed, " invoke [1]") {
			programID := extractProgramID(trimmed)
			_, inTargetProgram = d.programIDs[programID]
			continue
		}

		// Inner invocations (invoke [2], [3], ...).
		if strings.HasPrefix(trimmed, "Program ") && strings.Contains(trimmed, " invoke [") {
			programID := extractProgramID(trimmed)
			if _, ok := d.programIDs[programID]; ok {
				inTargetProgram = true
			}
			continue
		}

		// Reset state on program success/failure.
		if strings.HasPrefix(trimmed, "Program ") && (strings.HasSuffix(trimmed, " success") || strings.Contains(trimmed, " failed")) {
			programID := extractProgramID(trimmed)
			if _, ok := d.programIDs[programID]; ok {
				inTargetProgram = false
			}
			continue
		}

		// Only process "Program data:" lines while inside a target program.
		if !inTargetProgram {
			continue
		}

		if !strings.HasPrefix(trimmed, "Program data: ") {
			continue
		}

		b64Data := strings.TrimPrefix(trimmed, "Program data: ")
		decoded, err := base64.StdEncoding.DecodeString(b64Data)
		if err != nil {
			continue
		}

		if len(decoded) < 8 {
			continue
		}

		var disc [8]byte
		copy(disc[:], decoded[:8])

		sub, found := d.subscriptions[disc]
		if !found {
			continue
		}

		correlationID, err := ExtractCorrelationFields(decoded, sub.Correlation)
		if err != nil {
			continue
		}

		results = append(results, &BridgeDetails{
			BridgeLegType:     sub.BridgeLegType,
			CorrelationID:     correlationID,
			BridgeName:        sub.BridgeName,
			BridgeDescription: sub.BridgeDescription,
		})
	}

	return results
}

// DetectInstructionBridges scans logs for instruction-mode bridge events.
// It looks for "Program log: Instruction: <Name>" lines within target
// program invocations. Returns detections that need RPC resolution to
// extract correlation data.
func (d *BridgeDetector) DetectInstructionBridges(logs []string) []*InstructionDetection {
	if len(d.instructionSubscriptions) == 0 {
		return nil
	}

	var results []*InstructionDetection

	// Track program invocation stack.
	var programStack []string

	for _, line := range logs {
		trimmed := strings.TrimSpace(line)

		// Push on program invoke.
		if strings.HasPrefix(trimmed, "Program ") && strings.Contains(trimmed, " invoke [") {
			programID := extractProgramID(trimmed)
			programStack = append(programStack, programID)
			continue
		}

		// Pop on program exit.
		if strings.HasPrefix(trimmed, "Program ") && (strings.HasSuffix(trimmed, " success") || strings.Contains(trimmed, " failed")) {
			if len(programStack) > 0 {
				programStack = programStack[:len(programStack)-1]
			}
			continue
		}

		if len(programStack) == 0 {
			continue
		}

		const instrPrefix = "Program log: Instruction: "
		if !strings.HasPrefix(trimmed, instrPrefix) {
			continue
		}

		instrName := strings.TrimPrefix(trimmed, instrPrefix)
		currentProgram := programStack[len(programStack)-1]

		key := instructionKey{
			ProgramID:       currentProgram,
			InstructionName: instrName,
		}

		sub, found := d.instructionSubscriptions[key]
		if !found {
			continue
		}

		results = append(results, &InstructionDetection{
			Subscription: sub,
			ProgramID:    currentProgram,
		})
	}

	return results
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
