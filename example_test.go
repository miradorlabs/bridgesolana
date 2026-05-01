package bridgesolana_test

import (
	"fmt"

	"github.com/miradorlabs/bridgesolana"
)

// ExampleNewBridgeDetector shows how to construct a detector for a
// specific chain. NewBridgeDetector is cheap and safe to call once
// at process start.
func ExampleNewBridgeDetector() {
	d, err := bridgesolana.NewBridgeDetector("solana")
	if err != nil {
		panic(err)
	}
	fmt.Println(d.ChainName())
	// Output: solana
}

// ExampleBridgeDetector_DetectInstructionBridges shows the common path
// for instruction-mode detection: pass the raw program log strings from
// a Solana transaction and read off the bridge name and leg type.
func ExampleBridgeDetector_DetectInstructionBridges() {
	d, _ := bridgesolana.NewBridgeDetector("solana")

	logs := []string{
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd invoke [1]",
		"Program log: Instruction: ReceiveMessage",
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd success",
	}

	for _, det := range d.DetectInstructionBridges(logs) {
		fmt.Printf("%s %s leg on %s\n",
			det.Subscription.BridgeName,
			det.Subscription.BridgeLegType,
			det.ProgramID)
	}
	// Output: cctp destination leg on CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd
}
