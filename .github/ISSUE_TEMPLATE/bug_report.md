---
name: Bug report
about: Report a defect in detection or correlation extraction
title: "fix: "
labels: bug
---

**Describe the bug**

A clear description of what is wrong.

**To reproduce**

Minimal Go snippet, ideally with the offending program logs:

```go
detector, _ := bridgesolana.NewBridgeDetector("solana")

logs := []string{
    "Program <ID> invoke [1]",
    "Program log: Instruction: <Name>",
    "Program <ID> success",
}

details := detector.DetectFromLogs(logs)
detections := detector.DetectInstructionBridges(logs)
```

**Expected behaviour**

What you expected the detector to return.

**Actual behaviour**

What it actually returned.

**On-chain reference**

If applicable: tx signature, slot, source/destination chain.

**Environment**

- bridgesolana version (commit SHA or tag):
- Go version:
- OS / arch:
