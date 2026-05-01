package bridgesolana

// LegType identifies whether a detected bridge event is the source leg
// or the destination leg of a cross-chain transfer.
type LegType string

const (
	LegTypeSource      LegType = "source"
	LegTypeDestination LegType = "destination"
)
