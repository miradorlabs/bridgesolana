// Package bridgesolana detects cross-chain bridge events from Solana program logs.
//
// Build a BridgeDetector once per chain, then hand it the raw log strings
// from a Solana transaction. The detector supports two detection modes:
//
//   - Log mode: scans "Program data:" lines, decodes base64, matches 8-byte
//     Anchor discriminators, and returns BridgeDetails with the bridge name,
//     leg type (source / destination), and a correlation ID linking the leg
//     to its counterpart on the other chain.
//   - Instruction mode: scans "Program log: Instruction: <Name>" lines within
//     target program invocations and returns InstructionDetection records that
//     identify which bridge leg fired. Callers then resolve the correlation ID
//     by reading the relevant account or instruction data from RPC and feeding
//     it into ExtractPayload / ExtractCorrelationFields.
//
// Bridge configurations are embedded JSON, currently covering Circle CCTP V1
// and V2 source/destination legs on Solana. See the README for the full
// coverage matrix.
package bridgesolana
