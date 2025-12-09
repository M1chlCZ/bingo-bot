package analysis

import (
	"math"
	"sort"
	"sync"

	"github.com/M1chlCZ/bingo-bot/algos"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/go-playground/validator/v10"
)

type MarketAnalyzer struct {
	ATRPeriod               int     `validate:"required" json:"atrPeriod"`               // Period for ATR calculation
	ADXPeriod               int     `validate:"required" json:"adxPeriod"`               // Period for ADX calculation
	HighVolatilityThreshold float64 `validate:"required" json:"highVolatilityThreshold"` // Threshold to identify high volatility
	StrongTrendThreshold    float64 `validate:"required" json:"strongTrendThreshold"`    // Threshold to identify strong trends

	IchimokuConversionPeriod int     `validate:"required" json:"ichimokuConversionPeriod"`
	IchimokuBasePeriod       int     `validate:"required" json:"ichimokuBasePeriod"`
	IchimokuSpanBPeriod      int     `validate:"required" json:"ichimokuSpanBPeriod"`
	VolumeThreshold          float64 `validate:"required" json:"volumeThreshold"` // Threshold to consider volume "significant"
	FractalLookback          int     `validate:"required" json:"fractalLookback"` // Period used for Donchian or fractal analysis

	EMAPeriods []int `json:"emaPeriods" validate:"required"` // Periods for EMA calculation

	MFIPeriod     int     `json:"mfiPeriod" validate:"omitempty,gt=0"`
	MFIOverbought float64 `json:"mfiOverbought" validate:"omitempty,gt=0"`
	MFIOversold   float64 `json:"mfiOversold" validate:"omitempty,lt=100"`

	CCIPeriod     int     `json:"cciPeriod" validate:"omitempty,gt=0"`
	CCIOverbought float64 `json:"cciOverbought" validate:"omitempty,gt=0"`
	CCIOversold   float64 `json:"cciOversold" validate:"omitempty,lt=0"`

	MarketRegimePeriod   int     `json:"marketRegimePeriod" validate:"omitempty,gt=0"`    // Period for market regime detection
	VolatilityPeriod     int     `json:"volatilityPeriod" validate:"omitempty,gt=0"`      // Period for volatility calculation
	CorrelationPeriod    int     `json:"correlationPeriod" validate:"omitempty,gt=0"`     // Period for correlation analysis
	NoiseFilterThreshold float64 `json:"noiseFilterThreshold" validate:"omitempty,gte=0"` // Threshold for noise filtering
	ConfidenceThreshold  float64 `json:"confidenceThreshold" validate:"omitempty,gte=0"`  // Minimum confidence for signals
	AdaptiveLookback     int     `json:"adaptiveLookback" validate:"omitempty,gt=0"`      // Adaptive lookback period

	lastStates sync.Map // map[string]models.MarketState

	mfiAlgo *algos.MFIStrategy
	cciAlgo *algos.CCIStrategy
}

func NewMarketAnalyzer(analyzer MarketAnalyzer) *MarketAnalyzer {
	if err := validator.New().Struct(analyzer); err != nil {
		logger.Fatalf("Invalid MarketAnalyzer configuration: %v", err)
	}

	if analyzer.MarketRegimePeriod == 0 {
		analyzer.MarketRegimePeriod = 50
	}
	if analyzer.VolatilityPeriod == 0 {
		analyzer.VolatilityPeriod = 20
	}
	if analyzer.CorrelationPeriod == 0 {
		analyzer.CorrelationPeriod = 30
	}
	if analyzer.NoiseFilterThreshold == 0 {
		analyzer.NoiseFilterThreshold = 0.001
	}
	if analyzer.ConfidenceThreshold == 0 {
		analyzer.ConfidenceThreshold = 0.6
	}
	if analyzer.AdaptiveLookback == 0 {
		analyzer.AdaptiveLookback = 100
	}

	if analyzer.MFIPeriod != 0 && analyzer.MFIOverbought != 0 && analyzer.MFIOversold != 0 {
		logger.Infof("MFI Strategy enabled for MarketAnalysis with Period=%d, Overbought=%v, Oversold=%v",
			analyzer.MFIPeriod, analyzer.MFIOverbought, analyzer.MFIOversold)
		analyzer.mfiAlgo = &algos.MFIStrategy{
			Overbought: int(analyzer.MFIOverbought),
			Oversold:   int(analyzer.MFIOversold),
			Period:     analyzer.MFIPeriod,
		}
	}

	if analyzer.CCIPeriod != 0 && analyzer.CCIOverbought != 0 && analyzer.CCIOversold != 0 {
		logger.Infof("CCI Strategy enabled for MarketAnalysis with Period=%d, Overbought=%v, Oversold=%v",
			analyzer.CCIPeriod, analyzer.CCIOverbought, analyzer.CCIOversold)
		analyzer.cciAlgo = &algos.CCIStrategy{
			Period:     analyzer.CCIPeriod,
			Overbought: analyzer.CCIOverbought,
			Oversold:   analyzer.CCIOversold,
		}
	}

	sort.Ints(analyzer.EMAPeriods)
	return &analyzer
}

