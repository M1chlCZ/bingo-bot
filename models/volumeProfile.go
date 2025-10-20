package models

type VolumeProfile int

const (
	BalancedProfile VolumeProfile = iota
	UnbalancedProfile
)
