package bridgesolana

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestParseMessageSentAccount_V1(t *testing.T) {
	// V1 layout: disc(8) + rent_payer(32) + Vec<u8>(4-byte len + data).
	// Total header before Vec data: 40 + 4 = 44 bytes.
	disc := []byte{0x83, 0x64, 0x85, 0x38, 0xa6, 0xe1, 0x97, 0x3c}
	rentPayer := make([]byte, 32)

	// CCTP message: at least 20 bytes (nonce at offset 12, 8 bytes BE).
	cctpMessage := make([]byte, 20)
	binary.BigEndian.PutUint64(cctpMessage[12:20], 670212)

	vecLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vecLen, uint32(len(cctpMessage)))

	data := make([]byte, 0, 44+len(cctpMessage))
	data = append(data, disc...)
	data = append(data, rentPayer...)
	data = append(data, vecLen...)
	data = append(data, cctpMessage...)

	msgBytes, err := ParseMessageSentAccount(data, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgBytes) != len(cctpMessage) {
		t.Fatalf("expected message length %d, got %d", len(cctpMessage), len(msgBytes))
	}

	nonce, err := ExtractCorrelationFields(msgBytes, []CorrelationField{
		{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
	})
	if err != nil {
		t.Fatalf("unexpected error extracting nonce: %v", err)
	}
	if nonce != "670212" {
		t.Fatalf("expected nonce '670212', got %q", nonce)
	}
}

func TestParseMessageSentAccount_V2(t *testing.T) {
	// V2 layout: disc(8) + rent_payer(32) + created_at(8) + Vec<u8>(4-byte len + data).
	// Total header before Vec data: 48 + 4 = 52 bytes.
	disc := []byte{0x83, 0x64, 0x85, 0x38, 0xa6, 0xe1, 0x97, 0x3c}
	rentPayer := make([]byte, 32)
	createdAt := make([]byte, 8)

	cctpMessage := make([]byte, 44)

	vecLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vecLen, uint32(len(cctpMessage)))

	data := make([]byte, 0, 52+len(cctpMessage))
	data = append(data, disc...)
	data = append(data, rentPayer...)
	data = append(data, createdAt...)
	data = append(data, vecLen...)
	data = append(data, cctpMessage...)

	msgBytes, err := ParseMessageSentAccount(data, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgBytes) != len(cctpMessage) {
		t.Fatalf("expected message length %d, got %d", len(cctpMessage), len(msgBytes))
	}
}

func TestParseMessageSentAccount_TooShort(t *testing.T) {
	data := make([]byte, 10)
	_, err := ParseMessageSentAccount(data, 0)
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestParseMessageSentAccount_InvalidVersion(t *testing.T) {
	data := make([]byte, 100)
	_, err := ParseMessageSentAccount(data, 99)
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestParseReceiveMessageInstructionData(t *testing.T) {
	// Layout: anchor disc(8) + Vec<u8>(4-byte LE len + data).
	anchorDisc := make([]byte, 8)
	cctpMessage := make([]byte, 20)
	binary.BigEndian.PutUint64(cctpMessage[12:20], 12345)

	vecLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vecLen, uint32(len(cctpMessage)))

	data := make([]byte, 0, 12+len(cctpMessage))
	data = append(data, anchorDisc...)
	data = append(data, vecLen...)
	data = append(data, cctpMessage...)

	msgBytes, err := ParseReceiveMessageInstructionData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgBytes) != len(cctpMessage) {
		t.Fatalf("expected message length %d, got %d", len(cctpMessage), len(msgBytes))
	}

	nonce, err := ExtractCorrelationFields(msgBytes, []CorrelationField{
		{Offset: 12, Size: 8, Type: "uint64", Field: "nonce"},
	})
	if err != nil {
		t.Fatalf("unexpected error extracting nonce: %v", err)
	}
	if nonce != "12345" {
		t.Fatalf("expected nonce '12345', got %q", nonce)
	}
}

func TestParseReceiveMessageInstructionData_TooShort(t *testing.T) {
	data := make([]byte, 5)
	_, err := ParseReceiveMessageInstructionData(data)
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestExtractCorrelationFields_MatchesEVMFormat(t *testing.T) {
	// EVM CCTP nonce is big-endian uint64 at offset 12 in the CCTP message.
	// Solana extraction must produce the same decimal string format.
	cctpMessage := make([]byte, 20)
	binary.BigEndian.PutUint64(cctpMessage[12:20], 670212)

	nonce, err := ExtractCorrelationFields(cctpMessage, []CorrelationField{
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

	fields := []CorrelationField{{Offset: 0, Type: "keccak256", Field: "message_hash"}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := hex.DecodeString(tc.msgHex)
			if err != nil {
				t.Fatalf("failed to decode hex: %v", err)
			}
			got, err := ExtractCorrelationFields(data, fields)
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

	nonce, err := ExtractCorrelationFields(cctpMessage, []CorrelationField{
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
