package analysis

import (
	"github.com/M1chlCZ/bingo-bot/algos"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/go-playground/validator/v10"
	"math"
	"sort"
)

// MarketAnalyzer analyzes market conditions for trading decisions
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

	// Optional Indicators (MFI, CCI)
	MFIPeriod     int     `json:"mfiPeriod"`
	MFIOverbought float64 `json:"mfiOverbought"`
	MFIOversold   float64 `json:"mfiOversold"`

	CCIPeriod     int     `json:"cciPeriod"`
	CCIOverbought float64 `json:"cciOverbought"`
	CCIOversold   float64 `json:"cciOversold"`
}

var mfiAlgo *algos.MFIStrategy
var cciAlgo *algos.CCIStrategy

func NewMarketAnalyzer(analyzer MarketAnalyzer) *MarketAnalyzer {
	if err := validator.New().Struct(analyzer); err != nil {
		logger.Fatalf("Invalid MarketAnalyzer configuration: %v", err)
	}
	if analyzer.MFIPeriod != 0 && analyzer.MFIOverbought != 0 && analyzer.MFIOversold != 0 {
		logger.Infof("MFI Strategy enabled for MarketAnalysis with Period=%d, Overbought=%v, Oversold=%v",
			analyzer.MFIPeriod, analyzer.MFIOverbought, analyzer.MFIOversold)
		mfiAlgo = &algos.MFIStrategy{
			Overbought: int(analyzer.MFIOverbought),
			Oversold:   int(analyzer.MFIOversold),
			Period:     analyzer.MFIPeriod,
		}
	}

	if analyzer.CCIPeriod != 0 && analyzer.CCIOverbought != 0 && analyzer.CCIOversold != 0 {
		logger.Infof("CCI Strategy enabled for MarketAnalysis with Period=%d, Overbought=%v, Oversold=%v",
			analyzer.CCIPeriod, analyzer.CCIOverbought, analyzer.CCIOversold)
		cciAlgo = &algos.CCIStrategy{
			Period:     analyzer.CCIPeriod,
			Overbought: analyzer.CCIOverbought,
			Oversold:   analyzer.CCIOversold,
		}
	}
	// Sort the EMA periods in ascending order
	sort.Ints(analyzer.EMAPeriods)
	return &MarketAnalyzer{
		ATRPeriod:                analyzer.ATRPeriod,
		ADXPeriod:                analyzer.ADXPeriod,
		HighVolatilityThreshold:  analyzer.HighVolatilityThreshold,
		StrongTrendThreshold:     analyzer.StrongTrendThreshold,
		IchimokuConversionPeriod: analyzer.IchimokuConversionPeriod,
		IchimokuBasePeriod:       analyzer.IchimokuBasePeriod,
		IchimokuSpanBPeriod:      analyzer.IchimokuSpanBPeriod,
		VolumeThreshold:          analyzer.VolumeThreshold,
		FractalLookback:          analyzer.FractalLookback,
		MFIPeriod:                analyzer.MFIPeriod,
		MFIOverbought:            analyzer.MFIOverbought,
		MFIOversold:              analyzer.MFIOversold,
		CCIPeriod:                analyzer.CCIPeriod,
		CCIOverbought:            analyzer.CCIOverbought,
		CCIOversold:              analyzer.CCIOversold,
		EMAPeriods:               analyzer.EMAPeriods,
	}
}

