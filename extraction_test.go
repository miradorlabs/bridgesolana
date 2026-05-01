package bridgesolana

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestExtractVecPayload(t *testing.T) {
	// header(headerSize) + 4-byte LE Vec length + body bytes.
	body := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	const headerSize = 12

	data := make([]byte, headerSize, headerSize+4+len(body))
	vecLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vecLen, uint32(len(body)))
	data = append(data, vecLen...)
	data = append(data, body...)

	got, err := extractVecPayload(data, headerSize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("payload mismatch: got %x want %x", got, body)
	}
}

func TestExtractVecPayload_TooShortForLengthPrefix(t *testing.T) {
	if _, err := extractVecPayload(make([]byte, 5), 8); err == nil {
		t.Fatal("expected error: data shorter than header + length prefix")
	}
}

func TestExtractVecPayload_LengthExceedsData(t *testing.T) {
	// header(8) + len=999 but no body.
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[8:12], 999)
	if _, err := extractVecPayload(data, 8); err == nil {
		t.Fatal("expected error: declared vec length exceeds remaining bytes")
	}
}

func TestExtractVecPayload_ZeroHeader(t *testing.T) {
	// No header — data starts with the Vec length prefix directly.
	body := []byte{0xaa, 0xbb, 0xcc}
	data := make([]byte, 4, 4+len(body))
	binary.LittleEndian.PutUint32(data[:4], uint32(len(body)))
	data = append(data, body...)

	got, err := extractVecPayload(data, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("payload mismatch: got %x want %x", got, body)
	}
}

func TestExtractVecPayload_EmptyVec(t *testing.T) {
	// Header + zero-length Vec — should return an empty, non-nil slice.
	data := make([]byte, 12)
	// data[8:12] is already zero — vec length 0.

	got, err := extractVecPayload(data, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d bytes", len(got))
	}
}

func TestExtractCorrelationFields_MatchesEVMFormat(t *testing.T) {
	// EVM CCTP nonce is big-endian uint64 at offset 12 in the CCTP message.
	// Solana extraction must produce the same decimal string format.
	cctpMessage := make([]byte, 20)
	binary.BigEndian.PutUint64(cctpMessage[12:20], 670212)

	nonce, err := extractCorrelationFields(cctpMessage, []correlationField{
		{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if nonce != "670212" {
		t.Fatalf("expected nonce '670212' (EVM-compatible decimal), got %q", nonce)
	}
}

func TestExtractCorrelationFields_Keccak256(t *testing.T) {
	// Real on-chain CCTP V2 message bytes from Solana source transfers.
	// Hashes verified independently against the EVM MessageReceived topic[2].
	tests := []struct {
		name     string
		msgHex   string
		wantHash string
	}{
		{
			name:     "solana_to_base",
			msgHex:   "0000000100000005000000060000000000000000000000000000000000000000000000000000000000000000a65fc81d0fefa8860cb3b83f089b0224be8a6687b7ae49f594c0b9b4d7e9389300000000000000000000000028b5a0e9c621a5badaa536219b3a228c8168cf5d000000000000000000000000c1062b7c5dc8e4b1df9f200fe360cdc0ed6e7741000000010000000000000001c6fa7af3bedbad3a3d65f36aabc97431b1bbe4c2d2f6e0e47ca60203452f5d61000000000000000000000000c1062b7c5dc8e4b1df9f200fe360cdc0ed6e7741000000000000000000000000000000000000000000000000000000000137c8e980995a8681db8790ebd50b2b326a18199c27886f55cdefc48b463b4dd34a647400000000000000000000000000000000000000000000000000000000000008c90000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000cdeff8610854119222ebf4724821f369b6a5591b000000000000000000000000000000000000000000000000000000000000000000000000000fda22000000000000000000000000000047e4000000000000149400000000699664ef000000000000000000000000a5aa6e2171b416e1d27ec53ca8c13db3f91a89cd00",
			wantHash: "0xe7e0578cb8a6daa1cd65b75ed186b9bf98844305770cc25071a56c7ebe71d550",
		},
		{
			name:     "solana_to_arbitrum",
			msgHex:   "0000000100000005000000030000000000000000000000000000000000000000000000000000000000000000a65fc81d0fefa8860cb3b83f089b0224be8a6687b7ae49f594c0b9b4d7e9389300000000000000000000000028b5a0e9c621a5badaa536219b3a228c8168cf5d000000000000000000000000c1062b7c5dc8e4b1df9f200fe360cdc0ed6e7741000000010000000000000001c6fa7af3bedbad3a3d65f36aabc97431b1bbe4c2d2f6e0e47ca60203452f5d61000000000000000000000000c1062b7c5dc8e4b1df9f200fe360cdc0ed6e774100000000000000000000000000000000000000000000000000000000116aa83cbba6a9f99a1adec7d73a724725bfd09254e6ead780912c106ac86ea1162804670000000000000000000000000000000000000000000000000000000000007d9a0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000e4435e92f49ad60171bbd196dc0474c4649c67130000000000000000000000000000b7be000000000000000000000000a5aa6e2171b416e1d27ec53ca8c13db3f91a89cd000000000000000000000000000000000000000000000000000000000000000000",
			wantHash: "0x54808a142e3c0a3651a6b42e5002245042dde81bf17dbc7bfb3051869c9f8f5c",
		},
	}

	fields := []correlationField{{Offset: 0, Type: "keccak256", Field: "message_hash"}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := hex.DecodeString(tc.msgHex)
			if err != nil {
				t.Fatalf("failed to decode hex: %v", err)
			}
			got, err := extractCorrelationFields(data, fields)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantHash {
				t.Errorf("expected hash %q, got %q", tc.wantHash, got)
			}
		})
	}
}

func TestExtractCorrelationFields_Bytes32(t *testing.T) {
	// V2 uses bytes32 nonce at offset 12.
	cctpMessage := make([]byte, 44)

	nonce, err := extractCorrelationFields(cctpMessage, []correlationField{
		{Offset: 12, Size: 32, Type: "bytes32", Field: "nonce"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "0x0000000000000000000000000000000000000000000000000000000000000000"
	if nonce != expected {
		t.Fatalf("expected nonce %q, got %q", expected, nonce)
	}
}

func TestResolution_Resolve_SourceMatch(t *testing.T) {
	disc := [8]byte{0x83, 0x64, 0x85, 0x38, 0xa6, 0xe1, 0x97, 0x3c}
	rentPayer := make([]byte, 32)
	cctpMessage := make([]byte, 20)
	binary.BigEndian.PutUint64(cctpMessage[12:20], 670212)
	vecLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vecLen, uint32(len(cctpMessage)))

	data := make([]byte, 0, 44+len(cctpMessage))
	data = append(data, disc[:]...)
	data = append(data, rentPayer...)
	data = append(data, vecLen...)
	data = append(data, cctpMessage...)

	r := &Resolution{
		MessageProgramID:     "anyProg",
		AccountDiscriminator: disc,
		accountHeaderSize:    40,
		correlation: []correlationField{
			{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
		},
	}

	id, matched, err := r.Resolve(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Fatal("expected matched=true for matching discriminator")
	}
	if id != "670212" {
		t.Fatalf("id = %q, want %q", id, "670212")
	}
}

func TestResolution_Resolve_SourceDiscriminatorMismatch(t *testing.T) {
	r := &Resolution{
		AccountDiscriminator: [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		accountHeaderSize:    40,
		correlation: []correlationField{
			{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
		},
	}

	// Data with a different leading discriminator.
	data := make([]byte, 100)
	data[0] = 0xff

	id, matched, err := r.Resolve(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Fatal("expected matched=false for mismatched discriminator")
	}
	if id != "" {
		t.Fatalf("id should be empty on mismatch, got %q", id)
	}
}

func TestResolution_Resolve_SourceMatch_V2(t *testing.T) {
	// V2 source: disc(8) + rentPayer(32) + createdAt(8) + Vec<u8>.
	disc := [8]byte{0x83, 0x64, 0x85, 0x38, 0xa6, 0xe1, 0x97, 0x3c}
	rentPayer := make([]byte, 32)
	createdAt := make([]byte, 8)

	// V2 messages use a 32-byte nonce at offset 12 (bytes32 type) and
	// callers typically extract it via keccak256 over the full message.
	cctpMessage := make([]byte, 64)
	for i := range cctpMessage {
		cctpMessage[i] = byte(i)
	}
	vecLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vecLen, uint32(len(cctpMessage)))

	data := make([]byte, 0, 52+len(cctpMessage))
	data = append(data, disc[:]...)
	data = append(data, rentPayer...)
	data = append(data, createdAt...)
	data = append(data, vecLen...)
	data = append(data, cctpMessage...)

	r := &Resolution{
		MessageProgramID:     "anyProg",
		AccountDiscriminator: disc,
		accountHeaderSize:    48,
		correlation: []correlationField{
			{Offset: 0, Type: "keccak256", Field: "message_hash"},
		},
	}

	id, matched, err := r.Resolve(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Fatal("expected matched=true for matching V2 discriminator")
	}
	if id == "" || id[:2] != "0x" {
		t.Fatalf("expected hex-prefixed keccak hash, got %q", id)
	}
}

func TestResolution_Resolve_SourceShortData(t *testing.T) {
	r := &Resolution{
		AccountDiscriminator: [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		accountHeaderSize:    40,
	}

	if _, _, err := r.Resolve([]byte{0x01, 0x02}); err != nil {
		t.Fatalf("short data with no discriminator overlap should return matched=false, not error: %v", err)
	}
}

func TestResolution_Resolve_SourceMatchPayloadShort(t *testing.T) {
	// Discriminator matches but payload is too short to parse — this is
	// the only source path where matched=false and err!=nil are both
	// set.
	disc := [8]byte{0x83, 0x64, 0x85, 0x38, 0xa6, 0xe1, 0x97, 0x3c}
	data := append([]byte{}, disc[:]...) // disc only — no rentPayer, no Vec.

	r := &Resolution{
		AccountDiscriminator: disc,
		accountHeaderSize:    40,
		correlation: []correlationField{
			{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
		},
	}

	id, matched, err := r.Resolve(data)
	if err == nil {
		t.Fatal("expected error: discriminator matched but payload is truncated")
	}
	if matched {
		t.Fatal("matched should be false when the payload parse fails")
	}
	if id != "" {
		t.Fatalf("id should be empty on error, got %q", id)
	}
}

func TestResolution_Resolve_Destination(t *testing.T) {
	instrDisc := [8]byte{0x26, 0x90, 0x7f, 0xe1, 0x1f, 0xe1, 0xee, 0x19}
	cctpMessage := make([]byte, 20)
	binary.BigEndian.PutUint64(cctpMessage[12:20], 12345)
	vecLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vecLen, uint32(len(cctpMessage)))

	data := make([]byte, 0, 12+len(cctpMessage))
	data = append(data, instrDisc[:]...)
	data = append(data, vecLen...)
	data = append(data, cctpMessage...)

	r := &Resolution{
		MessageProgramID:         "anyProg",
		instructionDiscriminator: instrDisc,
		instructionHeaderSize:    8,
		correlation: []correlationField{
			{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
		},
	}

	id, matched, err := r.Resolve(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Fatal("destination resolve should report matched=true on success")
	}
	if id != "12345" {
		t.Fatalf("id = %q, want %q", id, "12345")
	}
}

func TestResolution_Resolve_DestinationDiscriminatorMismatch(t *testing.T) {
	// The footgun fix: passing source-shaped account data (with the
	// MessageSent discriminator 83648538a6e1973c) into a destination
	// Resolve must not silently produce a bogus correlation ID.
	r := &Resolution{
		instructionDiscriminator: [8]byte{0x26, 0x90, 0x7f, 0xe1, 0x1f, 0xe1, 0xee, 0x19},
		instructionHeaderSize:    8,
		correlation: []correlationField{
			{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
		},
	}

	// Build a plausible-looking source MessageSent account.
	sourceDisc := []byte{0x83, 0x64, 0x85, 0x38, 0xa6, 0xe1, 0x97, 0x3c}
	rentPayer := make([]byte, 32)
	cctpMessage := make([]byte, 20)
	binary.BigEndian.PutUint64(cctpMessage[12:20], 999999)
	vecLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vecLen, uint32(len(cctpMessage)))
	data := append(append(append(append([]byte{}, sourceDisc...), rentPayer...), vecLen...), cctpMessage...)

	id, matched, err := r.Resolve(data)
	if err != nil {
		t.Fatalf("expected nil error on discriminator mismatch, got %v", err)
	}
	if matched {
		t.Fatal("expected matched=false when destination discriminator does not match")
	}
	if id != "" {
		t.Fatalf("id should be empty on mismatch, got %q (the footgun returned bogus data)", id)
	}
}

func TestResolution_Resolve_DestinationShortData(t *testing.T) {
	r := &Resolution{
		instructionDiscriminator: [8]byte{0x26, 0x90, 0x7f, 0xe1, 0x1f, 0xe1, 0xee, 0x19},
		instructionHeaderSize:    8,
		correlation: []correlationField{
			{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
		},
	}

	// 5 bytes — too short even for the discriminator gate.
	_, matched, err := r.Resolve(make([]byte, 5))
	if err != nil {
		t.Fatalf("expected nil error: short data should fail the discriminator gate first: %v", err)
	}
	if matched {
		t.Fatal("matched should be false")
	}
}

func TestResolution_Resolve_DestinationDiscriminatorMatchPayloadShort(t *testing.T) {
	// Discriminator matches but the Vec<u8> length prefix is missing —
	// the parse path returns an error.
	instrDisc := [8]byte{0x26, 0x90, 0x7f, 0xe1, 0x1f, 0xe1, 0xee, 0x19}

	r := &Resolution{
		instructionDiscriminator: instrDisc,
		instructionHeaderSize:    8,
		correlation: []correlationField{
			{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
		},
	}

	if _, matched, err := r.Resolve(instrDisc[:]); err == nil || matched {
		t.Fatalf("expected matched=false and err!=nil for discriminator-only data, got matched=%t err=%v", matched, err)
	}
}

func TestResolution_Resolve_KeccakRoundTrip(t *testing.T) {
	// Real on-chain Solana → Base CCTP V2 message; verifies the Resolve
	// wrapper produces the same hash as the lower-level extractor.
	msgHex := "0000000100000005000000060000000000000000000000000000000000000000000000000000000000000000a65fc81d0fefa8860cb3b83f089b0224be8a6687b7ae49f594c0b9b4d7e9389300000000000000000000000028b5a0e9c621a5badaa536219b3a228c8168cf5d000000000000000000000000c1062b7c5dc8e4b1df9f200fe360cdc0ed6e7741000000010000000000000001c6fa7af3bedbad3a3d65f36aabc97431b1bbe4c2d2f6e0e47ca60203452f5d61000000000000000000000000c1062b7c5dc8e4b1df9f200fe360cdc0ed6e7741000000000000000000000000000000000000000000000000000000000137c8e980995a8681db8790ebd50b2b326a18199c27886f55cdefc48b463b4dd34a647400000000000000000000000000000000000000000000000000000000000008c90000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000cdeff8610854119222ebf4724821f369b6a5591b000000000000000000000000000000000000000000000000000000000000000000000000000fda22000000000000000000000000000047e4000000000000149400000000699664ef000000000000000000000000a5aa6e2171b416e1d27ec53ca8c13db3f91a89cd00"
	wantHash := "0xe7e0578cb8a6daa1cd65b75ed186b9bf98844305770cc25071a56c7ebe71d550"

	msg, err := hex.DecodeString(msgHex)
	if err != nil {
		t.Fatalf("hex decode: %v", err)
	}

	// Build ReceiveMessage instruction data: anchor disc(8) + vec len + msg.
	instrDisc := [8]byte{0x26, 0x90, 0x7f, 0xe1, 0x1f, 0xe1, 0xee, 0x19}
	vecLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vecLen, uint32(len(msg)))
	data := append(append(append([]byte{}, instrDisc[:]...), vecLen...), msg...)

	r := &Resolution{
		instructionDiscriminator: instrDisc,
		instructionHeaderSize:    8,
		correlation: []correlationField{
			{Offset: 0, Type: "keccak256", Field: "message_hash"},
		},
	}

	id, matched, err := r.Resolve(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Fatal("expected matched=true")
	}
	if id != wantHash {
		t.Fatalf("hash mismatch: got %q want %q", id, wantHash)
	}
}
