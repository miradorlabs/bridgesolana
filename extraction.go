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
// return ("", false, nil) when it does not match. For source legs the
// scan-and-skip semantic lets callers walk a transaction's candidate
// accounts cheaply; for destination legs matched=false indicates the
// caller fed the wrong instruction data and should be treated as a
// data-shape failure.
//
// matched=false can mean two distinct things: discriminator mismatch
// (err=nil) or the discriminator matched but the body parse failed
// (err!=nil). Callers must check err alongside matched — writing
// `if !matched { continue }` alone silently drops parse errors.
//
// For source legs (AccountDiscriminator non-zero), pass the full source
// account data including its leading 8-byte Anchor discriminator. Resolve
// verifies the discriminator, parses the account body, and returns
// (id, true, nil) on success.
//
// For destination legs (AccountDiscriminator zero), pass the full
// destination instruction data including its leading 8-byte Anchor
// instruction discriminator. Resolve verifies the discriminator,
// extracts the message bytes, and returns (id, true, nil) on success.
//
// Resolve dispatches between source and destination by AccountDiscriminator
// being non-zero. This encodes the "source = account, destination =
// instruction data" pattern that fits every Anchor bridge surveyed so
// far. A future bridge with an account-backed destination leg would need
// an explicit LegType field on Resolution.
//
// Data-shape errors past the discriminator gate (truncated payload,
// malformed correlation field) return ("", false, err).
func (r *Resolution) Resolve(data []byte) (id string, matched bool, err error) {
	if r.AccountDiscriminator == ([8]byte{}) {
		if r.instructionDiscriminator != ([8]byte{}) {
			if len(data) < 8 || !bytes.Equal(data[:8], r.instructionDiscriminator[:]) {
				return "", false, nil
			}
		}
		return r.parseAndExtract(data, r.instructionHeaderSize)
	}

	if len(data) < 8 || !bytes.Equal(data[:8], r.AccountDiscriminator[:]) {
		return "", false, nil
	}
	return r.parseAndExtract(data, r.accountHeaderSize)
}

func (r *Resolution) parseAndExtract(data []byte, headerSize int) (string, bool, error) {
	msgBytes, err := extractVecPayload(data, headerSize)
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
// decoded event or message data and joins them into a single opaque
// correlation ID.
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

// extractVecPayload reads a Borsh Vec<u8> body that follows a
// fixed-size header. Layout: header(headerSize) + Vec<u8> (4-byte
// little-endian length prefix + data). The header is opaque to this
// function; per-bridge configs supply the byte count, and headerSize
// of 0 is valid (the Vec<u8> starts at offset 0). The little-endian
// length prefix matches Borsh, which is what every Anchor program
// uses; programs with a different wire format would need a new
// extraction primitive, not just config.
func extractVecPayload(data []byte, headerSize int) ([]byte, error) {
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