// AnalyzeMarket determines the market state based on various signals
func (ma *MarketAnalyzer) AnalyzeMarket(candles []models.CandleStick) (marketState models.MarketState, atr float64, adx float64) {
	if len(candles) < ma.ATRPeriod || len(candles) < ma.ADXPeriod {
		logger.Debugf("[WARN] Insufficient candles for analysis. Expected at least %d candles, got %d",
			ma.ATRPeriod, len(candles))
		return models.Default, 0, 0
	}

	// Core Indicators
	atr = ma.calculateATR(candles)
	adx = ma.calculateADX(candles)
	isUptrend := ma.IsUptrend(candles)

	// Additional Indicators
	emaAlignment := ma.isEMABullishAlignment(candles)
	ichimokuSignal := ma.calculateIchimokuCloud(candles)
	volumeSignal := ma.analyzeVolume(candles)
	fractalSignal := ma.detectFractalCharacteristics(candles)
	isRange := ma.isRangeBoundMarket(candles, atr)
	volumeProfile := ma.analyzeVolumeProfile(candles)
	trendTransition := ma.detectTrendTransitions(candles, adx)

	// OPTIONAL Indicators
	var mfiVal float64
	var mfiSignal int
	var mfiEnabled = mfiAlgo != nil
	if mfiEnabled {
		var errMfi error
		mfiVal, mfiSignal, errMfi = mfiAlgo.Calculate(candles, "")
		if errMfi != nil {
			logger.Debugf("Error computing MFI: %v", errMfi)
		} else {
			logger.Debugf("MFI=%.2f, MFI-Signal=%d", mfiVal, mfiSignal)
		}
	}
	var cciVal float64
	var cciSignal int
	var cciEnabled = cciAlgo != nil
	if cciEnabled {
		var errCci error
		cciVal, cciSignal, errCci = cciAlgo.Calculate(candles, "")
		if errCci != nil {
			logger.Debugf("Error computing CCI: %v", errCci)
		} else {
			logger.Debugf("CCI=%.2f, CCI-Signal=%d", cciVal, cciSignal)
		}
	}

	logger.Debugf("[Market Analysis Raw Data] ATR=%.2f | ADX=%.2f | Uptrend=%v | Ichimoku=%v | Volume=%v | Fractal=%v | MFI=%.2f | CCI=%.2f",
		atr, adx, isUptrend, ichimokuSignal, volumeSignal, fractalSignal, mfiVal, cciVal)

	var trendingScore, chaoticScore, rangeScore, transitionalScore float64

	// --- Trending conditions (existing) ---
	if emaAlignment {
		trendingScore += 2.0
	}
	if isUptrend {
		trendingScore += 1.5
	}
	if adx > ma.StrongTrendThreshold {
		trendingScore += 1.0
	}
	if ichimokuSignal == models.Bullish {
		trendingScore += 0.5
	}
	if volumeSignal == models.Rising {
		trendingScore += 1.0
	}

	if mfiEnabled {
		if mfiVal < ma.MFIOverbought {
			trendingScore += 0.5 // a slight bump
		}
	}

	if cciEnabled {
		if cciVal < ma.CCIOverbought && cciVal > ma.CCIOversold {
			// CCI is not extremely over/under
			trendingScore += 0.5
		}
	}

	// --- Chaotic conditions (existing) ---
	if atr > ma.HighVolatilityThreshold {
		chaoticScore += 2.0
	}
	if fractalSignal == models.BreakoutChannel {
		chaoticScore += 1.5
	}
	if volumeProfile == models.Unbalanced {
		chaoticScore += 1.0
	}
	if mfiVal > ma.MFIOverbought || mfiVal < ma.MFIOversold {
		chaoticScore += 0.5
	}

	// NEW: maybe if MFI > Overbought => we suspect chaotic blow-off top
	if mfiEnabled {
		if mfiVal > ma.MFIOverbought {
			chaoticScore += 0.5
		}
	}
	// Similarly if CCI is extremely high or extremely low => might be chaotic
	if cciEnabled {
		if cciVal > ma.CCIOverbought || cciVal < ma.CCIOversold {
			chaoticScore += 0.5
		}
	}

	// --- Range-Bound conditions (existing) ---
	if isRange {
		rangeScore += 2.0
	}
	if volumeProfile == models.Balanced {
		rangeScore += 1.5
	}
	if adx < 25 {
		rangeScore += 1.0
	}
	if math.Abs(cciVal) < 100 {
		rangeScore += 0.5
	}

	// NEW: if MFI in mid-range => 40–60 => might indicate sideways range
	if mfiEnabled {
		if mfiVal >= ma.MFIOversold && mfiVal <= ma.MFIOverbought {
			rangeScore += 0.5
		}
	}
	// If CCI near 0 => not strongly trending => might be range
	if cciEnabled {
		if math.Abs(cciVal) < 50 {
			rangeScore += 0.5
		}
	}

	// --- Transitional Conditions ---
	if trendTransition != models.Default {
		transitionalScore += 3.0
	}
	if ma.isDivergencePresent(candles) {
		transitionalScore += 2.0
	}
	if ma.isTrendEmerging(candles, adx) {
		transitionalScore += 1.5
	}

	logger.Debugf("[State Scores] Trending=%.2f | Chaotic=%.2f | RangeBound=%.2f", trendingScore, chaoticScore, rangeScore)

	scores := map[models.MarketState]float64{
		models.StronglyTrending: trendingScore * 1.2,
		models.Trending:         trendingScore,
		models.Chaotic:          chaoticScore,
		models.RangeBound:       rangeScore,
		models.Transitional:     transitionalScore,
	}

	// Find highest scoring state
	var maxState models.MarketState
	maxScore := -1.0
	for state, score := range scores {
		if score > maxScore {
			maxScore = score
			maxState = state
		}
	}

	// Post-Processing Rules
	finalState := maxState
	if maxState == models.Trending && trendingScore >= 6.0 {
		finalState = models.StronglyTrending
	}

	if finalState == models.StronglyTrending && adx < 30 {
		finalState = models.Trending
	}

	if transitionalScore > 4.0 && maxScore-transitionalScore < 1.5 {
		finalState = models.Transitional
	}

	logger.Debugf("[Final Market State] %s (Score=%.2f)", finalState.String(), maxScore)
	return finalState, atr, adx
}

// CalculateVolatility measures the volatility of the market using the standard deviation of the candles' close prices
func (ma *MarketAnalyzer) CalculateVolatility(candles []models.CandleStick) float64 {
	if len(candles) == 0 {
		return 0
	}

	var sum, mean, variance float64

	// Calculate the mean of closing prices
	for _, candle := range candles {
		sum += candle.Close
	}
	mean = sum / float64(len(candles))

	// Calculate the variance
	for _, candle := range candles {
		deviation := candle.Close - mean
		variance += deviation * deviation
	}

	// Variance divided by number of data points
	variance /= float64(len(candles))

	// Standard deviation is the square root of variance
	return math.Sqrt(variance)
}

// calculateATR calculates the Average True Range (ATR)
func (ma *MarketAnalyzer) calculateATR(candles []models.CandleStick) float64 {
	if len(candles) < ma.ATRPeriod+1 { // Ensure we have enough candles for the calculation
		return 0
	}

	var trueRanges []float64
	for i := 1; i < len(candles); i++ {
		currentHigh := candles[i].High
		currentLow := candles[i].Low
		previousClose := candles[i-1].Close

		trueRange := math.Max(currentHigh-currentLow,
			math.Max(math.Abs(currentHigh-previousClose), math.Abs(currentLow-previousClose)))
		trueRanges = append(trueRanges, trueRange)
	}

	if len(trueRanges) < ma.ATRPeriod { // Ensure trueRanges has enough data for slicing
		return 0
	}

	// Calculate ATR as the average of the last `ATRPeriod` true ranges
	return ma.average(trueRanges[len(trueRanges)-ma.ATRPeriod:])
}

// calculateADX calculates the Average Directional Index (ADX)
func (ma *MarketAnalyzer) calculateADX(candles []models.CandleStick) float64 {
	if len(candles) < ma.ADXPeriod+1 { // Ensure we have enough candles for the calculation
		return 0
	}

	var plusDM, minusDM, trValues []float64
	for i := 1; i < len(candles); i++ {
		currentHigh := candles[i].High
		currentLow := candles[i].Low
		previousHigh := candles[i-1].High
		previousLow := candles[i-1].Low

		tr := math.Max(currentHigh-currentLow,
			math.Max(math.Abs(currentHigh-candles[i-1].Close), math.Abs(currentLow-candles[i-1].Close)))
		trValues = append(trValues, tr)

		if currentHigh-previousHigh > previousLow-currentLow {
			plusDM = append(plusDM, math.Max(currentHigh-previousHigh, 0))
			minusDM = append(minusDM, 0)
		} else {
			minusDM = append(minusDM, math.Max(previousLow-currentLow, 0))
			plusDM = append(plusDM, 0)
		}
	}

	trSmooth := ma.smoothValues(trValues, ma.ADXPeriod)
	plusDMSmooth := ma.smoothValues(plusDM, ma.ADXPeriod)
	minusDMSmooth := ma.smoothValues(minusDM, ma.ADXPeriod)

	if len(trSmooth) < ma.ADXPeriod || len(plusDMSmooth) < ma.ADXPeriod || len(minusDMSmooth) < ma.ADXPeriod {
		return 0 // Ensure smoothed values have enough data
	}

	var adxValues []float64
	for i := 0; i < len(trSmooth); i++ {
		if trSmooth[i] == 0 { // Avoid division by zero
			continue
		}
		plusDI := 100 * plusDMSmooth[i] / trSmooth[i]
		minusDI := 100 * minusDMSmooth[i] / trSmooth[i]
		dx := math.Abs(plusDI-minusDI) / (plusDI + minusDI) * 100
		adxValues = append(adxValues, dx)
	}

	if len(adxValues) < ma.ADXPeriod { // Ensure adxValues has enough data for slicing
		return 0
	}

	return ma.average(adxValues[len(adxValues)-ma.ADXPeriod:])
}

