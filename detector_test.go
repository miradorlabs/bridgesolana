package bridgesolana

import (
	"reflect"
	"sort"
	"testing"
)

func TestDetect_CCTPv1Source(t *testing.T) {
	d := newDetector(t)

	logs := []string{
		"Program CCTPiPYPc6AsJuwueEnWgSgucamXDZwBd53dQ11YiKX3 invoke [1]",
		"Program log: Instruction: DepositForBurn",
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd invoke [2]",
		"Program log: Instruction: SendMessage",
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd success",
		"Program CCTPiPYPc6AsJuwueEnWgSgucamXDZwBd53dQ11YiKX3 success",
	}

	got := d.Detect(logs)
	if len(got) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(got))
	}
	det := got[0]
	if det.BridgeName != "cctp" {
		t.Fatalf("BridgeName = %q, want %q", det.BridgeName, "cctp")
	}
	if det.BridgeLegType != LegTypeSource {
		t.Fatalf("BridgeLegType = %q, want %q", det.BridgeLegType, LegTypeSource)
	}
	if det.CorrelationID != "" {
		t.Fatalf("CorrelationID should be empty for instruction mode, got %q", det.CorrelationID)
	}
	if det.Resolution == nil {
		t.Fatal("Resolution must be non-nil for instruction-mode detection")
	}
	if det.Resolution.MessageProgramID != "CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd" {
		t.Fatalf("Resolution.MessageProgramID = %q", det.Resolution.MessageProgramID)
	}
	if det.Resolution.AccountDiscriminator == ([8]byte{}) {
		t.Fatal("Resolution.AccountDiscriminator must be set for source legs")
	}
	if det.Resolution.MessageVersion != 0 {
		t.Fatalf("Resolution.MessageVersion = %d, want 0 (V1)", det.Resolution.MessageVersion)
	}
}

func TestDetect_CCTPv1Destination(t *testing.T) {
	d := newDetector(t)

	logs := []string{
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd invoke [1]",
		"Program log: Instruction: ReceiveMessage",
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd success",
	}

	got := d.Detect(logs)
	if len(got) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(got))
	}
	det := got[0]
	if det.BridgeLegType != LegTypeDestination {
		t.Fatalf("BridgeLegType = %q, want %q", det.BridgeLegType, LegTypeDestination)
	}
	if det.Resolution == nil {
		t.Fatal("Resolution must be non-nil")
	}
	// Destination legs leave AccountDiscriminator as zero.
	if det.Resolution.AccountDiscriminator != ([8]byte{}) {
		t.Fatalf("AccountDiscriminator should be zero for destination legs, got %x", det.Resolution.AccountDiscriminator)
	}
}

func TestDetect_NoMatch(t *testing.T) {
	d := newDetector(t)
	logs := []string{
		"Program SomeOtherProgram111111111111111111111111 invoke [1]",
		"Program log: Instruction: Transfer",
		"Program SomeOtherProgram111111111111111111111111 success",
	}
	if got := d.Detect(logs); len(got) != 0 {
		t.Fatalf("expected 0 detections, got %d", len(got))
	}
}

func TestDetect_ProgramScopedCorrectly(t *testing.T) {
	d := newDetector(t)
	// SendMessage outside of any CCTP program context must not trigger.
	logs := []string{
		"Program SomeOtherProgram111111111111111111111111 invoke [1]",
		"Program log: Instruction: SendMessage",
		"Program SomeOtherProgram111111111111111111111111 success",
	}
	if got := d.Detect(logs); len(got) != 0 {
		t.Fatalf("expected 0 detections, got %d", len(got))
	}
}

func TestDetect_CCTPv2Source(t *testing.T) {
	d := newDetector(t)
	logs := []string{
		"Program CCTPV2vPZJS2u2BBsUoscuikbYjnpFmbFsvVuJdgUMQe invoke [1]",
		"Program log: Instruction: DepositForBurn",
		"Program CCTPV2Sm4AdWt5296sk4P66VBZ7bEhcARwFaaS9YPbeC invoke [2]",
		"Program log: Instruction: SendMessage",
		"Program CCTPV2Sm4AdWt5296sk4P66VBZ7bEhcARwFaaS9YPbeC success",
		"Program CCTPV2vPZJS2u2BBsUoscuikbYjnpFmbFsvVuJdgUMQe success",
	}
	got := d.Detect(logs)
	if len(got) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(got))
	}
	if got[0].BridgeName != "cctp-v2" {
		t.Fatalf("BridgeName = %q, want cctp-v2", got[0].BridgeName)
	}
	if got[0].Resolution == nil || got[0].Resolution.MessageVersion != 1 {
		t.Fatalf("expected V2 resolution (MessageVersion=1), got %+v", got[0].Resolution)
	}
}

func TestProgramIDs(t *testing.T) {
	d := newDetector(t)
	got := d.ProgramIDs()
	want := []string{
		"CCTPV2Sm4AdWt5296sk4P66VBZ7bEhcARwFaaS9YPbeC",
		"CCTPV2vPZJS2u2BBsUoscuikbYjnpFmbFsvVuJdgUMQe",
		"CCTPiPYPc6AsJuwueEnWgSgucamXDZwBd53dQ11YiKX3",
		"CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProgramIDs mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestProgramIDs_ReturnsCopy(t *testing.T) {
	d := newDetector(t)
	first := d.ProgramIDs()
	first[0] = "tampered"
	second := d.ProgramIDs()
	if second[0] == "tampered" {
		t.Fatal("ProgramIDs must return a fresh slice; mutation leaked into detector state")
	}
}

func newDetector(t *testing.T) *BridgeDetector {
	t.Helper()
	d, err := NewBridgeDetector()
	if err != nil {
		t.Fatalf("NewBridgeDetector: %v", err)
	}
	return d
}