func (ma *MarketAnalyzer) AnalyzeMarket(symbol string, candles []models.CandleStick) (marketState models.MarketState, atr float64, adx float64) {
	if len(candles) < ma.ATRPeriod || len(candles) < ma.ADXPeriod {
		logger.Debugf("[WARN] Insufficient candles for analysis. Expected at least %d candles, got %d",
			ma.ATRPeriod, len(candles))
		return models.Default, 0, 0
	}

	atr = ma.calculateATR(candles)
	adx = ma.calculateADX(candles)
	upOK, upScore, upReason := ma.IsUptrendWithScore(candles)

	marketRegime := ma.detectMarketRegime(candles)
	volatilityRegime := ma.detectVolatilityRegime(candles)
	priceActionQuality := ma.assessPriceActionQuality(candles)
	noiseLevel := ma.calculateNoiseLevel(candles)
	adaptiveThresholds := ma.calculateAdaptiveThresholds(candles, atr)

	emaAlignment := ma.isEMABullishAlignment(candles)
	ichimokuSignal := ma.calculateIchimokuCloud(candles)
	volumeSignal := ma.analyzeVolumeWithContext(candles, marketRegime)
	fractalSignal := ma.detectFractalCharacteristics(candles)
	isRange := ma.isRangeBoundMarket(candles, atr)
	volumeProfile := ma.analyzeVolumeProfile(candles)
	trendTransition := ma.detectTrendTransitions(candles, adx)

	priceChangePct := ma.shortTermPriceChange(candles, 5)
	momentumQuality := ma.assessMomentumQuality(candles)

	multiTimeframeScore := ma.simulateMultiTimeframeAnalysis(candles)

	var mfiVal float64
	var mfiSignal int
	mfiEnabled := ma.mfiAlgo != nil
	if mfiEnabled {
		var errMfi error
		mfiVal, mfiSignal, errMfi = ma.mfiAlgo.Calculate(candles, "")
		if errMfi != nil {
			logger.Debugf("Error computing MFI: %v", errMfi)
		} else {
			logger.Debugf("MFI=%.2f, MFI-Signal=%d", mfiVal, mfiSignal)
		}
	}

	var cciVal float64
	var cciSignal int
	cciEnabled := ma.cciAlgo != nil
	if cciEnabled {
		var errCci error
		cciVal, cciSignal, errCci = ma.cciAlgo.Calculate(candles, "")
		if errCci != nil {
			logger.Debugf("Error computing CCI: %v", errCci)
		} else {
			logger.Debugf("CCI=%.2f, CCI-Signal=%d", cciVal, cciSignal)
		}
	}

	logger.Debugf(
		"[Market Analysis Enhanced %s] ATR=%.2f | ADX=%.2f | UptrendOK=%v (score=%.2f, reason=%s) | Regime=%v | VolRegime=%v | NoiseLevel=%.4f | Quality=%.2f",
		symbol, atr, adx, upOK, upScore, upReason, marketRegime, volatilityRegime, noiseLevel, priceActionQuality,
	)

	var trendingScore, chaoticScore, rangeScore, transitionalScore float64

	confidenceMultiplier := ma.calculateConfidenceMultiplier(priceActionQuality, noiseLevel, momentumQuality)

	if emaAlignment {
		trendingScore += 1.6 * confidenceMultiplier
	}
	trendingScore += upScore * 1.2 * confidenceMultiplier
	if adx > adaptiveThresholds.strongTrend {
		trendingScore += 1.0 * confidenceMultiplier
	}
	if ichimokuSignal == models.BullishIchi {
		trendingScore += 0.7 * confidenceMultiplier
	}
	if volumeSignal == models.Rising {
		trendingScore += 0.9 * confidenceMultiplier
	}
	if marketRegime == models.TrendingRegime {
		trendingScore += 1.2 * confidenceMultiplier
	}
	if multiTimeframeScore > 0.6 {
		trendingScore += 0.8 * confidenceMultiplier
	}
	if mfiEnabled && mfiVal < ma.MFIOverbought && mfiVal > ma.MFIOversold {
		trendingScore += 0.4 * confidenceMultiplier
	}
	if cciEnabled && cciVal < ma.CCIOverbought && cciVal > ma.CCIOversold {
		trendingScore += 0.4 * confidenceMultiplier
	}

	if atr > adaptiveThresholds.highVolatility {
		chaoticScore += 2.4 * confidenceMultiplier
	}
	if fractalSignal == models.FractalSignalBreakout {
		chaoticScore += 1.6 * confidenceMultiplier
	}
	if volumeProfile == models.UnbalancedProfile {
		chaoticScore += 1.1 * confidenceMultiplier
	}
	if volatilityRegime == models.HighVolatilityRegime {
		chaoticScore += 1.8 * confidenceMultiplier
	}
	if noiseLevel > ma.NoiseFilterThreshold*2 {
		chaoticScore += 1.3 * confidenceMultiplier
	}
	if priceActionQuality < 0.3 {
		chaoticScore += 0.9 * confidenceMultiplier
	}
	if mfiEnabled && (mfiVal > ma.MFIOverbought || mfiVal < ma.MFIOversold) {
		chaoticScore += 0.7 * confidenceMultiplier
	}
	if cciEnabled && (cciVal > ma.CCIOverbought || cciVal < ma.CCIOversold) {
		chaoticScore += 0.7 * confidenceMultiplier
	}

	if isRange {
		rangeScore += 2.3 * confidenceMultiplier
	}
	if volumeProfile == models.BalancedProfile {
		rangeScore += 1.6 * confidenceMultiplier
	}
	if adx < adaptiveThresholds.weakTrend {
		rangeScore += 1.1 * confidenceMultiplier
	}
	if marketRegime == models.RangeBoundRegime {
		rangeScore += 1.8 * confidenceMultiplier
	}
	if volatilityRegime == models.LowVolatilityRegime {
		rangeScore += 1.3 * confidenceMultiplier
	}
	if cciEnabled && math.Abs(cciVal) < 50 {
		rangeScore += 0.7 * confidenceMultiplier
	}
	if mfiEnabled && mfiVal >= ma.MFIOversold && mfiVal <= ma.MFIOverbought {
		rangeScore += 0.7 * confidenceMultiplier
	}

	if trendTransition != models.Default {
		transitionalScore += 3.2 * confidenceMultiplier
	}
	if ma.isDivergencePresent(candles) {
		transitionalScore += 2.3 * confidenceMultiplier
	}
	if ma.isTrendEmerging(candles, adx) {
		transitionalScore += 1.8 * confidenceMultiplier
	}
	if ma.detectRegimeChange(candles) {
		transitionalScore += 1.8 * confidenceMultiplier
	}

	switch {
	case priceChangePct < -5.0:
		chaoticScore += 5.5 * confidenceMultiplier
		logger.Debugf("[%s] Major price drop (%.2f%%). Boost chaoticScore.", symbol, priceChangePct)
	case priceChangePct < -3.0:
		chaoticScore += 3.5 * confidenceMultiplier
		logger.Debugf("[%s] Significant price drop (%.2f%%). Boost chaoticScore.", symbol, priceChangePct)
	case priceChangePct > 7.0:
		trendingScore += 5.5 * confidenceMultiplier
		logger.Debugf("[%s] Major price pump (%.2f%%). Boost trendingScore.", symbol, priceChangePct)
	case priceChangePct > 5.0:
		trendingScore += 3.5 * confidenceMultiplier
		logger.Debugf("[%s] Significant price pump (%.2f%%). Boost trendingScore.", symbol, priceChangePct)
	}

	if upReason == "near_top" || upReason == "overextended" || upReason == "exhaustion" {
		d := math.Min(1.5*confidenceMultiplier, trendingScore*0.35)
		trendingScore -= d
		rangeScore += d * 0.6
		chaoticScore += d * 0.4
	}

	logger.Debugf(
		"[Enhanced State Scores %s] Trending=%.2f | Chaotic=%.2f | RangeBound=%.2f | Transitional=%.2f | Confidence=%.2f",
		symbol, trendingScore, chaoticScore, rangeScore, transitionalScore, confidenceMultiplier,
	)

	scores := map[models.MarketState]float64{
		models.StronglyTrending: trendingScore * 1.15,
		models.Trending:         trendingScore,
		models.Chaotic:          chaoticScore,
		models.RangeBound:       rangeScore,
		models.Transitional:     transitionalScore,
	}

	var maxState models.MarketState = models.Default
	maxScore := -1.0
	for state, score := range scores {
		if score > maxScore && score >= ma.ConfidenceThreshold {
			maxScore = score
			maxState = state
		}
	}
	if maxScore < ma.ConfidenceThreshold {
		maxState = models.Default
		for state, score := range scores {
			if score > maxScore {
				maxScore = score
				maxState = state
			}
		}
		logger.Debugf("[%s] Low confidence detection (%.2f < %.2f), fallback: %s",
			symbol, maxScore, ma.ConfidenceThreshold, maxState.String())
	}

	finalState := maxState

	strongTrendingThreshold := 5.0
	if volatilityRegime == models.HighVolatilityRegime {
		strongTrendingThreshold = 6.0
	} else if volatilityRegime == models.LowVolatilityRegime {
		strongTrendingThreshold = 5.0
	}

	if finalState == models.Trending && trendingScore >= strongTrendingThreshold {
		finalState = models.StronglyTrending
	}

	adxThreshold := 30.0
	if marketRegime == models.MarketHighVolatilityRegime {
		adxThreshold = 35.0
	} else if marketRegime == models.MarketLowVolatilityRegime {
		adxThreshold = 25.0
	}
	if finalState == models.StronglyTrending && adx < adxThreshold {
		finalState = models.Trending
	}

	if transitionalScore > 5.0 && maxScore-transitionalScore < 2.0 {
		finalState = models.Transitional
	}

	if noiseLevel > ma.NoiseFilterThreshold*3 && ma.NoiseFilterThreshold > 0 {
		if finalState == models.StronglyTrending {
			finalState = models.Trending
		} else if finalState == models.Trending && rangeScore > trendingScore*0.7 {
			finalState = models.RangeBound
		}
	}

	var prevState models.MarketState
	if v, ok := ma.lastStates.Load(symbol); ok {
		if ps, ok2 := v.(models.MarketState); ok2 {
			prevState = ps
		}
	}

	if prevState != models.Default && finalState != prevState {
		lastScore := scores[prevState]
		margin := maxScore - lastScore
		if margin < 0.6 {
			logger.Debugf("[Hysteresis %s] Keeping previous state %s (margin=%.2f insufficient)",
				symbol, prevState.String(), margin)
			finalState = prevState
		}
	}
	ma.lastStates.Store(symbol, finalState)

	logger.Debugf(
		"[Final Enhanced Market State %s] %s (Score=%.2f, Confidence=%.2f, Noise=%.4f)",
		symbol, finalState.String(), maxScore, confidenceMultiplier, noiseLevel,
	)

	return finalState, atr, adx
}

