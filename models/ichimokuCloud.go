package models

type IchimokuCloud int

const (
	NeutralIchi IchimokuCloud = iota
	BullishIchi
	BearishIchi
)

func (i IchimokuCloud) String() string {
	switch i {
	case NeutralIchi:
		return "neutral"
	case BullishIchi:
		return "bullish"
	case BearishIchi:
		return "bearish"
	default:
		return "neutral"
	}
}