// IsUptrend determines if the *current candle* is moving upwards
func (ma *MarketAnalyzer) IsUptrend(candles []models.CandleStick) bool {
	if len(candles) < 10 {
		logger.Infof("Insufficient candles for trend detection. Got %d\n", len(candles))
		return false
	}

	// Price Action Analysis
	priceDirection := ma.analyzePriceDirection(candles)
	if priceDirection == models.Downtrend {
		return false
	}

	// Momentum Confirmation
	momentum := ma.checkMomentum(candles)
	if momentum == models.Weak {
		return false
	}

	// Volume Validation
	if !ma.validateVolume(candles) {
		return false
	}

	// Trend Strength Check
	trendStrength := ma.assessTrendStrength(candles)
	if trendStrength == models.WeakTrend {
		return false
	}

	// Top Detection
	if ma.isNearTop(candles) {
		logger.Debugf("Price is near a potential top, not confirming uptrend")
		return false
	}

	return true
}

// Helper methods for IsUptrend
func (ma *MarketAnalyzer) analyzePriceDirection(candles []models.CandleStick) models.TrendDirection {
	// Use EMA crossover and slope analysis
	shortEMA := ma.calculateEMA(candles, 9)
	longEMA := ma.calculateEMA(candles, 21)

	if len(shortEMA) < 2 || len(longEMA) < 2 {
		return models.NoTrend
	}

	// Check EMA alignment and slope
	if shortEMA[len(shortEMA)-1] > longEMA[len(longEMA)-1] &&
		shortEMA[len(shortEMA)-1] > shortEMA[len(shortEMA)-2] {
		return models.Uptrend
	}
	return models.Downtrend
}

func (ma *MarketAnalyzer) checkMomentum(candles []models.CandleStick) models.MomentumStrength {
	rsi := algos.RSIStrategy{Period: 14}
	rsiVal, _, _ := rsi.Calculate(candles, "")

	macd := algos.MACDStrategy{FastPeriod: 12, SlowPeriod: 26, SignalPeriod: 9}
	_, _, _, macdSignal, _ := macd.Calculate(candles)

	if rsiVal > 60 && macdSignal == 1 {
		return models.Strong
	}
	if rsiVal > 50 && macdSignal != -1 {
		return models.Moderate
	}
	return models.Weak
}

func (ma *MarketAnalyzer) validateVolume(candles []models.CandleStick) bool {
	// Check if volume is increasing with price
	recentVolume := ma.averageVolume(candles[len(candles)-3:])
	previousVolume := ma.averageVolume(candles[len(candles)-6 : len(candles)-3])

	return recentVolume > previousVolume*1.1
}

func (ma *MarketAnalyzer) assessTrendStrength(candles []models.CandleStick) models.TrendStrength {
	adx := ma.calculateADX(candles)
	atr := ma.calculateATR(candles)

	if adx > 30 && atr > ma.HighVolatilityThreshold*0.7 {
		return models.StrongTrend
	}
	if adx > 25 {
		return models.ModerateTrend
	}
	return models.WeakTrend
}