func (ma *MarketAnalyzer) detectMarketRegime(candles []models.CandleStick) models.MarketRegime {
	if len(candles) < ma.MarketRegimePeriod {
		return models.UnknownRegime
	}

	volatility := ma.calculateRollingVolatility(candles, ma.MarketRegimePeriod)
	trendStrength := ma.calculateTrendStrength(candles, ma.MarketRegimePeriod)
	priceEfficiency := ma.calculatePriceEfficiency(candles, ma.MarketRegimePeriod)

	if volatility > ma.HighVolatilityThreshold*1.5 {
		return models.MarketHighVolatilityRegime
	} else if trendStrength > 0.7 && priceEfficiency > 0.6 {
		return models.TrendingRegime
	} else if volatility < ma.HighVolatilityThreshold*0.5 && trendStrength < 0.3 {
		return models.RangeBoundRegime
	} else if volatility < ma.HighVolatilityThreshold*0.3 {
		return models.MarketLowVolatilityRegime
	}

	return models.MarketMixedRegime
}

func (ma *MarketAnalyzer) detectVolatilityRegime(candles []models.CandleStick) models.VolatilityRegime {
	if len(candles) < ma.VolatilityPeriod {
		return models.NormalVolatilityRegime
	}

	recentVol := ma.calculateRollingVolatility(candles, ma.VolatilityPeriod/2)
	historicalVol := ma.calculateRollingVolatility(candles, ma.VolatilityPeriod)

	if historicalVol <= 0 {
		return models.NormalVolatilityRegime
	}

	volRatio := recentVol / historicalVol

	if volRatio > 1.5 {
		return models.HighVolatilityRegime
	} else if volRatio < 0.7 {
		return models.LowVolatilityRegime
	}

	return models.NormalVolatilityRegime
}

func (ma *MarketAnalyzer) assessPriceActionQuality(candles []models.CandleStick) float64 {
	if len(candles) < 20 {
		return 0.5
	}

	trendConsistency := ma.calculateTrendConsistency(candles)
	volumeConfirmation := ma.calculateVolumeConfirmation(candles)
	priceStructure := ma.assessPriceStructure(candles)
	breakoutQuality := ma.assessBreakoutQuality(candles)

	quality := trendConsistency*0.3 + volumeConfirmation*0.25 + priceStructure*0.25 + breakoutQuality*0.2

	return math.Min(1.0, math.Max(0.0, quality))
}

func (ma *MarketAnalyzer) calculateATR(candles []models.CandleStick) float64 {
	if ma.ATRPeriod <= 0 || len(candles) < ma.ATRPeriod+1 {
		return 0
	}

	if v, err := algos.ATR(candles, ma.ATRPeriod); err == nil {
		return v
	}

	trueRanges := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		h := candles[i].High
		l := candles[i].Low
		pc := candles[i-1].Close
		tr := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		trueRanges = append(trueRanges, tr)
	}
	if len(trueRanges) < ma.ATRPeriod {
		return 0
	}
	return ma.average(trueRanges[len(trueRanges)-ma.ATRPeriod:])
}

func (ma *MarketAnalyzer) calculateADX(candles []models.CandleStick) float64 {
	p := ma.ADXPeriod
	if p <= 0 || len(candles) < p+1 {
		return 0
	}

	tr := make([]float64, len(candles)-1)
	pdm := make([]float64, len(candles)-1)
	mdm := make([]float64, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		h, l := candles[i].High, candles[i].Low
		ph, pl := candles[i-1].High, candles[i-1].Low
		pc := candles[i-1].Close

		tr[i-1] = math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		upMove := h - ph
		downMove := pl - l
		if upMove > downMove && upMove > 0 {
			pdm[i-1] = upMove
			mdm[i-1] = 0
		} else if downMove > 0 {
			pdm[i-1] = 0
			mdm[i-1] = downMove
		}
	}

	if len(tr) < p {
		return 0
	}

	tr14 := 0.0
	pdm14 := 0.0
	mdm14 := 0.0
	for i := 0; i < p; i++ {
		tr14 += tr[i]
		pdm14 += pdm[i]
		mdm14 += mdm[i]
	}

	diPlus := make([]float64, 0, len(tr)-p+1)
	diMinus := make([]float64, 0, len(tr)-p+1)
	dxSeries := make([]float64, 0, len(tr)-p+1)

	for i := p; i < len(tr); i++ {
		tr14 = tr14 - tr14/float64(p) + tr[i]
		pdm14 = pdm14 - pdm14/float64(p) + pdm[i]
		mdm14 = mdm14 - mdm14/float64(p) + mdm[i]

		if tr14 <= 0 {
			continue
		}
		plusDI := 100.0 * (pdm14 / tr14)
		minusDI := 100.0 * (mdm14 / tr14)
		diPlus = append(diPlus, plusDI)
		diMinus = append(diMinus, minusDI)
		den := plusDI + minusDI
		if den <= 0 {
			continue
		}
		dx := 100.0 * math.Abs(plusDI-minusDI) / den
		dxSeries = append(dxSeries, dx)
	}

	if len(dxSeries) == 0 {
		return 0
	}
	if len(dxSeries) < p {
		return ma.average(dxSeries)
	}

	adx := 0.0
	for i := 0; i < p; i++ {
		adx += dxSeries[i]
	}
	adx /= float64(p)
	for i := p; i < len(dxSeries); i++ {
		adx = (adx*float64(p-1) + dxSeries[i]) / float64(p)
	}
	if math.IsNaN(adx) || math.IsInf(adx, 0) {
		return 0
	}
	if adx < 0 {
		adx = 0
	} else if adx > 100 {
		adx = 100
	}
	return adx
}

