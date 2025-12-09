package bot

import "github.com/M1chlCZ/bingo-bot/models"

type PositionContext struct {
	State      models.MarketState
	ATR        float64
	ADX        float64
	EntryPrice float64
	StopPrice  float64

	DynamicRR float64
	ActualRR  float64
	Edge      float64

	StateConfidence float64

	PriceActionQuality float64
	MomentumQuality    float64
	Noise              float64
	MultiTFTrendScore  float64

	RsiSlope  float64
	HistSlope float64
	RSI       float64

	MarketRegime models.MarketRegime
}

func clampf(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func positionContextMultiplier(ctx PositionContext) float64 {
	m := 1.0

	switch ctx.State {
	case models.StronglyTrending:
		m *= 1.25
	case models.Trending:
		m *= 1.10
	case models.RangeBound:
		m *= 0.90
	case models.Chaotic:
		m *= 0.70
	case models.Transitional:
		m *= 1.00
	default: // Default/unknown
		m *= 1.00
	}

	if ctx.ADX > 0 {
		switch {
		case ctx.ADX < 15:
			m *= 0.60
		case ctx.ADX < 20:
			m *= 0.80
		case ctx.ADX > 40:
			m *= 1.05
		}
	}

	if ctx.ATR > 0 && ctx.EntryPrice > 0 {
		atrPct := ctx.ATR / ctx.EntryPrice
		switch {
		case atrPct > 0.08:

			m *= 0.40
		case atrPct > 0.05:
			m *= 0.60
		case atrPct > 0.03:
			m *= 0.85
		default:

		}
	}

	if ctx.PriceActionQuality > 0 || ctx.MomentumQuality > 0 {
		paq := ctx.PriceActionQuality
		mq := ctx.MomentumQuality

		if paq > 0 && paq < 0.35 {
			m *= 0.70
		}
		if mq > 0 && mq < 0.35 {
			m *= 0.75
		}

		if paq >= 0.55 && mq >= 0.55 {
			m *= 1.10
		}
	}

	if ctx.Noise > 0 {
		switch {
		case ctx.Noise >= 0.80:
			m *= 0.60
		case ctx.Noise >= 0.70:
			m *= 0.75
		case ctx.Noise >= 0.60:
			m *= 0.90
		}
	}

	if ctx.MarketRegime == models.RangeBoundRegime {
		m *= 0.95
	}

	if ctx.MultiTFTrendScore > 0 {
		if ctx.MultiTFTrendScore >= 0.60 {
			m *= 1.05
		} else if ctx.MultiTFTrendScore < 0.35 {
			m *= 0.85
		}
	}

	if ctx.RsiSlope > 0 && ctx.HistSlope > 0 {
		m *= 1.03 // both sloping up → tiny reward
	} else if ctx.RsiSlope < 0 && ctx.HistSlope < 0 {
		m *= 0.90 // both sloping down → small cut
	}

	if ctx.Edge > 0 {

		switch {
		case ctx.Edge >= 1.20:
			m *= 1.10
		case ctx.Edge >= 1.05:
			m *= 1.03
		case ctx.Edge < 0.95:
			m *= 0.80
		}
	}

	if ctx.DynamicRR > 0 && ctx.ActualRR > 0 {
		rrRatio := ctx.ActualRR / ctx.DynamicRR
		switch {
		case rrRatio >= 1.30:
			m *= 1.10
		case rrRatio >= 1.10:
			m *= 1.05
		case rrRatio < 0.90:
			m *= 0.80
		}
	}

	if ctx.StateConfidence > 0 {

		m *= (0.9 + 0.3*clampf(ctx.StateConfidence, 0.0, 1.0))
	}

	return clampf(m, 0.25, 1.80)
}
