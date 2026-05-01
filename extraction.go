package bridgesolana

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// CorrelationDelimiter separates multiple correlation fields in the
// resulting correlation ID string.
const CorrelationDelimiter = ":"

// ExtractCorrelationFields extracts correlation field values from decoded
// event data and joins them with CorrelationDelimiter. The data parameter
// is the full decoded event bytes (including any 8-byte discriminator).
func ExtractCorrelationFields(data []byte, fields []CorrelationField) (string, error) {
	if len(fields) == 0 {
		return "", fmt.Errorf("no correlation fields configured")
	}

	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		value, err := extractFieldValue(data, field)
		if err != nil {
			return "", fmt.Errorf("failed to extract field %q: %w", field.Field, err)
		}
		parts = append(parts, value)
	}

	return strings.Join(parts, CorrelationDelimiter), nil
}

// ParseMessageSentAccount extracts CCTP message bytes from a MessageSent
// account's raw data.
//
// Account layout:
//   - V1 (version=0): discriminator(8) + rent_payer(32) + Vec<u8> at offset 40.
//   - V2 (version=1): discriminator(8) + rent_payer(32) + created_at(8) + Vec<u8> at offset 48.
//
// Vec<u8> layout: 4-byte LE length prefix + data.
func ParseMessageSentAccount(data []byte, version int) ([]byte, error) {
	var headerSize int
	switch version {
	case 0:
		headerSize = 40 // disc(8) + rent_payer(32).
	case 1:
		headerSize = 48 // disc(8) + rent_payer(32) + created_at(8).
	default:
		return nil, fmt.Errorf("unsupported message version %d", version)
	}

	return extractPayload(data, headerSize)
}

// ParseReceiveMessageInstructionData extracts CCTP message bytes from
// ReceiveMessage instruction data. Layout: Anchor discriminator(8) +
// Vec<u8> (4-byte LE length + data).
func ParseReceiveMessageInstructionData(data []byte) ([]byte, error) {
	return extractPayload(data, 8)
}

// extractPayload extracts a Borsh Vec<u8> payload from raw data, skipping
// headerSize bytes. Layout: header(headerSize) + Vec<u8> (4-byte LE length
// prefix + data).
func extractPayload(data []byte, headerSize int) ([]byte, error) {
	if len(data) < headerSize+4 {
		return nil, fmt.Errorf("data too short: need %d bytes, have %d", headerSize+4, len(data))
	}

	vecLen := binary.LittleEndian.Uint32(data[headerSize : headerSize+4])
	dataStart := headerSize + 4
	dataEnd := dataStart + int(vecLen)

	if dataEnd > len(data) {
		return nil, fmt.Errorf("vec length %d exceeds data (available: %d bytes)", vecLen, len(data)-dataStart)
	}

	return data[dataStart:dataEnd], nil
}

func extractFieldValue(data []byte, field CorrelationField) (string, error) {
	typ := strings.ToLower(strings.TrimSpace(field.Type))

	// keccak256 hashes from offset to end; does not use field.Size.
	if typ == "keccak256" {
		if field.Offset > len(data) {
			return "", fmt.Errorf("offset %d out of range (data %d bytes)", field.Offset, len(data))
		}
		return crypto.Keccak256Hash(data[field.Offset:]).Hex(), nil
	}

	end := field.Offset + field.Size
	if end > len(data) {
		return "", fmt.Errorf("offset %d+%d out of range (data has %d bytes)", field.Offset, field.Size, len(data))
	}

	rawBytes := data[field.Offset:end]

	switch typ {
	case "uint64_le":
		if field.Size != 8 {
			return "", fmt.Errorf("uint64_le requires 8 bytes, got %d", field.Size)
		}
		return fmt.Sprintf("%d", binary.LittleEndian.Uint64(rawBytes)), nil

	case "uint32_le":
		if field.Size != 4 {
			return "", fmt.Errorf("uint32_le requires 4 bytes, got %d", field.Size)
		}
		return fmt.Sprintf("%d", binary.LittleEndian.Uint32(rawBytes)), nil

	case "uint64":
		if field.Size != 8 {
			return "", fmt.Errorf("uint64 requires 8 bytes, got %d", field.Size)
		}
		return fmt.Sprintf("%d", binary.BigEndian.Uint64(rawBytes)), nil

	case "uint32":
		if field.Size != 4 {
			return "", fmt.Errorf("uint32 requires 4 bytes, got %d", field.Size)
		}
		return fmt.Sprintf("%d", binary.BigEndian.Uint32(rawBytes)), nil

	case "bytes32":
		if field.Size != 32 {
			return "", fmt.Errorf("bytes32 requires 32 bytes, got %d", field.Size)
		}
		return fmt.Sprintf("0x%x", rawBytes), nil

	case "pubkey":
		if field.Size != 32 {
			return "", fmt.Errorf("pubkey requires 32 bytes, got %d", field.Size)
		}
		// Base58 would be ideal, but hex suffices for correlation.
		return fmt.Sprintf("0x%x", rawBytes), nil

	default:
		// Generic big-endian integer.
		return new(big.Int).SetBytes(rawBytes).String(), nil
	}
}