func (ma *MarketAnalyzer) calculateNoiseLevel(candles []models.CandleStick) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var totalNoise float64
	for i := 1; i < len(candles); i++ {
		priceChange := math.Abs(candles[i].Close - candles[i-1].Close)
		trueRange := math.Max(
			candles[i].High-candles[i].Low,
			math.Max(
				math.Abs(candles[i].High-candles[i-1].Close),
				math.Abs(candles[i].Low-candles[i-1].Close),
			),
		)

		if trueRange > 0 {
			noise := 1.0 - (priceChange / trueRange)
			totalNoise += noise
		}
	}

	return totalNoise / float64(len(candles)-1)
}

type AdaptiveThresholds struct {
	highVolatility float64
	strongTrend    float64
	weakTrend      float64
}

func (ma *MarketAnalyzer) calculateAdaptiveThresholds(candles []models.CandleStick, _ float64) AdaptiveThresholds {
	volatility := ma.calculateRollingVolatility(candles, 20)

	highVolThreshold := ma.HighVolatilityThreshold
	if highVolThreshold <= 0 {
		highVolThreshold = 0.01
	}

	strongTrendThreshold := ma.StrongTrendThreshold
	if strongTrendThreshold <= 0 {
		strongTrendThreshold = 20.0
	}

	volMultiplier := volatility / highVolThreshold

	factor := math.Max(0.5, math.Min(2.0, volMultiplier))

	return AdaptiveThresholds{
		highVolatility: highVolThreshold * factor,
		strongTrend:    strongTrendThreshold * math.Max(0.8, math.Min(1.5, volMultiplier)),
		weakTrend:      strongTrendThreshold * 0.6 * math.Max(0.5, math.Min(1.2, volMultiplier)),
	}
}

func (ma *MarketAnalyzer) calculateConfidenceMultiplier(priceActionQuality, noiseLevel, momentumQuality float64) float64 {
	confidence := 1.0

	confidence *= 0.5 + priceActionQuality*0.5

	noiseDen := ma.NoiseFilterThreshold
	if noiseDen <= 0 {
		noiseDen = 0.001
	}
	noiseAdjustment := math.Max(0.5, 1.0-(noiseLevel/noiseDen))
	confidence *= noiseAdjustment

	confidence *= 0.7 + momentumQuality*0.3

	return math.Max(0.3, math.Min(1.5, confidence))
}

func (ma *MarketAnalyzer) analyzeVolumeWithContext(candles []models.CandleStick, regime models.MarketRegime) models.VolumeSignal {
	if len(candles) < 25 {
		return models.Neutral
	}
	recent := ma.averageVolume(candles[len(candles)-5:])
	hist := ma.averageVolume(candles[len(candles)-20:])
	if hist <= 0 {
		return models.Neutral
	}
	vr := recent / hist

	switch regime {
	case models.TrendingRegime:
		if vr >= 1.25 {
			return models.Rising
		} else if vr <= 0.75 {
			return models.Declining
		}
	case models.RangeBoundRegime:
		if vr >= 1.4 {
			return models.Rising
		} else if vr <= 0.7 {
			return models.Declining
		}
	case models.MarketHighVolatilityRegime:
		if vr >= 1.2 {
			return models.Rising
		} else if vr <= 0.8 {
			return models.Declining
		}
	default:
		if vr >= 1.2 {
			return models.Rising
		} else if vr <= 0.8 {
			return models.Declining
		}
	}
	return models.Neutral
}

