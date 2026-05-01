package bridgesolana

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// correlationDelimiter joins multiple correlation field values into the
// final correlation ID string. Treat the resulting string as opaque —
// the format is not part of the public contract.
const correlationDelimiter = ":"

// Resolve extracts the correlation ID from raw on-chain data.
//
// Both legs gate parsing on the leading 8-byte Anchor discriminator and
// return ("", false, nil) when it does not match — for source legs that
// is the scan-and-skip signal callers use to walk a transaction's
// candidate accounts; for destination legs it indicates the bytes did
// not come from the expected ReceiveMessage instruction. Treat
// matched=false as a data-shape failure on destination dispatch.
//
// For source legs (AccountDiscriminator non-zero), pass the full
// MessageSent account data including the leading 8-byte Anchor
// discriminator. Resolve verifies the discriminator, parses the
// MessageSent layout (selected by messageVersion), and returns
// (id, true, nil) on success.
//
// For destination legs (AccountDiscriminator zero), pass the
// ReceiveMessage instruction data including its 8-byte Anchor
// instruction discriminator. Resolve verifies the discriminator,
// extracts the message bytes, and returns (id, true, nil) on success.
//
// Data-shape errors past the discriminator gate (truncated payload,
// unsupported version, malformed fields) return ("", false, err).
func (r *Resolution) Resolve(data []byte) (id string, matched bool, err error) {
	if r.AccountDiscriminator == ([8]byte{}) {
		if r.instructionDiscriminator != ([8]byte{}) {
			if len(data) < 8 || !bytes.Equal(data[:8], r.instructionDiscriminator[:]) {
				return "", false, nil
			}
		}
		msgBytes, err := parseReceiveMessageInstructionData(data)
		if err != nil {
			return "", false, err
		}
		correlationID, err := extractCorrelationFields(msgBytes, r.correlation)
		if err != nil {
			return "", false, err
		}
		return correlationID, true, nil
	}

	if len(data) < 8 || !bytes.Equal(data[:8], r.AccountDiscriminator[:]) {
		return "", false, nil
	}
	msgBytes, err := parseMessageSentAccount(data, r.messageVersion)
	if err != nil {
		return "", false, err
	}
	correlationID, err := extractCorrelationFields(msgBytes, r.correlation)
	if err != nil {
		return "", false, err
	}
	return correlationID, true, nil
}

// extractCorrelationFields extracts correlation field values from
// decoded event data and joins them into a single opaque correlation
// ID. The data parameter is the full decoded event bytes (including any
// 8-byte discriminator).
func extractCorrelationFields(data []byte, fields []correlationField) (string, error) {
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

	return strings.Join(parts, correlationDelimiter), nil
}

// parseMessageSentAccount extracts CCTP message bytes from a MessageSent
// account's raw data.
//
// Account layout:
//   - V1 (version=0): discriminator(8) + rent_payer(32) + Vec<u8> at offset 40.
//   - V2 (version=1): discriminator(8) + rent_payer(32) + created_at(8) + Vec<u8> at offset 48.
//
// Vec<u8> layout: 4-byte LE length prefix + data.
func parseMessageSentAccount(data []byte, version int) ([]byte, error) {
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

// parseReceiveMessageInstructionData extracts CCTP message bytes from
// ReceiveMessage instruction data. Layout: Anchor discriminator(8) +
// Vec<u8> (4-byte LE length + data).
func parseReceiveMessageInstructionData(data []byte) ([]byte, error) {
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

func extractFieldValue(data []byte, field correlationField) (string, error) {
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
		return strconv.FormatUint(binary.LittleEndian.Uint64(rawBytes), 10), nil

	case "uint32_le":
		if field.Size != 4 {
			return "", fmt.Errorf("uint32_le requires 4 bytes, got %d", field.Size)
		}
		return strconv.FormatUint(uint64(binary.LittleEndian.Uint32(rawBytes)), 10), nil

	case "uint64":
		if field.Size != 8 {
			return "", fmt.Errorf("uint64 requires 8 bytes, got %d", field.Size)
		}
		return strconv.FormatUint(binary.BigEndian.Uint64(rawBytes), 10), nil

	case "uint32":
		if field.Size != 4 {
			return "", fmt.Errorf("uint32 requires 4 bytes, got %d", field.Size)
		}
		return strconv.FormatUint(uint64(binary.BigEndian.Uint32(rawBytes)), 10), nil

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
