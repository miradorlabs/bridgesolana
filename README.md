# bridgesolana

[![CI](https://github.com/miradorlabs/bridgesolana/actions/workflows/ci.yml/badge.svg)](https://github.com/miradorlabs/bridgesolana/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/miradorlabs/bridgesolana.svg)](https://pkg.go.dev/github.com/miradorlabs/bridgesolana)
[![Go Report Card](https://goreportcard.com/badge/github.com/miradorlabs/bridgesolana)](https://goreportcard.com/report/github.com/miradorlabs/bridgesolana)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Detect cross-chain bridge events from Solana program logs.

> **Status:** pre-1.0. The public API may change in any `0.x.0` release; patch
> releases (`0.x.y`) will not break callers. See [CHANGELOG.md](CHANGELOG.md).

Build a `BridgeDetector` once at process start, then hand it the raw log strings from a Solana transaction's metadata. `Detect` returns one `Detection` per matched bridge event.

A `Detection` carries the bridge identity (name, description, leg type) plus one of:

- A populated `CorrelationID` — extracted in-band from an Anchor `Program data:` event. The detection is fully resolved; no RPC needed.
- A non-nil `Resolution` — the bridge fires through a CPI without emitting an event. The caller fetches the relevant on-chain bytes via RPC and passes them to `Resolution.Resolve`, which parses and extracts the correlation ID in one step.

## Usage

```go
import "github.com/miradorlabs/bridgesolana"

d, err := bridgesolana.NewBridgeDetector()
if err != nil {
    log.Fatal(err)
}

// `logs` is the slice of "Program ..." strings from a transaction's
// metadata (e.g. `meta.logMessages` from a getTransaction RPC response).
for _, det := range d.Detect(logs) {
    // Log-mode: the correlation ID is already on the detection.
    if det.Resolution == nil {
        fmt.Printf("%s %s leg, correlation %s\n",
            det.BridgeName, det.BridgeLegType, det.CorrelationID)
        continue
    }

    // Instruction-mode: resolve via RPC. What you fetch depends on the leg.
    var data []byte
    switch det.BridgeLegType {
    case bridgesolana.LegTypeSource:
        // Find the writable account in this transaction owned by
        // det.Resolution.MessageProgramID whose first 8 bytes match
        // det.Resolution.AccountDiscriminator. Resolve will re-check
        // the discriminator and return matched=false if you guessed
        // wrong, so it's also fine to call it on every candidate.
        data = fetchAccountData(det.Resolution.MessageProgramID, det.Resolution.AccountDiscriminator)

    case bridgesolana.LegTypeDestination:
        // The bytes are the instruction data of the bridge's
        // destination call (e.g. CCTP's ReceiveMessage) executed
        // by det.Resolution.MessageProgramID in this transaction.
        data = fetchInstructionData(det.Resolution.MessageProgramID)
    }

    correlationID, matched, err := det.Resolution.Resolve(data)
    if err != nil {
        log.Printf("resolve %s: %v", det.BridgeName, err)
        continue
    }
    if !matched {
        // Source leg: the candidate account didn't match the
        // expected discriminator — try the next one.
        // Destination leg: the bytes you passed don't look like the
        // expected destination instruction data.
        continue
    }
    fmt.Printf("%s %s leg, correlation %s\n",
        det.BridgeName, det.BridgeLegType, correlationID)
}
```

Both legs gate parsing on the leading 8-byte Anchor discriminator. Source legs use `matched=false` as a scan-and-skip signal so callers can iterate candidate accounts cheaply. Destination legs use it as a wrong-data-type guard — there is no candidate iteration to do, so `matched=false` on a destination resolve indicates the caller fed the wrong bytes (e.g. account data instead of instruction data) and should be logged.

## Coverage

| Bridge                | solana |
|-----------------------|:-:|
| CCTP V1 source        | ✓ |
| CCTP V1 destination   | ✓ |
| CCTP V2 source        | ✓ |
| CCTP V2 destination   | ✓ |

## API

```go
type BridgeDetector struct{ /* ... */ }

func NewBridgeDetector() (*BridgeDetector, error)
func (d *BridgeDetector) Detect(logs []string) []Detection
func (d *BridgeDetector) ProgramIDs() []string

type Detection struct {
    BridgeName        string
    BridgeDescription string
    BridgeLegType     LegType
    CorrelationID     string      // populated for log-mode detections
    Resolution        *Resolution // non-nil for instruction-mode detections
}

type Resolution struct {
    MessageProgramID     string
    AccountDiscriminator [8]byte // source legs only (zero for destination)
}

func (r *Resolution) Resolve(data []byte) (id string, matched bool, err error)

type LegType string
const (
    LegTypeSource      LegType = "source"
    LegTypeDestination LegType = "destination"
)
```

A `BridgeDetector` is read-only after construction and safe to share across goroutines.

## How it works

Bridge configurations are embedded JSON, one file per protocol (currently `config/cctp.json`). Each entry declares the program ID, the event or instruction name, and how to extract the correlation ID from the decoded payload — by fixed-offset uint64/uint32/bytes32 fields, or as a keccak256 hash of the message tail.

`NewBridgeDetector` builds two `O(1)` lookup maps:
- discriminator → log-mode subscription
- (programID, instructionName) → instruction-mode subscription

`Detect` is a streaming scan of the log lines that tracks the program invocation stack, so instruction names are only matched within their owning program's context.

For instruction-mode source legs, the correlation ID lives in an on-chain account created by the source instruction (CCTP, for example, calls this the `MessageSent` account, with V1 and V2 layouts of different sizes). For destination legs, it lives in the destination instruction's data (CCTP's `ReceiveMessage`). The per-bridge config supplies the discriminators and header sizes; `Resolution.Resolve` verifies them, decodes the trailing `Vec<u8>` body, and runs the configured correlation extraction in one step.

## License

MIT. See [LICENSE](LICENSE).