func (ma *MarketAnalyzer) IsUptrendWithScore(candles []models.CandleStick) (bool, float64, string) {
	if len(candles) < 30 {
		return false, 0.0, "insufficient_data"
	}

	ema9 := ma.calculateEMA(candles, 9)
	ema21 := ma.calculateEMA(candles, 21)
	if len(ema9) < 3 || len(ema21) < 3 {
		return false, 0.0, "insufficient_ema"
	}
	ema9Now, ema9Prev := ema9[len(ema9)-1], ema9[len(ema9)-2]
	ema21Now, ema21Prev := ema21[len(ema21)-1], ema21[len(ema21)-2]
	priceNow := candles[len(candles)-1].Close

	dirOK := ema9Now > ema21Now && ema9Now > ema9Prev && ema21Now > ema21Prev && priceNow > ema21Now

	rsi := algos.RSIStrategy{Period: 14}
	rsiVal, _, errRSI := rsi.Calculate(candles, "")
	if errRSI != nil {
		return false, 0.0, "rsi_error"
	}
	var prevRSI float64
	if val, _, _ := rsi.Calculate(candles[:len(candles)-1], ""); val != 0 {
		prevRSI = val
	}
	rsiSlope := rsiVal - prevRSI
	rsiOK := rsiVal >= 52 && rsiVal <= 66 && rsiSlope >= 0

	macd := algos.MACDStrategy{FastPeriod: 12, SlowPeriod: 26, SignalPeriod: 9}
	hist, sigLine, macdLine, macdSig, errMACD := macd.Calculate(candles)
	if errMACD != nil {
		return false, 0.0, "macd_error"
	}
	prevHist := 0.0
	if ph, _, _, _, e := macd.Calculate(candles[:len(candles)-1]); e == nil {
		prevHist = ph
	}
	histSlope := hist - prevHist
	macdOK := macdSig == 1 && hist > 0 && macdLine > sigLine && histSlope > 0

	adxNow := ma.calculateADX(candles)
	adxPrev := ma.calculateADX(candles[:len(candles)-1])
	adxOK := adxNow >= 23 && (adxPrev == 0 || adxNow >= adxPrev-0.5)

	vol5 := ma.averageVolume(candles[len(candles)-5:])
	vol20 := ma.averageVolume(candles[len(candles)-20:])
	volOK := true
	if vol5 > 0 && vol20 > 0 {
		volOK = (vol5 / vol20) >= 1.10
	}

	bb := algos.BollingerBands{Period: 20, Width: 2.0}
	lower, middle, upper, bbErr := bb.Calculate(candles)
	overextended := false
	if bbErr == nil && middle > 0 {
		bandWidth := upper - lower

		if priceNow > upper*1.01 || (priceNow-upper) > 0.10*bandWidth {
			overextended = true
		}
	}

	emaDist := 0.0
	if ema21Now > 0 {
		emaDist = (priceNow - ema21Now) / ema21Now
	}
	if emaDist > 0.03 {
		overextended = true
	}

	atrAbs := ma.calculateATR(candles)
	oneBarRange := math.Abs(candles[len(candles)-1].Close - candles[len(candles)-2].Close)
	blowoff := atrAbs > 0 && oneBarRange > 1.6*atrAbs

	exhaustion := false
	if len(candles) >= 2 && vol20 > 0 {
		c := candles[len(candles)-1]
		prev := candles[len(candles)-2]
		upperWick := c.High - math.Max(c.Close, c.Open)
		total := c.High - c.Low
		upperWickRatio := 0.0
		if total > 0 {
			upperWickRatio = upperWick / total
		}

		if c.Close > prev.Close && upperWickRatio > 0.55 && c.Volume > 1.8*vol20 {
			exhaustion = true
		}
	}

	shortPump := ma.shortTermPriceChange(candles, 3) > 3.5

	distTop := false
	if ma.mfiAlgo != nil {
		if mv, _, e := ma.mfiAlgo.Calculate(candles, ""); e == nil && mv > 80 && rsiSlope <= 0 {
			distTop = true
		}
	}
	if ma.cciAlgo != nil {
		if cv, _, e := ma.cciAlgo.Calculate(candles, ""); e == nil && cv > 150 && rsiSlope <= 0 {
			distTop = true
		}
	}

	score := 0.0
	if dirOK {
		score += 0.38
	}
	if macdOK {
		score += 0.24
	}
	if rsiOK {
		score += 0.14
	}
	if volOK {
		score += 0.10
	}
	if adxOK {
		score += 0.10
	}
	if bbErr == nil && middle > 0 && priceNow <= upper && priceNow <= middle+(upper-lower)*0.10 {
		score += 0.04
	}

	if overextended {
		return false, math.Max(0, score-0.25), "overextended"
	}
	if blowoff {
		return false, math.Max(0, score-0.25), "blowoff"
	}
	if exhaustion {
		return false, math.Max(0, score-0.20), "exhaustion"
	}
	if shortPump {
		return false, math.Max(0, score-0.20), "near_top"
	}
	if distTop {
		return false, math.Max(0, score-0.20), "near_top"
	}
	if ma.isNearTop(candles) {
		return false, math.Max(0, score-0.20), "near_top"
	}

	ok := score >= 0.72
	reason := "ok"
	if !ok {
		switch {
		case !dirOK:
			reason = "ema_direction_weak"
		case !macdOK:
			reason = "macd_weak"
		case !rsiOK:
			reason = "rsi_out_of_band_or_falling"
		case !volOK:
			reason = "volume_not_confirming"
		case !adxOK:
			reason = "adx_low_or_falling"
		}
	}
	return ok, math.Max(0, math.Min(1, score)), reason
}

func (ma *MarketAnalyzer) IsUptrend(candles []models.CandleStick) bool {
	ok, _, _ := ma.IsUptrendWithScore(candles)
	return ok
}

func (ma *MarketAnalyzer) isNearTop(candles []models.CandleStick) bool {
	rsi := algos.RSIStrategy{Period: 14}
	rsiVals := make([]float64, len(candles))
	for i := range candles {
		val, _, _ := rsi.Calculate(candles[:i+1], "")
		rsiVals[i] = val
	}

	if ma.hasBearishDivergence(candles, rsiVals) {
		return true
	}
	if ma.detectPriceTopPatterns(candles) {
		return true
	}
	return false
}

func (ma *MarketAnalyzer) hasBearishDivergence(candles []models.CandleStick, rsiVals []float64) bool {
	pricePeaks := ma.detectPeaks(candles, true)
	rsiPeaks := ma.detectPeaksFloat(rsiVals, true)

	if len(pricePeaks) < 2 || len(rsiPeaks) < 2 {
		return false
	}

	lastPrice := candles[pricePeaks[len(pricePeaks)-1]].Close
	prevPrice := candles[pricePeaks[len(pricePeaks)-2]].Close
	lastRSI := rsiVals[rsiPeaks[len(rsiPeaks)-1]]
	prevRSI := rsiVals[rsiPeaks[len(rsiPeaks)-2]]

	return lastPrice > prevPrice && lastRSI < prevRSI
}

func (ma *MarketAnalyzer) detectPriceTopPatterns(candles []models.CandleStick) bool {
	if len(candles) < 3 {
		return false
	}

	recentHighs := []float64{candles[len(candles)-1].High}
	if len(candles) >= 2 {
		recentHighs = append(recentHighs, candles[len(candles)-2].High)
	}
	if len(candles) >= 3 {
		recentHighs = append(recentHighs, candles[len(candles)-3].High)
	}
	if len(recentHighs) < 3 {
		return false
	}

	h0, h1, h2 := recentHighs[0], recentHighs[1], recentHighs[2]
	return math.Abs(h0-h1)/math.Max(h0, h1) < 0.01 &&
		math.Abs(h1-h2)/math.Max(h1, h2) < 0.01
}

func (ma *MarketAnalyzer) assessMomentumQuality(candles []models.CandleStick) float64 {
	if len(candles) < 20 {
		return 0.5
	}

	momentumPersistence := ma.calculateMomentumPersistence(candles, 10)
	momentumAcceleration := ma.calculateMomentumAcceleration(candles, 5)
	momentumVolumeConfirmation := ma.calculateMomentumVolumeConfirmation(candles, 10)

	quality := momentumPersistence*0.4 + momentumAcceleration*0.3 + momentumVolumeConfirmation*0.3
	return math.Min(1.0, math.Max(0.0, quality))
}

func (ma *MarketAnalyzer) simulateMultiTimeframeAnalysis(candles []models.CandleStick) float64 {
	if len(candles) < 100 {
		return 0.5
	}

	var higherTFCandles []models.CandleStick
	for i := 0; i < len(candles); i += 4 {
		if i+3 < len(candles) {
			high := candles[i].High
			low := candles[i].Low
			open := candles[i].Open
			cls := candles[i+3].Close
			volume := 0.0

			for j := 0; j < 4 && i+j < len(candles); j++ {
				if candles[i+j].High > high {
					high = candles[i+j].High
				}
				if candles[i+j].Low < low {
					low = candles[i+j].Low
				}
				volume += candles[i+j].Volume
			}

			higherTFCandles = append(higherTFCandles, models.CandleStick{
				Open:   open,
				High:   high,
				Low:    low,
				Close:  cls,
				Volume: volume,
			})
		}
	}

	if len(higherTFCandles) < 20 {
		return 0.5
	}

	higherTFTrend := ma.IsUptrend(higherTFCandles)
	currentTFTrend := ma.IsUptrend(candles)

	if higherTFTrend == currentTFTrend {
		return 0.8
	}
	return 0.3
}

