package models

type VolatilityRegime int

const (
	NormalVolatilityRegime VolatilityRegime = iota
	LowVolatilityRegime
	HighVolatilityRegime
)

func (vr VolatilityRegime) String() string {
	switch vr {
	case NormalVolatilityRegime:
		return "Normal"
	case LowVolatilityRegime:
		return "Low"
	case HighVolatilityRegime:
		return "High"
	default:
		return "Unknown"
	}
}
