package bridgesolana

import (
	"testing"

	"go.uber.org/zap"
)

func TestDetectInstructionBridges_CCTPv1Source(t *testing.T) {
	detector, err := NewBridgeDetector()
	if err != nil {
		t.Fatalf("failed to create detector: %v", err)
	}

	logs := []string{
		"Program CCTPiPYPc6AsJuwueEnWgSgucamXDZwBd53dQ11YiKX3 invoke [1]",
		"Program log: Instruction: DepositForBurn",
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd invoke [2]",
		"Program log: Instruction: SendMessage",
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd success",
		"Program CCTPiPYPc6AsJuwueEnWgSgucamXDZwBd53dQ11YiKX3 success",
	}

	detections := detector.DetectInstructionBridges(logs)
	if len(detections) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(detections))
	}

	d := detections[0]
	if d.Subscription.BridgeLegType != LegTypeSource {
		t.Fatalf("expected source leg type, got %v", d.Subscription.BridgeLegType)
	}
	if d.Subscription.BridgeName != "cctp" {
		t.Fatalf("expected bridge name 'cctp', got %q", d.Subscription.BridgeName)
	}
	// ProgramID is the executing program when the instruction was detected (MessageTransmitter).
	if d.ProgramID != "CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd" {
		t.Fatalf("expected program ID CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd, got %q", d.ProgramID)
	}
}

func TestDetectInstructionBridges_CCTPv1Destination(t *testing.T) {
	detector, err := NewBridgeDetector()
	if err != nil {
		t.Fatalf("failed to create detector: %v", err)
	}

	logs := []string{
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd invoke [1]",
		"Program log: Instruction: ReceiveMessage",
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd success",
	}

	detections := detector.DetectInstructionBridges(logs)
	if len(detections) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(detections))
	}

	d := detections[0]
	if d.Subscription.BridgeLegType != LegTypeDestination {
		t.Fatalf("expected destination leg type, got %v", d.Subscription.BridgeLegType)
	}
}

func TestDetectInstructionBridges_NoMatch(t *testing.T) {
	detector, err := NewBridgeDetector()
	if err != nil {
		t.Fatalf("failed to create detector: %v", err)
	}

	logs := []string{
		"Program SomeOtherProgram111111111111111111111111 invoke [1]",
		"Program log: Instruction: Transfer",
		"Program SomeOtherProgram111111111111111111111111 success",
	}

	detections := detector.DetectInstructionBridges(logs)
	if len(detections) != 0 {
		t.Fatalf("expected 0 detections, got %d", len(detections))
	}
}

func TestDetectInstructionBridges_ProgramScopedCorrectly(t *testing.T) {
	detector, err := NewBridgeDetector()
	if err != nil {
		t.Fatalf("failed to create detector: %v", err)
	}

	// SendMessage outside of the CCTP program context should not trigger.
	logs := []string{
		"Program SomeOtherProgram111111111111111111111111 invoke [1]",
		"Program log: Instruction: SendMessage",
		"Program SomeOtherProgram111111111111111111111111 success",
	}

	detections := detector.DetectInstructionBridges(logs)
	if len(detections) != 0 {
		t.Fatalf("expected 0 detections for wrong program, got %d", len(detections))
	}
}

func TestDetectInstructionBridges_V2Source(t *testing.T) {
	detector, err := NewBridgeDetector()
	if err != nil {
		t.Fatalf("failed to create detector: %v", err)
	}

	logs := []string{
		"Program CCTPV2vPZJS2u2BBsUoscuikbYjnpFmbFsvVuJdgUMQe invoke [1]",
		"Program log: Instruction: DepositForBurn",
		"Program CCTPV2Sm4AdWt5296sk4P66VBZ7bEhcARwFaaS9YPbeC invoke [2]",
		"Program log: Instruction: SendMessage",
		"Program CCTPV2Sm4AdWt5296sk4P66VBZ7bEhcARwFaaS9YPbeC success",
		"Program CCTPV2vPZJS2u2BBsUoscuikbYjnpFmbFsvVuJdgUMQe success",
	}

	detections := detector.DetectInstructionBridges(logs)
	if len(detections) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(detections))
	}

	d := detections[0]
	if d.Subscription.BridgeName != "cctp-v2" {
		t.Fatalf("expected bridge name 'cctp-v2', got %q", d.Subscription.BridgeName)
	}
}

func TestDetectFromLogs_NoLogModeSubscriptions(t *testing.T) {
	// All bundled CCTP configs are instruction-mode; DetectFromLogs should find nothing.
	detector, err := NewBridgeDetector()
	if err != nil {
		t.Fatalf("failed to create detector: %v", err)
	}

	logs := []string{
		"Program CCTPiPYPc6AsJuwueEnWgSgucamXDZwBd53dQ11YiKX3 invoke [1]",
		"Program data: AAAAAAAAAAAAAAAAAAAAAA==",
		"Program CCTPiPYPc6AsJuwueEnWgSgucamXDZwBd53dQ11YiKX3 success",
	}

	details := detector.DetectFromLogs(logs)
	if len(details) != 0 {
		t.Fatalf("expected 0 log-mode detections (all are instruction-mode), got %d", len(details))
	}
}

func TestResolver_ProgramIDs(t *testing.T) {
	r := NewResolver(zap.NewNop())
	ids, err := r.ProgramIDs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 4 bridge configs across 4 distinct (programID, messageProgramID) pairs.
	expected := map[string]bool{
		"CCTPiPYPc6AsJuwueEnWgSgucamXDZwBd53dQ11YiKX3": false,
		"CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd": false,
		"CCTPV2vPZJS2u2BBsUoscuikbYjnpFmbFsvVuJdgUMQe": false,
		"CCTPV2Sm4AdWt5296sk4P66VBZ7bEhcARwFaaS9YPbeC": false,
	}
	for _, id := range ids {
		if _, ok := expected[id]; !ok {
			t.Fatalf("unexpected program ID %q", id)
		}
		expected[id] = true
	}
	for id, seen := range expected {
		if !seen {
			t.Fatalf("missing expected program ID %q", id)
		}
	}
}
