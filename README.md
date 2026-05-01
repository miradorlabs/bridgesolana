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

- A populated `CorrelationID` — extracted in-band from an Anchor `Program data:` event. The detection is fully resolved.
- A non-nil `Resolution` — the bridge fires through a CPI without emitting an event. The caller fetches the relevant on-chain data via RPC and feeds it through the package helpers (`ParseMessageSentAccount` / `ParseReceiveMessageInstructionData`, then `ExtractCorrelationFields`) to produce the correlation ID.

```go
import "github.com/miradorlabs/bridgesolana"

d, err := bridgesolana.NewBridgeDetector()
if err != nil {
    log.Fatal(err)
}

for _, det := range d.Detect(logs) {
    if det.Resolution == nil {
        // Log-mode: correlation ID is already extracted.
        fmt.Printf("%s %s leg, correlation %s\n",
            det.BridgeName, det.BridgeLegType, det.CorrelationID)
        continue
    }

    // Instruction-mode: fetch the message bytes via RPC, then extract.
    msgBytes, _ := bridgesolana.ParseMessageSentAccount(accountData, det.Resolution.MessageVersion)
    correlationID, _ := bridgesolana.ExtractCorrelationFields(msgBytes, det.Resolution.Correlation)
    fmt.Printf("%s %s leg, correlation %s\n",
        det.BridgeName, det.BridgeLegType, correlationID)
}
```

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
    MessageVersion       int     // 0 = V1, 1 = V2
    Correlation          []CorrelationField
}

type CorrelationField struct {
    Offset int
    Size   int
    Type   string
    Field  string
}

type LegType string
const (
    LegTypeSource      LegType = "source"
    LegTypeDestination LegType = "destination"
)

// RPC follow-up helpers for instruction-mode detections.
func ParseMessageSentAccount(data []byte, version int) ([]byte, error)
func ParseReceiveMessageInstructionData(data []byte) ([]byte, error)
func ExtractCorrelationFields(data []byte, fields []CorrelationField) (string, error)
```

A `BridgeDetector` is read-only after construction and safe to share across goroutines.

## How it works

Bridge configurations are embedded JSON, one file per protocol (currently `config/cctp.json`). Each entry declares the program ID, the event or instruction name, and how to extract the correlation ID from the decoded payload — by fixed-offset uint64/uint32/bytes32 fields, or as a keccak256 hash of the message tail.

`NewBridgeDetector` builds two `O(1)` lookup maps:
- discriminator → log-mode subscription
- (programID, instructionName) → instruction-mode subscription

`Detect` is a streaming scan of the log lines that tracks the program invocation stack, so instruction names are only matched within their owning program's context.

For instruction-mode source legs, the correlation ID lives in the on-chain `MessageSent` account whose layout depends on the message version (V1 vs V2). For destination legs, it lives in the `ReceiveMessage` instruction data. The `Resolution` returned with each detection carries everything the caller needs to fetch and parse it.

## License

MIT. See [LICENSE](LICENSE).
