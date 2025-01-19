package analysis

import (
	"github.com/M1chlCZ/bingo-bot/algos"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/go-playground/validator/v10"
	"math"
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
	ichimokuSignal := ma.calculateIchimokuCloud(candles)
	volumeSignal := ma.analyzeVolume(candles)
	fractalSignal := ma.detectFractalCharacteristics(candles)

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

	var trendingScore, chaoticScore, rangeScore float64

	// --- Trending conditions (existing) ---
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

	logger.Debugf("[State Scores] Trending=%.2f | Chaotic=%.2f | RangeBound=%.2f", trendingScore, chaoticScore, rangeScore)

	scores := map[models.MarketState]float64{
		models.Trending:   trendingScore,
		models.Chaotic:    chaoticScore,
		models.RangeBound: rangeScore,
	}

	var chosenState models.MarketState
	highestScore := math.Inf(-1)
	for state, score := range scores {
		if score > highestScore {
			highestScore = score
			chosenState = state
		}
	}

	requiredScore := 1.5
	strongTrendThreshold := 3.5
	transitionalThreshold := 0.5

	if highestScore < requiredScore {
		if highestScore > transitionalThreshold {
			logger.Infof("Scores not enough for main states but above transitional threshold. Using TRANSITIONAL.")
			return models.Transitional, atr, adx
		}
		logger.Debugf("No state met the required score (%.2f). Using DEFAULT. Scores: T=%.2f C=%.2f R=%.2f",
			requiredScore, trendingScore, chaoticScore, rangeScore)
		return models.Default, atr, adx
	}

	if chosenState == models.Trending && trendingScore >= strongTrendThreshold {
		logger.Debugf("Chosen Market State: StronglyTrending with Score=%.2f | T=%.2f C=%.2f R=%.2f",
			highestScore, trendingScore, chaoticScore, rangeScore)
		return models.StronglyTrending, atr, adx
	}

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

// IsUptrend determines if the *current candle* is moving upwards
func (ma *MarketAnalyzer) IsUptrend(candles []models.CandleStick) bool {
	if len(candles) < 2 {
		return false
	}

	lastCandle := candles[len(candles)-1]

	// 1) Intrabar up check
	if lastCandle.Close <= lastCandle.Open {
		return false
	}

	// 2) short EMA
	if len(candles) < 5 {
		return true // fallback if not enough data for EMA
	}
	shortEmaValues := ma.calculateEMA(candles, 5)
	if len(shortEmaValues) == 0 {
		return true
	}
	lastEMA := shortEmaValues[len(shortEmaValues)-1]
	return lastCandle.Close > lastEMA
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
