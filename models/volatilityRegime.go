package models

type VolatilityRegime int

const (
	NormalVolatilityRegime VolatilityRegime = iota
	LowVolatilityRegime
	HighVolatilityRegime
)
