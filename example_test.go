package bridgesolana_test

import (
	"fmt"

	"github.com/miradorlabs/bridgesolana"
)

// ExampleNewBridgeDetector shows how to construct a detector. It is cheap
// and safe to call once at process start.
func ExampleNewBridgeDetector() {
	_, err := bridgesolana.NewBridgeDetector()
	fmt.Println(err)
	// Output: <nil>
}

// ExampleBridgeDetector_Detect shows the common path: pass the raw program
// log strings from a Solana transaction and read off each detection. A
// Detection with a populated CorrelationID is fully resolved; one with a
// non-nil Resolution requires the caller to fetch the relevant account or
// instruction data from RPC and feed it through the package helpers.
func ExampleBridgeDetector_Detect() {
	d, _ := bridgesolana.NewBridgeDetector()

	logs := []string{
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd invoke [1]",
		"Program log: Instruction: ReceiveMessage",
		"Program CCTPmbSD7gX1bxKPAmg77w8oFzNFpaQiQUWD43TKaecd success",
	}

	for _, det := range d.Detect(logs) {
		fmt.Printf("%s %s leg (needs RPC follow-up: %t)\n",
			det.BridgeName, det.BridgeLegType, det.Resolution != nil)
	}
	// Output: cctp destination leg (needs RPC follow-up: true)
}
