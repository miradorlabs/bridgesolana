package bridgesolana

import (
	"encoding/hex"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// Resolver resolves Solana bridge subscriptions.
type Resolver struct {
	logger *zap.Logger
}

// NewResolver creates a new Solana bridge resolver.
func NewResolver(logger *zap.Logger) *Resolver {
	return &Resolver{
		logger: logger.Named("solana-bridges"),
	}
}

// Subscriptions builds bridge subscriptions from embedded JSON configuration.
func (r *Resolver) Subscriptions() ([]*BridgeSubscription, error) {
	cfgs, err := allConfigs()
	if err != nil {
		return nil, err
	}

	subs := make([]*BridgeSubscription, 0, len(cfgs))
	for _, cfg := range cfgs {
		sub, err := newSubscription(cfg)
		if err != nil {
			return nil, err
		}

		r.logger.Info("loaded Solana bridge",
			zap.String("bridge_name", cfg.BridgeName),
			zap.String("description", cfg.BridgeDescription),
			zap.String("leg_type", cfg.BridgeEvent.Type),
			zap.String("program_id", cfg.BridgeProgram.ProgramID),
			zap.String("event_name", cfg.BridgeEvent.Name),
			zap.String("detection_mode", sub.DetectionMode),
		)
		subs = append(subs, sub)
	}

	return subs, nil
}

// ProgramIDs returns the unique set of program IDs to subscribe to.
func (r *Resolver) ProgramIDs() ([]string, error) {
	cfgs, err := allConfigs()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	programIDs := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		pid := cfg.BridgeProgram.ProgramID
		if _, ok := seen[pid]; !ok {
			seen[pid] = struct{}{}
			programIDs = append(programIDs, pid)
		}
		// For instruction-mode, also subscribe to the message program.
		if cfg.BridgeEvent.DetectionMode == detectionModeInstruction && cfg.BridgeEvent.MessageProgramID != "" {
			mpid := cfg.BridgeEvent.MessageProgramID
			if _, ok := seen[mpid]; !ok {
				seen[mpid] = struct{}{}
				programIDs = append(programIDs, mpid)
			}
		}
	}

	return programIDs, nil
}

func newSubscription(cfg *bridgeConfig) (*BridgeSubscription, error) {
	if cfg == nil {
		return nil, fmt.Errorf("bridge config is nil")
	}

	var legType LegType
	switch strings.ToLower(cfg.BridgeEvent.Type) {
	case "source":
		legType = LegTypeSource
	case "destination":
		legType = LegTypeDestination
	default:
		return nil, fmt.Errorf("bridge %s has unsupported event type %q", cfg.BridgeName, cfg.BridgeEvent.Type)
	}

	corFields := make([]correlationField, len(cfg.BridgeEvent.Correlation))
	copy(corFields, cfg.BridgeEvent.Correlation)

	sub := &BridgeSubscription{
		ProgramID:         cfg.BridgeProgram.ProgramID,
		BridgeName:        cfg.BridgeName,
		BridgeDescription: cfg.BridgeDescription,
		BridgeLegType:     legType,
		EventName:         cfg.BridgeEvent.Name,
		Correlation:       corFields,
		DetectionMode:     cfg.BridgeEvent.DetectionMode,
		MessageVersion:    cfg.BridgeEvent.MessageVersion,
		MessageProgramID:  cfg.BridgeEvent.MessageProgramID,
	}

	// Log-mode: compute the Anchor discriminator.
	if cfg.BridgeEvent.DetectionMode == detectionModeLog {
		sub.Discriminator = computeAnchorDiscriminator(cfg.BridgeEvent.AnchorType, cfg.BridgeEvent.Name)
	}

	// Instruction-mode: store the instruction name, account discriminator, and header sizes.
	if cfg.BridgeEvent.DetectionMode == detectionModeInstruction {
		sub.InstructionName = cfg.BridgeEvent.Name
		sub.DataHeaderSize = cfg.BridgeEvent.DataHeaderSize
		sub.AccountDataHeaderSize = cfg.BridgeEvent.AccountDataHeaderSize

		if cfg.BridgeEvent.AccountDiscriminator != "" {
			discBytes, err := hex.DecodeString(cfg.BridgeEvent.AccountDiscriminator)
			if err != nil {
				return nil, fmt.Errorf("bridge %s: invalid accountDiscriminator hex %q: %w", cfg.BridgeName, cfg.BridgeEvent.AccountDiscriminator, err)
			}
			if len(discBytes) != 8 {
				return nil, fmt.Errorf("bridge %s: accountDiscriminator must be 8 bytes, got %d", cfg.BridgeName, len(discBytes))
			}
			copy(sub.AccountDiscriminator[:], discBytes)
			sub.HasAccountDisc = true
		}
	}

	return sub, nil
}
