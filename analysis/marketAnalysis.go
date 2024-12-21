package analysis

import (
	"binance_bot/logger"
	"binance_bot/models"
	"github.com/go-playground/validator/v10"
	"math"
)

// MarketAnalyzer analyzes market conditions for trading decisions
type MarketAnalyzer struct {
	ATRPeriod               int     `validate:"required"` // Period for ATR calculation
	ADXPeriod               int     `validate:"required"` // Period for ADX calculation
	HighVolatilityThreshold float64 `validate:"required"` // Threshold to identify high volatility
	StrongTrendThreshold    float64 `validate:"required"` // Threshold to identify strong trends

	IchimokuConversionPeriod int     `validate:"required"`
	IchimokuBasePeriod       int     `validate:"required"`
	IchimokuSpanBPeriod      int     `validate:"required"`
	VolumeThreshold          float64 `validate:"required"` // Threshold to consider volume "significant"
	FractalLookback          int     `validate:"required"` // Period used for Donchian or fractal analysis
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
	ichimokuSignal := ma.calculateIchimokuCloud(candles)      // "bullish", "bearish", or "neutral"
	volumeSignal := ma.analyzeVolume(candles)                 // "rising", "falling", "stable"
	fractalSignal := ma.detectFractalCharacteristics(candles) // "range", "breakout", "mixed"

	logger.Debugf("[Market Analysis Raw Data] ATR=%.2f | ADX=%.2f | Uptrend=%v | Ichimoku=%v | Volume=%v | Fractal=%v",
		atr, adx, isUptrend, ichimokuSignal, volumeSignal, fractalSignal)

	var trendingScore, chaoticScore, rangeScore float64

	// Trending conditions (as before)
	if atr < ma.HighVolatilityThreshold {
		trendingScore += 1.0
	}
	if isUptrend {
		trendingScore += 1.0
	}
	if adx > ma.StrongTrendThreshold {
		trendingScore += 1.0
	}
	if ichimokuSignal == models.Bullish {
		trendingScore += 1.0
	}
	if volumeSignal == models.Rising {
		trendingScore += 1.0
	} else if volumeSignal == models.Stable {
		trendingScore += 0.5
	}

	// Chaotic conditions
	if atr > ma.HighVolatilityThreshold {
		chaoticScore += 1.0
	}
	if !isUptrend {
		chaoticScore += 1.0
	}
	if fractalSignal == models.MixedChannel || fractalSignal == models.BreakoutChannel {
		chaoticScore += 1.0
	}
	if ichimokuSignal != models.Bullish && ichimokuSignal != models.Bearish {
		chaoticScore += 0.5
	}

	// Range-Bound conditions
	if atr < ma.HighVolatilityThreshold {
		rangeScore += 1.0
	}
	if adx < ma.StrongTrendThreshold {
		rangeScore += 1.0
	}
	if ichimokuSignal == models.Neutral {
		rangeScore += 1.0
	}
	if fractalSignal == models.RangeChannel {
		rangeScore += 1.0
	}

	logger.Debugf("[State Scores] Trending=%.2f | Chaotic=%.2f | RangeBound=%.2f", trendingScore, chaoticScore, rangeScore)

	// For now, let's just handle them here without changing models:
	// If you can't modify models easily, consider a local mapping.

	scores := map[models.MarketState]float64{
		models.Trending:   trendingScore,
		models.Chaotic:    chaoticScore,
		models.RangeBound: rangeScore,
		// Default will get 0 unless all fail
	}

	// Determine top state
	var chosenState models.MarketState
	highestScore := math.Inf(-1)
	for state, score := range scores {
		if score > highestScore {
			highestScore = score
			chosenState = state
		}
	}

	requiredScore := 1.5
	strongTrendThreshold := 3.5  // If trending score > 3.5, call it strongly trending
	transitionalThreshold := 0.5 // If we don't meet requiredScore but >0.5, call it transitional

	// If highestScore doesn't meet the requiredScore
	if highestScore < requiredScore {
		// Check if we have at least some minor indication
		if highestScore > transitionalThreshold {
			logger.Infof("Scores not enough for main states but above transitional threshold. Using TRANSITIONAL.")
			// You'll need to define TRANSITIONAL in your models:
			// e.g., const Transitional MarketState = iota + 4
			// For now assume models.Transitional exists:
			return models.Transitional, atr, adx
		}

		logger.Debugf("No state met the required score (%.2f). Using DEFAULT. Scores: T=%.2f C=%.2f R=%.2f",
			requiredScore, trendingScore, chaoticScore, rangeScore)
		return models.Default, atr, adx
	}

	// If Trending is chosen and score is very high, classify as StronglyTrending
	if chosenState == models.Trending && trendingScore >= strongTrendThreshold {
		logger.Debugf("Chosen Market State: StronglyTrending with Score=%.2f | T=%.2f C=%.2f R=%.2f",
			highestScore, trendingScore, chaoticScore, rangeScore)
		return models.StronglyTrending, atr, adx
	}

	// Otherwise return the chosen state normally
	logger.Debugf("Chosen Market State: %v with Score=%.2f | T=%.2f C=%.2f R=%.2f",
		chosenState, highestScore, trendingScore, chaoticScore, rangeScore)
	return chosenState, atr, adx
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

// IsUptrend determines if the market is in an uptrend based on EMA and ADX
func (ma *MarketAnalyzer) IsUptrend(candles []models.CandleStick) bool {
	totalCandles := len(candles)

	if totalCandles >= 50 {
		// Use the full method
		return ma.checkUptrendWithLongSMAs(candles)
	} else if totalCandles >= 20 {
		// Fallback #1: Shorter SMAs since we have fewer candles
		return ma.checkUptrendWithShortSMAs(candles)
	} else if totalCandles >= 10 {
		// Fallback #2: Simple price-based heuristic
		return ma.checkSimpleUptrend(candles, 3)
	} else {
		// Not enough data at all
		logger.Infof("Extremely insufficient candles for trend detection. Got %d\n", totalCandles)
		return false
	}
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