func (ma *MarketAnalyzer) detectRegimeChange(candles []models.CandleStick) bool {
	if len(candles) < ma.MarketRegimePeriod*2 {
		return false
	}

	recentPeriod := ma.MarketRegimePeriod / 2
	recentCandles := candles[len(candles)-recentPeriod:]
	historicalCandles := candles[len(candles)-ma.MarketRegimePeriod : len(candles)-recentPeriod]

	recentRegime := ma.detectMarketRegime(recentCandles)
	historicalRegime := ma.detectMarketRegime(historicalCandles)

	if recentRegime != historicalRegime && recentRegime != models.UnknownRegime && historicalRegime != models.UnknownRegime {
		logger.Debugf("Market regime change detected: %v -> %v", historicalRegime, recentRegime)
		return true
	}

	recentVol := ma.calculateRollingVolatility(recentCandles, len(recentCandles))
	historicalVol := ma.calculateRollingVolatility(historicalCandles, len(historicalCandles))

	if historicalVol <= 0 {
		return false
	}
	volChangeRatio := recentVol / historicalVol
	if volChangeRatio > 2.0 || volChangeRatio < 0.5 {
		logger.Debugf("Volatility regime change detected: ratio=%.2f", volChangeRatio)
		return true
	}

	return false
}

func (ma *MarketAnalyzer) shortTermPriceChange(candles []models.CandleStick, lookback int) float64 {
	if len(candles) < lookback+1 {
		return 0
	}

	end := len(candles) - 1
	start := end - lookback

	oldPrice := candles[start].Close
	newPrice := candles[end].Close

	if oldPrice == 0 {
		return 0
	}

	return (newPrice - oldPrice) / oldPrice * 100.0
}