func (ma *MarketAnalyzer) isNearTop(candles []models.CandleStick) bool {
	// Detect potential tops using RSI divergence and price patterns
	rsi := algos.RSIStrategy{Period: 14}
	rsiVals := make([]float64, len(candles))
	for i := range candles {
		val, _, _ := rsi.Calculate(candles[:i+1], "")
		rsiVals[i] = val
	}

	// Check for bearish divergence
	if ma.hasBearishDivergence(candles, rsiVals) {
		return true
	}

	// Check for double/triple top patterns
	if ma.detectPriceTopPatterns(candles) {
		return true
	}

	return false
}

func (ma *MarketAnalyzer) hasBearishDivergence(candles []models.CandleStick, rsiVals []float64) bool {
	// Look for higher highs in price with lower highs in RSI
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
	// Detect double/triple top patterns
	if len(candles) < 10 {
		return false
	}

	// Get recent highs
	recentHighs := make([]float64, 3)
	for i := 0; i < 3; i++ {
		recentHighs[i] = candles[len(candles)-1-i].High
	}

	// Check for similar highs within 1% range
	if math.Abs(recentHighs[0]-recentHighs[1])/recentHighs[0] < 0.01 &&
		math.Abs(recentHighs[1]-recentHighs[2])/recentHighs[1] < 0.01 {
		return true
	}

	return false
}

func (ma *MarketAnalyzer) checkUptrendWithLongSMAs(candles []models.CandleStick) bool {
	shortPeriod := 20
	longPeriod := 50

	shortSMA := ma.calculateSMA(candles, shortPeriod)
	longSMA := ma.calculateSMA(candles, longPeriod)
	if shortSMA == nil || longSMA == nil {
		logger.Infof("Not enough data to calculate long SMAs for trend detection.")
		return false
	}

	recentShortSMA := shortSMA[len(shortSMA)-1]
	recentLongSMA := longSMA[len(longSMA)-1]

	if recentShortSMA <= recentLongSMA {
		return false
	}

	// Check that short SMA above long SMA for last 3 bars
	if !ma.isShortAboveLongForBars(shortSMA, longSMA, 3) {
		return false
	}

	// SMA Slope Check
	if !ma.isSMAIncreasing(shortSMA, 3) {
		return false
	}

	// Price Action Check
	if !ma.isPriceMakingHigherHighs(candles, 3) {
		return false
	}

	return true
}

func (ma *MarketAnalyzer) checkUptrendWithShortSMAs(candles []models.CandleStick) bool {
	// Use shorter periods, e.g., 7 and 14
	shortPeriod := 7
	longPeriod := 14

	shortSMA := ma.calculateSMA(candles, shortPeriod)
	longSMA := ma.calculateSMA(candles, longPeriod)
	if shortSMA == nil || longSMA == nil {
		logger.Infof("Not enough data for even short SMAs.")
		return false
	}

	if shortSMA[len(shortSMA)-1] <= longSMA[len(longSMA)-1] {
		return false
	}

	// Less strict checks since we have less data
	if !ma.isShortAboveLongForBars(shortSMA, longSMA, 2) {
		return false
	}

	// Maybe skip slope check or just do 2 bars
	if !ma.isSMAIncreasing(shortSMA, 2) {
		return false
	}

	// Price action check with fewer bars
	if !ma.isPriceMakingHigherHighs(candles, 2) {
		return false
	}

	return true
}

// Helper function to check if short SMA has been consistently above long SMA for given bars
func (ma *MarketAnalyzer) isShortAboveLongForBars(shortSMA, longSMA []float64, bars int) bool {
	// To compare apples to apples, align arrays by their ending indices
	minLen := len(longSMA)
	shortAligned := shortSMA[len(shortSMA)-minLen:]
	longAligned := longSMA

	if len(shortAligned) < bars {
		return false
	}
	for i := len(shortAligned) - bars; i < len(shortAligned); i++ {
		if shortAligned[i] <= longAligned[i] {
			return false
		}
	}
	return true
}

