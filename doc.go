// Package bridgesolana detects cross-chain bridge events from Solana program logs.
//
// Build a [BridgeDetector] once at process start, then hand it the raw
// log strings from a Solana transaction's metadata.
// [BridgeDetector.Detect] returns one [Detection] per matched bridge
// event.
//
// A [Detection] carries the bridge identity (name, description, leg
// type) and one of two payloads:
//
//   - A populated CorrelationID — extracted in-band from an Anchor
//     "Program data:" event. The detection is fully resolved.
//   - A non-nil [Resolution] — the bridge fires through a CPI without
//     emitting an event. The caller fetches the relevant on-chain data
//     via RPC and feeds it through [ParseMessageSentAccount] or
//     [ParseReceiveMessageInstructionData] and then
//     [ExtractCorrelationFields] to produce the correlation ID.
//
// Bridge configurations are embedded JSON, currently covering Circle CCTP
// V1 and V2 source/destination legs.
package bridgesolana
