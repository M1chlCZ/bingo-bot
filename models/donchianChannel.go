package models

type DonchianChannel int

const (
	NeutralChannel DonchianChannel = iota
	RangeChannel
	BreakoutChannel
	MixedChannel
)

func (d DonchianChannel) String() string {
	switch d {
	case NeutralChannel:
		return "neutral"
	case RangeChannel:
		return "range"
	case BreakoutChannel:
		return "breakout"
	case MixedChannel:
		return "mixed"
	default:
		return "mixed"
	}
}