func (ma *MarketAnalyzer) calculateRollingVolatility(candles []models.CandleStick, period int) float64 {
	if len(candles) < period || period < 2 {
		return 0.0
	}

	startIdx := max(len(candles)-period, 0)

	var returns []float64
	for i := startIdx + 1; i < len(candles); i++ {
		if candles[i-1].Close > 0 {
			ret := math.Log(candles[i].Close / candles[i-1].Close)
			returns = append(returns, ret)
		}
	}

	if len(returns) < 2 {
		return 0.0
	}

	mean := 0.0
	for _, ret := range returns {
		mean += ret
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, ret := range returns {
		diff := ret - mean
		variance += diff * diff
	}
	variance /= float64(len(returns) - 1)

	return math.Sqrt(variance) * math.Sqrt(24*365) // Annualized volatility
}

func (ma *MarketAnalyzer) calculateTrendStrength(candles []models.CandleStick, period int) float64 {
	if len(candles) < period {
		return 0.0
	}

	startIdx := max(len(candles)-period, 0)

	startPrice := candles[startIdx].Close
	endPrice := candles[len(candles)-1].Close

	if startPrice == 0 {
		return 0.0
	}

	totalReturn := (endPrice - startPrice) / startPrice

	totalDistance := 0.0
	for i := startIdx + 1; i < len(candles); i++ {
		totalDistance += math.Abs(candles[i].Close - candles[i-1].Close)
	}

	directDistance := math.Abs(endPrice - startPrice)

	if totalDistance == 0 {
		return 0.0
	}

	efficiency := directDistance / totalDistance
	return math.Abs(totalReturn) * efficiency
}

func (ma *MarketAnalyzer) calculatePriceEfficiency(candles []models.CandleStick, period int) float64 {
	if len(candles) < period {
		return 0.0
	}

	startIdx := max(len(candles)-period, 0)

	startPrice := candles[startIdx].Close
	endPrice := candles[len(candles)-1].Close
	directDistance := math.Abs(endPrice - startPrice)

	actualDistance := 0.0
	for i := startIdx + 1; i < len(candles); i++ {
		actualDistance += math.Abs(candles[i].Close - candles[i-1].Close)
	}

	if actualDistance == 0 {
		return 1.0
	}

	return directDistance / actualDistance
}

func (ma *MarketAnalyzer) calculateTrendConsistency(candles []models.CandleStick) float64 {
	if len(candles) < 10 {
		return 0.5
	}

	upMoves := 0
	downMoves := 0
	totalMoves := 0

	for i := 1; i < len(candles); i++ {
		if candles[i].Close > candles[i-1].Close {
			upMoves++
		} else if candles[i].Close < candles[i-1].Close {
			downMoves++
		}
		totalMoves++
	}

	if totalMoves == 0 {
		return 0.5
	}

	majorityMoves := math.Max(float64(upMoves), float64(downMoves))
	return majorityMoves / float64(totalMoves)
}

func (ma *MarketAnalyzer) calculateVolumeConfirmation(candles []models.CandleStick) float64 {
	if len(candles) < 10 {
		return 0.5
	}

	confirmations := 0
	total := 0

	avgVolume := ma.averageVolume(candles)

	for i := 1; i < len(candles); i++ {
		priceUp := candles[i].Close > candles[i-1].Close
		highVolume := candles[i].Volume > avgVolume

		if (priceUp && highVolume) || (!priceUp && !highVolume) {
			confirmations++
		}
		total++
	}

	if total == 0 {
		return 0.5
	}

	return float64(confirmations) / float64(total)
}

func (ma *MarketAnalyzer) isEMABullishAlignment(candles []models.CandleStick) bool {
	if len(ma.EMAPeriods) < 3 {
		logger.Error("Need at least 3 EMA periods for ribbon analysis")
		return false
	}

	var emas [][]float64
	for _, period := range ma.EMAPeriods {
		ema := ma.calculateEMA(candles, period)
		if len(ema) < 1 {
			return false
		}
		emas = append(emas, ema)
	}

	for i := 0; i < len(emas)-1; i++ {
		currentEMA := emas[i][len(emas[i])-1]
		nextEMA := emas[i+1][len(emas[i+1])-1]
		if currentEMA <= nextEMA {
			return false
		}
	}
	return true
}

func (ma *MarketAnalyzer) isRangeBoundMarket(candles []models.CandleStick, atr float64) bool {
	if len(candles) < 25 {
		return false
	}
	currentPrice := candles[len(candles)-1].Close
	if currentPrice <= 0 {
		return false
	}
	atrRatio := (atr / currentPrice) * 100

	bb := algos.BollingerBands{Period: 20, Width: 2.0}
	lower, middle, upper, err := bb.Calculate(candles)
	if err != nil || middle <= 0 {
		return false
	}
	bandWidth := (upper - lower) / middle

	oscScore := ma.priceOscillationScore(candles, 20)

	adxNow := ma.calculateADX(candles)
	thr := ma.calculateAdaptiveThresholds(candles, atr)

	return atrRatio < 1.5 && bandWidth < 0.10 && oscScore >= 0.6 && adxNow < thr.weakTrend
}

func (ma *MarketAnalyzer) priceOscillationScore(candles []models.CandleStick, period int) float64 {
	if period <= 0 || len(candles) < period {
		return 0
	}

	window := candles[len(candles)-period:]
	bb := algos.BollingerBands{Period: period, Width: 2.0}
	lower, middle, upper, err := bb.Calculate(window)
	if err != nil || middle == 0 {
		return 0
	}

	touchesUpper, touchesLower := 0, 0
	for _, c := range window {
		if c.High >= upper*0.98 {
			touchesUpper++
		}
		if c.Low <= lower*1.02 {
			touchesLower++
		}
	}
	return float64(touchesUpper+touchesLower) / float64(period*2)
}

func (ma *MarketAnalyzer) detectTrendTransitions(candles []models.CandleStick, adx float64) models.MarketState {
	if ma.isDivergencePresent(candles) {
		return models.Transitional
	}
	if ma.isTrendEmerging(candles, adx) {
		return models.Transitional
	}
	return models.Default
}

func (ma *MarketAnalyzer) assessPriceStructure(candles []models.CandleStick) float64 {
	if len(candles) < 20 {
		return 0.5
	}

	highs := ma.detectPeaks(candles, true)
	lows := ma.detectPeaks(candles, false)

	if len(highs) < 2 || len(lows) < 2 {
		return 0.5
	}

	higherHighs := 0
	lowerLows := 0
	totalHighs := len(highs) - 1
	totalLows := len(lows) - 1

	for i := 1; i < len(highs); i++ {
		if candles[highs[i]].High > candles[highs[i-1]].High {
			higherHighs++
		}
	}

	for i := 1; i < len(lows); i++ {

		if candles[lows[i]].Low < candles[lows[i-1]].Low {
			lowerLows++
		}
	}

	structureScore := 0.0
	if totalHighs > 0 && totalLows > 0 {
		highsScore := float64(higherHighs) / float64(totalHighs)
		lowsScore := float64(lowerLows) / float64(totalLows)
		structureScore = (highsScore + lowsScore) / 2.0
	}

	return structureScore
}

func (ma *MarketAnalyzer) assessBreakoutQuality(candles []models.CandleStick) float64 {
	if len(candles) < 20 {
		return 0.5
	}

	bb := algos.BollingerBands{Period: 20, Width: 2.0}
	lower, middle, upper, err := bb.Calculate(candles)
	if err != nil {
		return 0.5
	}

	recentPrice := candles[len(candles)-1].Close
	prevPrice := candles[len(candles)-2].Close

	breakoutQuality := 0.5

	if recentPrice > upper && prevPrice <= upper {
		followThrough := (recentPrice - upper) / (upper - middle)
		breakoutQuality = math.Min(1.0, 0.5+followThrough)
	} else if recentPrice < lower && prevPrice >= lower {
		followThrough := (lower - recentPrice) / (middle - lower)
		breakoutQuality = math.Min(1.0, 0.5+followThrough)
	}

	return breakoutQuality
}

func (ma *MarketAnalyzer) calculateMomentumPersistence(candles []models.CandleStick, period int) float64 {
	if len(candles) < period+1 {
		return 0.5
	}

	consistentMoves := 0
	totalMoves := 0

	for i := len(candles) - period; i < len(candles)-1; i++ {
		currentMove := candles[i+1].Close - candles[i].Close
		if i > 0 {
			prevMove := candles[i].Close - candles[i-1].Close
			if (currentMove > 0 && prevMove > 0) || (currentMove < 0 && prevMove < 0) {
				consistentMoves++
			}
		}
		totalMoves++
	}

	if totalMoves == 0 {
		return 0.5
	}

	return float64(consistentMoves) / float64(totalMoves)
}

func (ma *MarketAnalyzer) calculateMomentumAcceleration(candles []models.CandleStick, period int) float64 {
	if len(candles) < period+2 {
		return 0.5
	}

	recentMomentum := 0.0
	earlierMomentum := 0.0

	halfPeriod := period / 2
	if halfPeriod == 0 {
		return 0.5
	}

	for i := len(candles) - halfPeriod; i < len(candles)-1; i++ {
		recentMomentum += math.Abs(candles[i+1].Close - candles[i].Close)
	}
	recentMomentum /= float64(halfPeriod)

	for i := len(candles) - period; i < len(candles)-halfPeriod-1; i++ {
		earlierMomentum += math.Abs(candles[i+1].Close - candles[i].Close)
	}
	earlierMomentum /= float64(halfPeriod)

	if earlierMomentum == 0 {
		return 0.5
	}

	accelerationRatio := recentMomentum / earlierMomentum
	return math.Min(1.0, math.Max(0.0, (accelerationRatio-0.5)*2))
}

func (ma *MarketAnalyzer) calculateMomentumVolumeConfirmation(candles []models.CandleStick, period int) float64 {
	if len(candles) < period {
		return 0.5
	}

	confirmations := 0
	total := 0
	avgVolume := ma.averageVolume(candles[len(candles)-period:])

	for i := len(candles) - period + 1; i < len(candles); i++ {
		priceMove := math.Abs(candles[i].Close - candles[i-1].Close)
		volumeRatio := candles[i].Volume / avgVolume

		if priceMove > 0 {
			expectedVolumeRatio := 1.0 + priceMove*10
			if volumeRatio >= expectedVolumeRatio*0.8 {
				confirmations++
			}
		}
		total++
	}

	if total == 0 {
		return 0.5
	}

	return float64(confirmations) / float64(total)
}

func (ma *MarketAnalyzer) detectPeaksFloat(values []float64, findHighs bool) []int {
	var peaks []int
	minDistance := 2

	for i := 1; i < len(values)-1; i++ {
		if findHighs {
			if values[i] > values[i-1] && values[i] > values[i+1] {
				if len(peaks) == 0 || i-peaks[len(peaks)-1] >= minDistance {
					peaks = append(peaks, i)
				}
			}
		} else {
			if values[i] < values[i-1] && values[i] < values[i+1] {
				if len(peaks) == 0 || i-peaks[len(peaks)-1] >= minDistance {
					peaks = append(peaks, i)
				}
			}
		}
	}

	return peaks
}

func (ma *MarketAnalyzer) isDivergencePresent(candles []models.CandleStick) bool {
	if len(candles) < 20 {
		return false
	}

	rsi := algos.RSIStrategy{Period: 14}
	var rsiValues []float64

	for i := 14; i < len(candles); i++ {
		val, _, _ := rsi.Calculate(candles[:i+1], "")
		rsiValues = append(rsiValues, val)
	}

	if len(rsiValues) < 10 {
		return false
	}

	recentCandles := candles[len(candles)-10:]
	recentRSI := rsiValues[len(rsiValues)-10:]

	pricePeaks := ma.detectPeaks(recentCandles, true)
	rsiPeaks := ma.detectPeaksFloat(recentRSI, true)

	if len(pricePeaks) >= 2 && len(rsiPeaks) >= 2 {
		lastPriceHigh := recentCandles[pricePeaks[len(pricePeaks)-1]].High
		prevPriceHigh := recentCandles[pricePeaks[len(pricePeaks)-2]].High
		lastRSIHigh := recentRSI[rsiPeaks[len(rsiPeaks)-1]]
		prevRSIHigh := recentRSI[rsiPeaks[len(rsiPeaks)-2]]

		if lastPriceHigh > prevPriceHigh && lastRSIHigh < prevRSIHigh {
			return true
		}
	}

	return false
}

func (ma *MarketAnalyzer) isTrendEmerging(candles []models.CandleStick, adx float64) bool {
	if len(candles) < 20 {
		return false
	}

	recentADX := adx

	if recentADX > 20 && recentADX < 35 {
		emaAlignment := ma.isEMABullishAlignment(candles)
		recentVolume := ma.averageVolume(candles[len(candles)-5:])
		avgVolume := ma.averageVolume(candles)

		if emaAlignment && recentVolume > avgVolume*1.1 {
			return true
		}
	}

	return false
}

func (ma *MarketAnalyzer) calculateEMA(candles []models.CandleStick, period int) []float64 {
	if period <= 0 || len(candles) < period {
		return nil
	}

	multiplier := 2.0 / float64(period+1)
	ema := make([]float64, len(candles))

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i].Close
	}
	ema[period-1] = sum / float64(period)

	for i := period; i < len(candles); i++ {
		ema[i] = (candles[i].Close-ema[i-1])*multiplier + ema[i-1]
	}

	out := make([]float64, 0, len(candles)-(period-1))
	for i := period - 1; i < len(candles); i++ {
		out = append(out, ema[i])
	}
	return out
}

