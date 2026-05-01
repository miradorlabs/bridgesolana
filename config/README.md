# Bridge configuration schema

Each `*.json` file in this directory is an array of bridge definitions
loaded once at process start by `NewBridgeDetector` and embedded in the
binary via `//go:embed`. Keep entries small and self-contained — one
file per protocol (e.g. `cctp.json`).

The schema is defined in [`../config.go`](../config.go). This file is
the human-readable summary; the Go validation is the source of truth.

## Top-level shape

```json
[
  {
    "bridgeName": "cctp",
    "bridgeDescription": "Circle CCTP V1 - DepositForBurn (Source)",
    "bridgeProgram": { "programId": "..." },
    "bridgeEvent": { /* see below */ }
  }
]
```

| Field               | Required | Notes                                                                  |
|---------------------|----------|------------------------------------------------------------------------|
| `bridgeName`        | yes      | Stable identifier surfaced as `Detection.BridgeName`. Lowercase, kebab-case. |
| `bridgeDescription` | yes      | Free-text human description, surfaced as `Detection.BridgeDescription`. |
| `bridgeProgram.programId` | yes | Solana program ID used to scope log/instruction matches and reported by `BridgeDetector.ProgramIDs`. |
| `bridgeEvent`       | yes      | The detection rule. See below.                                          |

## `bridgeEvent`

```json
{
  "type": "source",                 // or "destination"
  "name": "SendMessage",            // event name (log mode) or instruction name (instruction mode)
  "detectionMode": "instruction",   // "log" (default) or "instruction"
  "anchorType": "event",            // log mode only: "event" or "account"

  "messageProgramId": "...",        // instruction mode only
  "accountDiscriminator": "hex8",   // instruction mode + source only
  "accountDataHeaderSize": 40,      // instruction mode + source only (required)
  "instructionDiscriminator": "hex8", // instruction mode + destination only (required)
  "dataHeaderSize": 8,              // instruction mode + destination only; defaults to 8

  "correlation": [ /* see below */ ]
}
```

### Common fields