// Check if SMA is increasing over the last 'bars' periods
// i.e., each subsequent SMA value should be higher than the previous one
func (ma *MarketAnalyzer) isSMAIncreasing(sma []float64, bars int) bool {
	if len(sma) < bars+1 {
		return false
	}

	for i := len(sma) - bars; i < len(sma); i++ {
		if i == 0 {
			continue
		}
		if sma[i] <= sma[i-1] {
			return false
		}
	}
	return true
}

// Check if price is making higher highs for the last 'bars' candles
// This can help confirm that not only the SMAs are aligned, but price action is also trending up.
func (ma *MarketAnalyzer) isPriceMakingHigherHighs(candles []models.CandleStick, bars int) bool {
	if len(candles) < bars+1 {
		return false
	}

	// Check if each subsequent high is greater than the previous one
	for i := len(candles) - bars; i < len(candles); i++ {
		if i == len(candles)-bars {
			continue // Skip the first in the series since we have nothing to compare it to
		}
		// Ensure the current candle's close is higher than the previous candle's close
		if candles[i].Close <= candles[i-1].Close {
			return false
		}
	}
	return true
}

// Helper function to calculate SMA
func (ma *MarketAnalyzer) calculateSMA(candles []models.CandleStick, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	sma := make([]float64, len(candles)-period+1)
	for i := 0; i <= len(candles)-period; i++ {
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += candles[i+j].Close
		}
		sma[i] = sum / float64(period)
	}
	return sma
}

func (ma *MarketAnalyzer) checkSimpleUptrend(candles []models.CandleStick, barsToCheck int) bool {
	// Simple heuristic: last few closes each higher than the previous
	// Or last close > average of last barsToCheck closes
	if len(candles) < barsToCheck {
		return false
	}

	sum := 0.0
	for i := len(candles) - barsToCheck; i < len(candles); i++ {
		sum += candles[i].Close
	}
	avg := sum / float64(barsToCheck)

	if candles[len(candles)-1].Close > avg {
		// Optional: Check if each close is higher than the previous
		for i := len(candles) - barsToCheck + 1; i < len(candles); i++ {
			if candles[i].Close <= candles[i-1].Close {
				return false
			}
		}
		return true
	}
	return false
}

func (ma *MarketAnalyzer) isEMABullishAlignment(candles []models.CandleStick) bool {
	if len(ma.EMAPeriods) < 3 {
		logger.Error("Need at least 3 EMA periods for ribbon analysis")
		return false
	}

	// Calculate all EMAs
	var emas [][]float64
	for _, period := range ma.EMAPeriods {
		ema := ma.calculateEMA(candles, period)
		if len(ema) < 1 {
			return false
		}
		emas = append(emas, ema)
	}

	// Check alignment (fastest EMA first)
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
	// Use ATR/Price ratio for volatility check
	currentPrice := candles[len(candles)-1].Close
	atrRatio := (atr / currentPrice) * 100

	// Bollinger Band Squeeze detection
	bb := algos.BollingerBands{Period: 20, Width: 2.0}
	lower, middle, upper, err := bb.Calculate(candles)
	if err != nil {
		logger.Error("BB calculation failed:", err)
		return false
	}
	bandWidth := (upper - lower) / middle

	// Price oscillation check
	oscScore := ma.priceOscillationScore(candles, 14)

	return atrRatio < 1.5 && bandWidth < 0.1 && oscScore > 0.7
}