func (ma *MarketAnalyzer) calculateIchimokuCloud(candles []models.CandleStick) models.IchimokuCloud {
	if len(candles) < ma.IchimokuSpanBPeriod {
		return models.NeutralIchi
	}

	ichimoku := algos.IchimokuStrategy{
		ConversionPeriod: ma.IchimokuConversionPeriod,
		BasePeriod:       ma.IchimokuBasePeriod,
		SpanBPeriod:      ma.IchimokuSpanBPeriod,
	}

	result, err := ichimoku.Calculate(candles)
	if err != nil {
		return models.NeutralIchi
	}

	if result.Bullish {
		return models.BullishIchi
	} else if result.Bearish {
		return models.BearishIchi
	}

	return models.NeutralIchi
}

func (ma *MarketAnalyzer) detectFractalCharacteristics(candles []models.CandleStick) models.FractalSignal {
	if len(candles) < ma.FractalLookback {
		return models.FractalSignalNone
	}
	lookback := ma.FractalLookback
	win := candles[len(candles)-lookback:]

	hi, lo := win[0].High, win[0].Low
	for _, c := range win {
		if c.High > hi {
			hi = c.High
		}
		if c.Low < lo {
			lo = c.Low
		}
	}
	cur := candles[len(candles)-1].Close
	width := hi - lo
	if width <= 0 {
		return models.FractalSignalNone
	}

	if cur >= hi-0.05*width || cur <= lo+0.05*width {
		return models.FractalSignalBreakout
	}
	return models.FractalSignalNone
}

func (ma *MarketAnalyzer) analyzeVolumeProfile(candles []models.CandleStick) models.VolumeProfile {
	if len(candles) < 20 {
		return models.BalancedProfile
	}

	upVolumeCandles := candles[len(candles)-10:]
	totalUpVolume := 0.0
	totalDownVolume := 0.0

	for i := 1; i < len(upVolumeCandles); i++ {
		if upVolumeCandles[i].Close > upVolumeCandles[i-1].Close {
			totalUpVolume += upVolumeCandles[i].Volume
		} else {
			totalDownVolume += upVolumeCandles[i].Volume
		}
	}

	if totalUpVolume == 0 && totalDownVolume == 0 {
		return models.BalancedProfile
	}

	ratio := totalUpVolume / (totalUpVolume + totalDownVolume)

	if ratio > 0.65 {
		return models.BalancedProfile
	} else if ratio < 0.35 {
		return models.UnbalancedProfile
	}

	return models.BalancedProfile
}

func (ma *MarketAnalyzer) smoothValues(values []float64, period int) []float64 {
	if period <= 0 || len(values) < period {
		return nil
	}
	smoothed := make([]float64, len(values)-period+1)
	for i := 0; i <= len(values)-period; i++ {
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += values[i+j]
		}
		smoothed[i] = sum / float64(period)
	}
	return smoothed
}

func (ma *MarketAnalyzer) average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func (ma *MarketAnalyzer) DetectMarketRegime(c []models.CandleStick) models.MarketRegime {
	return ma.detectMarketRegime(c)
}

func (ma *MarketAnalyzer) DetectVolatilityRegime(c []models.CandleStick) models.VolatilityRegime {
	return ma.detectVolatilityRegime(c)
}

func (ma *MarketAnalyzer) AssessPriceActionQuality(c []models.CandleStick) float64 {
	return ma.assessPriceActionQuality(c)
}

func (ma *MarketAnalyzer) AssessMomentumQuality(c []models.CandleStick) float64 {
	return ma.assessMomentumQuality(c)
}

func (ma *MarketAnalyzer) CalculateNoiseLevel(c []models.CandleStick) float64 {
	return ma.calculateNoiseLevel(c)
}

func (ma *MarketAnalyzer) SimulateMultiTimeframeAnalysis(c []models.CandleStick) float64 {
	return ma.simulateMultiTimeframeAnalysis(c)
}

func (ma *MarketAnalyzer) AnalyzeVolumeProfile(c []models.CandleStick) models.VolumeProfile {
	return ma.analyzeVolumeProfile(c)
}

func (ma *MarketAnalyzer) AssessPriceStructure(c []models.CandleStick) float64 {
	return ma.assessPriceStructure(c)
}

func (ma *MarketAnalyzer) IsDivergencePresent(c []models.CandleStick) bool {
	return ma.isDivergencePresent(c)
}

func (ma *MarketAnalyzer) averageVolume(candles []models.CandleStick) float64 {
	if len(candles) == 0 {
		return 0
	}

	sum := 0.0
	for _, candle := range candles {
		sum += candle.Volume
	}
	return sum / float64(len(candles))
}

func (ma *MarketAnalyzer) detectPeaks(candles []models.CandleStick, lookForHighs bool) []int {
	var peaks []int
	minDistance := 2

	for i := 1; i < len(candles)-1; i++ {
		if lookForHighs {
			if candles[i].High > candles[i-1].High && candles[i].High > candles[i+1].High {
				if len(peaks) == 0 || i-peaks[len(peaks)-1] >= minDistance {
					peaks = append(peaks, i)
				}
			}
		} else {
			if candles[i].Low < candles[i-1].Low && candles[i].Low < candles[i+1].Low {
				if len(peaks) == 0 || i-peaks[len(peaks)-1] >= minDistance {
					peaks = append(peaks, i)
				}
			}
		}
	}

	return peaks
}

func (ma *MarketAnalyzer) Validate() error {
	v := validator.New()
	return v.Struct(ma)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
