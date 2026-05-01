# bridgesolana

[![CI](https://github.com/miradorlabs/bridgesolana/actions/workflows/ci.yml/badge.svg)](https://github.com/miradorlabs/bridgesolana/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/miradorlabs/bridgesolana.svg)](https://pkg.go.dev/github.com/miradorlabs/bridgesolana)
[![Go Report Card](https://goreportcard.com/badge/github.com/miradorlabs/bridgesolana)](https://goreportcard.com/report/github.com/miradorlabs/bridgesolana)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Detect cross-chain bridge events from Solana program logs.

> **Status:** pre-1.0. The public API may change in any `0.x.0` release; patch
> releases (`0.x.y`) will not break callers. See [CHANGELOG.md](CHANGELOG.md).

Build a `BridgeDetector` once per chain, then hand it the raw log strings from a Solana transaction. The detector supports two complementary modes:

- **Log mode** (`DetectFromLogs`) — scans `Program data:` lines, decodes base64, matches 8-byte Anchor discriminators, and returns `BridgeDetails` directly with the bridge name, leg type, and correlation ID.
- **Instruction mode** (`DetectInstructionBridges`) — scans `Program log: Instruction: <Name>` lines within target program invocations and returns `InstructionDetection` records. Callers then resolve the correlation ID by reading the relevant account or instruction data from RPC and feeding it into `ExtractPayload` / `ExtractCorrelationFields`.

```go
import (
    "github.com/miradorlabs/bridgesolana"
)

detector, err := bridgesolana.NewBridgeDetector("solana")
if err != nil {
    log.Fatal(err)
}

// Log-mode detections (self-contained — correlation ID extracted in-band).
for _, d := range detector.DetectFromLogs(logs) {
    fmt.Printf("%s %s leg, correlation %s\n",
        d.BridgeName, d.BridgeLegType, d.CorrelationID)
}

// Instruction-mode detections (need RPC follow-up to read the message bytes).
for _, det := range detector.DetectInstructionBridges(logs) {
    sub := det.Subscription
    fmt.Printf("%s %s leg fired in %s — fetch instruction/account data and call ExtractPayload\n",
        sub.BridgeName, sub.BridgeLegType, det.ProgramID)
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

func NewBridgeDetector(chainName string) (*BridgeDetector, error)
func (d *BridgeDetector) ChainName() string
func (d *BridgeDetector) DetectFromLogs(logs []string) []*BridgeDetails
func (d *BridgeDetector) DetectInstructionBridges(logs []string) []*InstructionDetection

type BridgeDetails struct {
    BridgeLegType     LegType    // "source" or "destination"
    CorrelationID     string
    BridgeName        string
    BridgeDescription string
}

type InstructionDetection struct {
    Subscription *BridgeSubscription
    ProgramID    string
}

type LegType string
const (
    LegTypeSource      LegType = "source"
    LegTypeDestination LegType = "destination"
)

// Lower-level helpers for instruction-mode RPC resolution.
func ExtractPayload(data []byte, headerSize int) ([]byte, error)
func ExtractCorrelationFields(data []byte, fields []correlationField) (string, error)
func ParseMessageSentAccount(data []byte, version int) ([]byte, error)
func ParseReceiveMessageInstructionData(data []byte) ([]byte, error)

// Build subscriptions and program-ID lists for an RPC subscriber.
type Resolver struct{ /* ... */ }
func NewResolver(logger *zap.Logger) *Resolver
func (r *Resolver) SubscriptionsForChain(chain string) ([]*BridgeSubscription, error)
func (r *Resolver) ProgramIDs(chain string) ([]string, error)
```

A `BridgeDetector` is read-only after construction and safe to share across goroutines.

## How it works

Bridge configurations are embedded JSON, one file per protocol per chain (currently `config/solana/cctp.json`). Each entry declares the program ID, the event or instruction name, and how to extract the correlation ID from the decoded payload — by fixed-offset uint64/uint32/bytes32 fields, or as a keccak256 hash of the message tail.

`NewBridgeDetector` builds two `O(1)` lookup maps:
- discriminator → subscription (log mode)
- (programID, instructionName) → subscription (instruction mode)

Detection is a streaming scan of the log lines, tracking the program invocation stack so that instruction names are only matched within their owning program's context.

For instruction-mode source legs, the correlation ID lives in the on-chain `MessageSent` account whose data layout depends on the message version (v1 vs v2). Use `ParseMessageSentAccount` after fetching the account from RPC. For destination legs, the correlation ID lives in the `ReceiveMessage` instruction data — use `ParseReceiveMessageInstructionData` once you've located that instruction in the transaction.

## License

MIT. See [LICENSE](LICENSE).