func (ma *MarketAnalyzer) priceOscillationScore(candles []models.CandleStick, period int) float64 {
	if len(candles) < period {
		return 0
	}

	var touchesUpper, touchesLower int
	for _, c := range candles[len(candles)-period:] {
		bb := algos.BollingerBands{Period: period, Width: 2.0}
		lower, _, upper, _ := bb.Calculate(candles)

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
	// Check for potential trend exhaustion
	if ma.isDivergencePresent(candles) {
		return models.Transitional
	}

	// Check for emerging trends
	if ma.isTrendEmerging(candles, adx) { // here its missing implementation
		return models.Transitional
	}

	return models.Default
}

func (ma *MarketAnalyzer) detectPeaks(candles []models.CandleStick, lookForHighs bool) []int {
	var peaks []int
	minDistance := 2 // minimum distance between peaks

	for i := 1; i < len(candles)-1; i++ {
		if lookForHighs {
			if candles[i].High > candles[i-1].High && candles[i].High > candles[i+1].High {
				// Check distance from last peak
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

func (ma *MarketAnalyzer) detectPeaksFloat(values []float64, lookForHighs bool) []int {
	var peaks []int
	minDistance := 2

	for i := 1; i < len(values)-1; i++ {
		if lookForHighs {
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

func checkBearishDivergence(pricePeaks, rsiPeaks []int, candles []models.CandleStick, rsiVals []float64) bool {
	if len(pricePeaks) < 2 || len(rsiPeaks) < 2 {
		return false
	}

	lastPricePeak := pricePeaks[len(pricePeaks)-1]
	prevPricePeak := pricePeaks[len(pricePeaks)-2]
	lastRSIPeak := rsiPeaks[len(rsiPeaks)-1]
	prevRSIPeak := rsiPeaks[len(rsiPeaks)-2]

	return candles[lastPricePeak].Close > candles[prevPricePeak].Close &&
		rsiVals[lastRSIPeak] < rsiVals[prevRSIPeak]
}

// 3. Update isDivergencePresent to pass RSI values
func (ma *MarketAnalyzer) isDivergencePresent(candles []models.CandleStick) bool {
	rsi := algos.RSIStrategy{Period: 14}
	rsiVals := make([]float64, len(candles))
	for i := range candles {
		val, _, _ := rsi.Calculate(candles[:i+1], "")
		rsiVals[i] = val
	}

	pricePeaks := ma.detectPeaks(candles, true)
	rsiPeaks := ma.detectPeaksFloat(rsiVals, true)

	return checkBearishDivergence(pricePeaks, rsiPeaks, candles, rsiVals)
}

// 4. Implement missing isTrendEmerging
func (ma *MarketAnalyzer) isTrendEmerging(candles []models.CandleStick, adx float64) bool {
	// Check for ADX rising above 25 with price breaking out
	if adx < 25 || len(candles) < 20 {
		return false
	}

	// Check for recent breakout
	recentHigh := candles[len(candles)-1].High
	previousHigh := ma.calculateMaxHigh(candles, 20, 5) // Last 5 candles vs previous 15

	// Volume validation
	recentVolume := ma.averageVolume(candles[len(candles)-3:])
	previousVolume := ma.averageVolume(candles[len(candles)-8 : len(candles)-3])

	return recentHigh > previousHigh && recentVolume > previousVolume*1.2
}

// Helper functions for isTrendEmerging
func (ma *MarketAnalyzer) calculateMaxHigh(candles []models.CandleStick, lookback, offset int) float64 {
	start := len(candles) - lookback - offset
	if start < 0 {
		start = 0
	}

	maxHigh := 0.0
	for _, c := range candles[start : len(candles)-offset] {
		if c.High > maxHigh {
			maxHigh = c.High
		}
	}
	return maxHigh
}

func (ma *MarketAnalyzer) averageVolume(candles []models.CandleStick) float64 {
	total := 0.0
	for _, c := range candles {
		total += c.Volume
	}
	return total / float64(len(candles))
}

func (ma *MarketAnalyzer) analyzeVolumeProfile(candles []models.CandleStick) models.VolumeProfileType {
	// Implement simple volume-by-price analysis
	priceBuckets := make(map[float64]float64)
	for _, c := range candles {
		// Round to nearest 0.5% price level
		level := math.Round(c.Close*200) / 200
		priceBuckets[level] += c.Volume
	}

	// Find value area (70% of total volume)
	totalVolume := 0.0
	for _, v := range priceBuckets {
		totalVolume += v
	}

	// Sort price levels by volume
	type kv struct {
		Key   float64
		Value float64
	}
	var sorted []kv
	for k, v := range priceBuckets {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	// Calculate value area
	cumulative := 0.0
	valueArea := make(map[float64]bool)
	for _, kv := range sorted {
		cumulative += kv.Value
		valueArea[kv.Key] = true
		if cumulative/totalVolume >= 0.7 {
			break
		}
	}

	// Check current price position
	currentPrice := candles[len(candles)-1].Close
	if _, exists := valueArea[currentPrice]; exists {
		return models.Balanced
	}
	return models.Unbalanced
}

// calculateIchimokuCloud calculates a simplified Ichimoku signal
func (ma *MarketAnalyzer) calculateIchimokuCloud(candles []models.CandleStick) models.IchimokuCloud {
	if len(candles) < ma.IchimokuSpanBPeriod {
		return models.Neutral
	}

	tenkan := ma.calculateMidpoint(candles, ma.IchimokuConversionPeriod)
	kijun := ma.calculateMidpoint(candles, ma.IchimokuBasePeriod)
	spanB := ma.calculateMidpoint(candles, ma.IchimokuSpanBPeriod)

	currentPrice := candles[len(candles)-1].Close

	if tenkan > kijun && currentPrice > spanB {
		return models.Bullish
	} else if tenkan < kijun && currentPrice < spanB {
		return models.Bearish
	}
	return models.Neutral
}

// calculateMidpoint finds midpoint of highs and lows over a period
func (ma *MarketAnalyzer) calculateMidpoint(candles []models.CandleStick, period int) float64 {
	if len(candles) < period {
		return candles[len(candles)-1].Close
	}
	segment := candles[len(candles)-period:]
	high := segment[0].High
	low := segment[0].Low
	for _, c := range segment {
		if c.High > high {
			high = c.High
		}
		if c.Low < low {
			low = c.Low
		}
	}
	return (high + low) / 2.0
}

// analyzeVolume checks if volume is increasing, stable, or decreasing relative to a threshold
func (ma *MarketAnalyzer) analyzeVolume(candles []models.CandleStick) models.VolumeAnalysis {
	if len(candles) < 2 {
		return models.NeutralVolume
	}
	recentVolume := candles[len(candles)-1].Volume
	previousVolume := candles[len(candles)-2].Volume

	if recentVolume > ma.VolumeThreshold && recentVolume > previousVolume {
		return models.Rising
	} else if recentVolume < previousVolume {
		return models.Falling
	}
	return models.Stable
}

// detectFractalCharacteristics uses a Donchian channel concept to label market as range, breakout, or mixed
func (ma *MarketAnalyzer) detectFractalCharacteristics(candles []models.CandleStick) models.DonchianChannel {
	if len(candles) < ma.FractalLookback {
		return models.NeutralChannel
	}

	upper, lower := ma.donchianChannel(candles, ma.FractalLookback)
	currentPrice := candles[len(candles)-1].Close
	mid := (upper + lower) / 2.0

	if math.Abs(currentPrice-mid)/mid < 0.01 {
		return models.RangeChannel
	} else if currentPrice > upper*0.99 || currentPrice < lower*1.01 {
		return models.BreakoutChannel
	}
	return models.MixedChannel
}

func (ma *MarketAnalyzer) donchianChannel(candles []models.CandleStick, period int) (upper, lower float64) {
	segment := candles[len(candles)-period:]
	high := segment[0].High
	low := segment[0].Low
	for _, c := range segment {
		if c.High > high {
			high = c.High
		}
		if c.Low < low {
			low = c.Low
		}
	}
	return high, low
}

// calculateEMA calculates the Exponential Moving Average (EMA)
func (ma *MarketAnalyzer) calculateEMA(candles []models.CandleStick, period int) []float64 {
	if len(candles) < period {
		return nil // Not enough data
	}

	ema := make([]float64, len(candles))
	multiplier := 2.0 / float64(period+1)

	// Initialize EMA with the first close price
	ema[0] = candles[0].Close

	// Calculate EMA for the rest of the candles
	for i := 1; i < len(candles); i++ {
		ema[i] = ((candles[i].Close - ema[i-1]) * multiplier) + ema[i-1]
	}
	return ema
}

// smoothValues applies a moving average over a given period
func (ma *MarketAnalyzer) smoothValues(values []float64, period int) []float64 {
	smoothed := make([]float64, len(values)-period+1)
	for i := range smoothed {
		smoothed[i] = ma.average(values[i : i+period])
	}
	return smoothed
}

// average calculates the average of a slice of float64 values
func (ma *MarketAnalyzer) average(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func (ma *MarketAnalyzer) Validate() error {
	v := validator.New()
	return v.Struct(ma)
}
