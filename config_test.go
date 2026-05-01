package bridgesolana

import (
	"strings"
	"testing"
)

func TestValidate_InstructionSource_RequiresAccountDiscriminator(t *testing.T) {
	cfg := &bridgeConfig{
		BridgeName:        "test",
		BridgeDescription: "test bridge",
		BridgeProgram:     bridgeProgram{ProgramID: "Prog11111111111111111111111111111111111111"},
		BridgeEvent: bridgeEvent{
			Type:                  string(LegTypeSource),
			Name:                  "SendMessage",
			DetectionMode:         detectionModeInstruction,
			MessageProgramID:      "Prog11111111111111111111111111111111111111",
			AccountDataHeaderSize: 40,
			// AccountDiscriminator deliberately omitted.
			Correlation: []correlationField{
				{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
			},
		},
	}

	err := validateBridgeConfig("test.json", 0, cfg)
	if err == nil {
		t.Fatal("expected validation error for instruction-mode source leg without accountDiscriminator")
	}
	if !strings.Contains(err.Error(), "accountDiscriminator") {
		t.Fatalf("error should mention accountDiscriminator, got: %v", err)
	}
}

func TestValidate_InstructionSource_RequiresAccountDataHeaderSize(t *testing.T) {
	cfg := &bridgeConfig{
		BridgeName:        "test",
		BridgeDescription: "test bridge",
		BridgeProgram:     bridgeProgram{ProgramID: "Prog11111111111111111111111111111111111111"},
		BridgeEvent: bridgeEvent{
			Type:                 string(LegTypeSource),
			Name:                 "SendMessage",
			DetectionMode:        detectionModeInstruction,
			MessageProgramID:     "Prog11111111111111111111111111111111111111",
			AccountDiscriminator: "0102030405060708",
			// AccountDataHeaderSize deliberately omitted.
			Correlation: []correlationField{
				{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
			},
		},
	}

	err := validateBridgeConfig("test.json", 0, cfg)
	if err == nil {
		t.Fatal("expected validation error for instruction-mode source leg without accountDataHeaderSize")
	}
	if !strings.Contains(err.Error(), "accountDataHeaderSize") {
		t.Fatalf("error should mention accountDataHeaderSize, got: %v", err)
	}
}

func TestValidate_InstructionSource_RejectsNegativeAccountDataHeaderSize(t *testing.T) {
	cfg := &bridgeConfig{
		BridgeName:        "test",
		BridgeDescription: "test bridge",
		BridgeProgram:     bridgeProgram{ProgramID: "Prog11111111111111111111111111111111111111"},
		BridgeEvent: bridgeEvent{
			Type:                  string(LegTypeSource),
			Name:                  "SendMessage",
			DetectionMode:         detectionModeInstruction,
			MessageProgramID:      "Prog11111111111111111111111111111111111111",
			AccountDiscriminator:  "0102030405060708",
			AccountDataHeaderSize: -1,
			Correlation: []correlationField{
				{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
			},
		},
	}

	err := validateBridgeConfig("test.json", 0, cfg)
	if err == nil {
		t.Fatal("expected validation error for negative accountDataHeaderSize")
	}
	if !strings.Contains(err.Error(), "accountDataHeaderSize") {
		t.Fatalf("error should mention accountDataHeaderSize, got: %v", err)
	}
}

func TestValidate_InstructionDestination_RejectsNegativeDataHeaderSize(t *testing.T) {
	cfg := &bridgeConfig{
		BridgeName:        "test",
		BridgeDescription: "test bridge",
		BridgeProgram:     bridgeProgram{ProgramID: "Prog11111111111111111111111111111111111111"},
		BridgeEvent: bridgeEvent{
			Type:                     string(LegTypeDestination),
			Name:                     "ReceiveMessage",
			DetectionMode:            detectionModeInstruction,
			MessageProgramID:         "Prog11111111111111111111111111111111111111",
			InstructionDiscriminator: "0102030405060708",
			DataHeaderSize:           -5,
			Correlation: []correlationField{
				{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
			},
		},
	}

	err := validateBridgeConfig("test.json", 0, cfg)
	if err == nil {
		t.Fatal("expected validation error for negative dataHeaderSize")
	}
	if !strings.Contains(err.Error(), "dataHeaderSize") {
		t.Fatalf("error should mention dataHeaderSize, got: %v", err)
	}
}

func TestValidate_InstructionDestination_RequiresInstructionDiscriminator(t *testing.T) {
	cfg := &bridgeConfig{
		BridgeName:        "test",
		BridgeDescription: "test bridge",
		BridgeProgram:     bridgeProgram{ProgramID: "Prog11111111111111111111111111111111111111"},
		BridgeEvent: bridgeEvent{
			Type:             string(LegTypeDestination),
			Name:             "ReceiveMessage",
			DetectionMode:    detectionModeInstruction,
			MessageProgramID: "Prog11111111111111111111111111111111111111",
			// InstructionDiscriminator deliberately omitted.
			Correlation: []correlationField{
				{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
			},
		},
	}

	err := validateBridgeConfig("test.json", 0, cfg)
	if err == nil {
		t.Fatal("expected validation error for instruction-mode destination leg without instructionDiscriminator")
	}
	if !strings.Contains(err.Error(), "instructionDiscriminator") {
		t.Fatalf("error should mention instructionDiscriminator, got: %v", err)
	}
}