| Field           | Required | Allowed values            | Notes |
|-----------------|----------|---------------------------|-------|
| `type`          | yes      | `"source"`, `"destination"` | Surfaces as `Detection.BridgeLegType`. |
| `name`          | yes      | string                    | Log mode: the Anchor event/account name (used for discriminator computation). Instruction mode: the instruction name as it appears in `Program log: Instruction: <Name>` runtime logs. |
| `detectionMode` | no       | `"log"` (default), `"instruction"` | See [Detection modes](../CLAUDE.md#detection-modes). |
| `correlation`   | yes      | array of correlation fields | Must contain at least one field. |

### Log mode

Used when the program emits an Anchor `emit!` event that carries the
correlation ID directly in a `Program data:` line. The detector matches
the leading 8-byte Anchor discriminator and extracts correlation fields
from the decoded bytes.

| Field         | Required | Notes |
|---------------|----------|-------|
| `anchorType`  | yes      | `"event"` for `emit!` events, `"account"` for account-discriminator-prefixed payloads. Used to derive the discriminator (`sha256("event:<name>")[:8]` or `sha256("account:<name>")[:8]`). |

Log mode `Detection`s arrive with `CorrelationID` populated and
`Resolution == nil` — no RPC follow-up needed.

### Instruction mode

Used when the program fires its bridge intent through CPI without
emitting an event. The detector matches `(programID, instructionName)`
on the runtime-emitted `Program log: Instruction: <Name>` lines and
returns a `Resolution` describing how to fetch and parse the payload.

#### Source legs (`type: "source"`)

The correlation ID lives in an account created by the instruction
(typically a PDA — for CCTP, the `MessageSent` account). The caller
fetches the candidate accounts owned by `messageProgramId` and passes
them to `Resolution.Resolve`, which verifies the leading 8-byte Anchor
discriminator and parses a `Vec<u8>` body that begins after the first
`accountDataHeaderSize` bytes of the account.

| Field                   | Required | Notes |
|-------------------------|----------|-------|
| `messageProgramId`      | yes      | Program owning the source account. |
| `accountDiscriminator`  | yes      | 8-byte hex (16 chars). Surfaced as `Resolution.AccountDiscriminator` so callers can scan a transaction's writable accounts. `Resolve` rejects non-matching accounts with `matched=false`. |
| `accountDataHeaderSize` | yes      | Total bytes before the `Vec<u8>` length prefix, **including** the 8-byte Anchor discriminator. Examples: CCTP V1 = 40 (`disc(8)+rentPayer(32)`), CCTP V2 = 48 (`disc(8)+rentPayer(32)+createdAt(8)`). Layouts vary by program; there is no universal default. |

#### Destination legs (`type: "destination"`)

The correlation ID lives in the raw instruction data of the destination
call (e.g. CCTP's `ReceiveMessage`). The caller passes that
instruction's data to `Resolution.Resolve`, which verifies the leading
8-byte Anchor instruction discriminator and parses a `Vec<u8>` body
that begins after the first `dataHeaderSize` bytes.

| Field                      | Required | Notes |
|----------------------------|----------|-------|
| `messageProgramId`         | yes      | Program executing the destination instruction. |
| `instructionDiscriminator` | yes      | 8-byte hex (16 chars) of the Anchor instruction discriminator (`sha256("global:<snake_case_name>")[:8]`). Verified by `Resolve`; mismatched bytes return `matched=false` to guard against the caller passing the wrong data. |
| `dataHeaderSize`           | no       | Total bytes before the `Vec<u8>` length prefix, **including** the 8-byte Anchor instruction discriminator. Defaults to 8 — matching Anchor's convention where the discriminator is the entire header. Override when the program wraps the discriminator with extra fixed-size fields. |

## `correlation`

Each entry pulls one value out of the parsed message bytes; values are
joined with `:` to form the opaque `CorrelationID` string. Treat the
resulting string as opaque — it is only meaningful as an equality key.

```json
{
  "offset": 12,
  "size": 8,
  "type": "uint64",
  "field": "nonce"
}
```

| Field    | Required | Notes |
|----------|----------|-------|
| `offset` | yes      | Byte offset into the decoded payload (>= 0). |
| `size`   | conditional | Byte length. Required for fixed-size types; ignored for `keccak256`. |
| `type`   | yes      | See type table below. |
| `field`  | no       | Human-readable label used in error messages only. Not part of the output ID. |

### Supported types

| Type        | Output                                 | Notes |
|-------------|----------------------------------------|-------|
| `uint64`    | decimal string                         | Big-endian; size must be 8. (CCTP V1 uses this for its nonce, matching EVM format.) |
| `uint32`    | decimal string                         | Big-endian; size must be 4. |
| `uint64_le` | decimal string                         | Little-endian; size must be 8. |
| `uint32_le` | decimal string                         | Little-endian; size must be 4. |
| `bytes32`   | `0x`-prefixed hex                      | Size must be 32. |
| `pubkey`    | `0x`-prefixed hex                      | Size must be 32. (Hex, not base58 — sufficient for correlation.) |
| `keccak256` | `0x`-prefixed hex (32 bytes)           | Hashes from `offset` to the end of the payload. `size` is ignored; use `offset: 0` to hash the full message. |
| _other_     | decimal string                         | Generic big-endian integer fallback over `size` bytes. Avoid relying on this for new bridges. |

## Adding a new bridge

1. Drop a new `<bridge>.json` in this directory (or extend an existing
   one). Run `go test ./...` — `validateBridgeConfig` will catch most
   schema errors at startup. The validation is in
   [`../config.go`](../config.go).
2. If the layout is new (a non-CCTP source-account header, a
   destination instruction with a non-Anchor 8-byte prefix, etc.), the
   parsers in [`../extraction.go`](../extraction.go) need extending
   too.
3. Add a fixture-driven detector test under
   [`../detector_test.go`](../detector_test.go) and update the
   coverage matrix in [`../README.md`](../README.md).
4. Land it via a `feat(config): add <bridge>` commit — release-please
   will roll it into the next minor.
