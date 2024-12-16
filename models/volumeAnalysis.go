package models

type VolumeAnalysis int

const (
	Stable VolumeAnalysis = iota
	NeutralVolume
	Rising
	Falling
)

// String converts MarketState to a string representation
func (v VolumeAnalysis) String() string {
	switch v {
	case Stable:
		return "stable"
	case NeutralVolume:
		return "neutral"
	case Rising:
		return "rising"
	case Falling:
		return "falling"
	default:
		return "stable"
	}
}
