package analysis

import (
	"binance_bot/algos"
	"binance_bot/interfaces"
	"binance_bot/logger"
	"binance_bot/models"
	"binance_bot/strategies"
	"github.com/go-playground/validator/v10"
	"math"
)

// MarketAnalyzer analyzes market conditions for trading decisions
type MarketAnalyzer struct {
	ATRPeriod               int     `validate:"required"` // Period for ATR calculation
	ADXPeriod               int     `validate:"required"` // Period for ADX calculation
	HighVolatilityThreshold float64 `validate:"required"` // Threshold to identify high volatility
	StrongTrendThreshold    float64 `validate:"required"` // Threshold to identify strong trends
}

// AnalyzeMarket determines the market state based on volatility and trend
func (ma *MarketAnalyzer) AnalyzeMarket(candles []models.CandleStick) (marketState models.MarketState, atr float64, adx float64) {
	if len(candles) < ma.ATRPeriod || len(candles) < ma.ADXPeriod {
		logger.Debug("[WARN] Insufficient candles for analysis. Expected at least %d candles, got %d", ma.ATRPeriod, len(candles))
		return models.Default, 0, 0
	}

	atr = ma.calculateATR(candles)
	adx = ma.calculateADX(candles)
	isTrending := ma.IsUptrend(candles)

	logger.Debugf("Market Analysis | ATR: %.2f, ADX: %.2f, IsTrending: %v", atr, adx, isTrending)

	switch {
	case atr > ma.HighVolatilityThreshold && !isTrending:
		return models.Chaotic, atr, adx
	case atr < ma.HighVolatilityThreshold && isTrending && adx > ma.StrongTrendThreshold:
		return models.Trending, atr, adx
	case atr < ma.HighVolatilityThreshold && !isTrending:
		return models.RangeBound, atr, adx
	default:
		return models.Default, atr, adx
	}
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

func (ma *MarketAnalyzer) SuggestStrategy(marketState models.MarketState) interfaces.Strategy {
	switch marketState {
	case models.Trending:
		return &strategies.CompoundStrategy{
			RSI: &algos.RSIStrategy{
				Overbought: 70,
				Oversold:   30,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   12,
				SlowPeriod:   26,
				SignalPeriod: 9,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.0,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 80,
				Oversold:   20,
				Period:     14,
			},
			CandleInterval:            "4h",
			DesiredProfit:             8.0,
			HighestPriceFallOffMargin: 5.0,
			FeeRate:                   0.001,
			MarketState:               models.Trending,
		}
	case models.Chaotic:
		return &strategies.CompoundStrategy{
			MACD: &algos.MACDStrategy{
				FastPeriod:   6,
				SlowPeriod:   13,
				SignalPeriod: 5,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 10,
				Width:  2.5,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 85,
				Oversold:   15,
				Period:     7,
			},
			CandleInterval:            "30m",
			DesiredProfit:             2.5,
			HighestPriceFallOffMargin: 1.5,
			FeeRate:                   0.001,
			MarketState:               models.Chaotic,
		}
	case models.RangeBound:
		return &strategies.CompoundStrategy{
			MACD: &algos.MACDStrategy{
				FastPeriod:   9,
				SlowPeriod:   21,
				SignalPeriod: 7,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 70,
				Oversold:   30,
				Period:     14,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.0,
			},
			RSI: &algos.RSIStrategy{
				Overbought: 70,
				Oversold:   30,
				Period:     14,
			},
			CandleInterval:            "1h",
			DesiredProfit:             2.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.RangeBound,
		}
	default:
		return &strategies.CompoundStrategy{
			RSI: &algos.RSIStrategy{
				Overbought: 70,
				Oversold:   35,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   12,
				SlowPeriod:   26,
				SignalPeriod: 9,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.0,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 80,
				Oversold:   20,
				Period:     14,
			},
			CandleInterval:            "1h",
			DesiredProfit:             1.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.Default,
		}
	}
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
	shortPeriod := 12 // Short-term EMA period
	longPeriod := 26  // Long-term EMA period
	adxThreshold := ma.StrongTrendThreshold

	// Ensure we have enough candles for the calculation
	if len(candles) < longPeriod+ma.ADXPeriod {
		logger.Warnf("Insufficient candles for uptrend detection. Expected at least %d, got %d", longPeriod+ma.ADXPeriod, len(candles))
		return false
	}

	// Calculate EMAs
	shortEMA := ma.calculateEMA(candles, shortPeriod)
	longEMA := ma.calculateEMA(candles, longPeriod)

	// Ensure EMA lengths match for comparison
	minLength := len(shortEMA)
	if len(longEMA) < minLength {
		minLength = len(longEMA)
	}
	shortEMA = shortEMA[len(shortEMA)-minLength:]
	longEMA = longEMA[len(longEMA)-minLength:]

	// Check if the short-term EMA is above the long-term EMA
	if shortEMA[len(shortEMA)-1] <= longEMA[len(longEMA)-1] {
		logger.Debugf("EMA indicates no uptrend: ShortEMA=%.2f, LongEMA=%.2f", shortEMA[len(shortEMA)-1], longEMA[len(longEMA)-1])
		return false
	}

	// Calculate ADX
	adx := ma.calculateADX(candles)
	if adx < adxThreshold {
		logger.Debugf("ADX below threshold: ADX=%.2f, Threshold=%.2f", adx, adxThreshold)
		return false
	}

	logger.Debugf("Uptrend detected: ShortEMA=%.2f, LongEMA=%.2f, ADX=%.2f", shortEMA[len(shortEMA)-1], longEMA[len(longEMA)-1], adx)
	return true
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
