package models

type VolumeProfileType int

const (
	Balanced VolumeProfileType = iota
	Unbalanced
)
