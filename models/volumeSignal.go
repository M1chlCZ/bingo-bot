package models

type VolumeSignal int

const (
	Rising VolumeSignal = iota
	Declining
	Neutral
)
