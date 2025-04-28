# Binance Trading Bot Configuration Guide

This document provides a comprehensive guide to all configuration parameters used in the Binance trading bot. It explains their purpose, valid values, and default settings.

## Table of Contents

1. [Main Configuration](#main-configuration)
2. [Market Analysis Configuration](#market-analysis-configuration)
3. [Strategy Configuration](#strategy-configuration)
   - [Compound Strategy](#compound-strategy)
   - [Market State Strategies](#market-state-strategies)
4. [Configuration File Example](#configuration-file-example)

## Main Configuration

The main configuration is defined in the `MultiTrading` struct in `config/config.go`.

### General Settings

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `autoTrading` | boolean | Enables or disables automatic trading | Must be a boolean | `true` |
| `tradingLoopInterval` | duration | Time interval between trading loop iterations | Must be positive | `15s` |
| `analysisLoopInterval` | duration | Time interval between market analysis iterations | Must be positive | `20m` |
| `pendingBuyCoolDown` | duration | Cooldown period after a pending buy | Must be positive | `2m` |
| `maxDailyTrades` | integer | Maximum number of trades allowed per day | Must be >= 0 | `40` |
| `maxTotalTrades` | integer | Maximum number of trades allowed in total | Must be >= 0, must be >= `maxDailyTrades` | `40` |

### Market Thresholds

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `thresholdStopTrading` | float | Threshold at which trading stops | Must be >= 0 | `0` |
| `thresholdStartTrading` | float | Threshold at which trading starts | Must be >= 0, must be <= `thresholdStopTrading` if `thresholdStopTrading` > 0 | `0` |

### Market Filtering

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `excludedMarkets` | array of TradingPair | Markets to exclude from trading | Each trading pair must have non-empty symbol, baseAsset, and quoteAsset | `[]` |
| `excludedQuoteMarkets` | array of string | Quote assets to exclude from trading | Each string must be non-empty | `["USDT", "USDP", "FDUSD", "TUSD", "EURUSDT", "EURIUSDT"]` |
| `includedBaseMarkets` | array of string | Base assets to include in trading | Must be non-empty, each string must be non-empty | `["USDC"]` |

### Market State Strategies

Each market state has its own strategy configuration:

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `default` | MarketStateStrategy | Strategy for default market state | Required | See [Market State Strategies](#market-state-strategies) |
| `chaotic` | MarketStateStrategy | Strategy for chaotic market state | Required | See [Market State Strategies](#market-state-strategies) |
| `trending` | MarketStateStrategy | Strategy for trending market state | Required | See [Market State Strategies](#market-state-strategies) |
| `rangeBound` | MarketStateStrategy | Strategy for range-bound market state | Required | See [Market State Strategies](#market-state-strategies) |
| `transitional` | MarketStateStrategy | Strategy for transitional market state | Required | See [Market State Strategies](#market-state-strategies) |
| `stronglyTrending` | MarketStateStrategy | Strategy for strongly trending market state | Required | See [Market State Strategies](#market-state-strategies) |

## Market Analysis Configuration

The market analysis configuration is defined in the `MarketAnalyzer` struct in `analysis/marketAnalysis.go`.

### Core Indicators

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `atrPeriod` | integer | Period for Average True Range (ATR) calculation | Required | `14` |
| `adxPeriod` | integer | Period for Average Directional Index (ADX) calculation | Required | `14` |
| `highVolatilityThreshold` | float | Threshold to identify high volatility | Required | `0.035` |
| `strongTrendThreshold` | float | Threshold to identify strong trends | Required | `25` |
| `emaPeriods` | array of integer | Periods for Exponential Moving Average (EMA) calculation | Required | `[9, 21, 55]` |

### Ichimoku Cloud Settings

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `ichimokuConversionPeriod` | integer | Period for Ichimoku Conversion Line (Tenkan-sen) | Required | `9` |
| `ichimokuBasePeriod` | integer | Period for Ichimoku Base Line (Kijun-sen) | Required | `26` |
| `ichimokuSpanBPeriod` | integer | Period for Ichimoku Span B (Senkou Span B) | Required | `52` |

### Volume and Fractal Analysis

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `volumeThreshold` | float | Threshold to consider volume "significant" | Required | `15000` |
| `fractalLookback` | integer | Period used for Donchian or fractal analysis | Required | `20` |

### Optional Indicators

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `mfiPeriod` | integer | Period for Money Flow Index (MFI) calculation | If provided, must be > 0 | `14` |
| `mfiOverbought` | float | Threshold for MFI overbought condition | If provided, must be > 0 | `80` |
| `mfiOversold` | float | Threshold for MFI oversold condition | If provided, must be < 100 | `20` |
| `cciPeriod` | integer | Period for Commodity Channel Index (CCI) calculation | If provided, must be > 0 | `20` |
| `cciOverbought` | float | Threshold for CCI overbought condition | If provided, must be > 0 | `100` |
| `cciOversold` | float | Threshold for CCI oversold condition | If provided, must be < 0 | `-100` |

## Strategy Configuration

### Market State Strategies

Each market state strategy is defined in the `MarketStateStrategy` struct in `types/marketStateStrategy.go`.

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `enabled` | boolean | Enables or disables the strategy | - | Varies by market state |
| `strategy` | Strategy | The strategy to use | Required if `enabled` is true | Varies by market state |

### Compound Strategy

The compound strategy is defined in the `CompoundStrategy` struct in `strategies/compound.go`.

| Parameter | Type | Description | Validation | Default |
|-----------|------|-------------|------------|---------|
| `strategyType` | string | Type of strategy | Required | `"compound"` |
| `rsi` | RSIStrategy | Relative Strength Index (RSI) strategy | Required | See [RSI Strategy](#rsi-strategy) |
| `macd` | MACDStrategy | Moving Average Convergence Divergence (MACD) strategy | Required | See [MACD Strategy](#macd-strategy) |
| `stochastic` | StochasticOscillator | Stochastic Oscillator strategy | Required | See [Stochastic Strategy](#stochastic-strategy) |
| `bollingerBands` | BollingerBands | Bollinger Bands strategy | Required | See [Bollinger Bands Strategy](#bollinger-bands-strategy) |
| `ichimoku` | IchimokuStrategy | Ichimoku Cloud strategy | Required | See [Ichimoku Strategy](#ichimoku-strategy) |
| `cci` | CCIStrategy | Commodity Channel Index (CCI) strategy | Required | See [CCI Strategy](#cci-strategy) |
| `mfi` | MFIStrategy | Money Flow Index (MFI) strategy | Required | See [MFI Strategy](#mfi-strategy) |
| `adr` | ADRStrategy | Average Daily Range (ADR) strategy | Required | See [ADR Strategy](#adr-strategy) |
| `marketState` | MarketState | Market state for which this strategy is configured | Must be a valid market state | Varies by market state |
| `riskRewardThreshold` | float | Minimum risk/reward ratio for trades | Must be >= 0 | Varies by market state |
| `feeRate` | float | Trading fee rate | Must be >= 0 | Varies by market state |
| `desiredProfit` | float | Target profit percentage | Must be >= 0 | Varies by market state |
| `highestPriceFallOffMargin` | float | Percentage drop from highest price to trigger sell | Must be >= 0 | Varies by market state |
| `candleInterval` | string | Candle interval for analysis | Required | Varies by market state |
| `panicSell` | boolean | Enables or disables panic selling | - | Varies by market state |
| `sellOnBearish` | boolean | Enables or disables selling on bearish signals | - | Varies by market state |

#### RSI Strategy

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `period` | integer | Period for RSI calculation | `14` |
| `overbought` | integer | Threshold for overbought condition | `70` |
| `oversold` | integer | Threshold for oversold condition | `30` |

#### MACD Strategy

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `fastPeriod` | integer | Fast period for MACD calculation | `12` |
| `slowPeriod` | integer | Slow period for MACD calculation | `26` |
| `signalPeriod` | integer | Signal period for MACD calculation | `9` |

#### Stochastic Strategy

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `kPeriod` | integer | K period for Stochastic calculation | `14` |
| `dPeriod` | integer | D period for Stochastic calculation | `3` |
| `slowing` | integer | Slowing period for Stochastic calculation | `3` |
| `overbought` | integer | Threshold for overbought condition | `80` |
| `oversold` | integer | Threshold for oversold condition | `20` |

#### Bollinger Bands Strategy

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `period` | integer | Period for Bollinger Bands calculation | `20` |
| `deviations` | float | Number of standard deviations | `2.0` |

#### Ichimoku Strategy

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `conversionPeriod` | integer | Period for Conversion Line (Tenkan-sen) | `9` |
| `basePeriod` | integer | Period for Base Line (Kijun-sen) | `26` |
| `laggingSpanPeriod` | integer | Period for Lagging Span (Chikou Span) | `52` |
| `displacement` | integer | Displacement period | `26` |

#### CCI Strategy

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `period` | integer | Period for CCI calculation | `20` |
| `overbought` | float | Threshold for overbought condition | `100` |
| `oversold` | float | Threshold for oversold condition | `-100` |

#### MFI Strategy

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `period` | integer | Period for MFI calculation | `14` |
| `overbought` | integer | Threshold for overbought condition | `80` |
| `oversold` | integer | Threshold for oversold condition | `20` |

#### ADR Strategy

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `period` | integer | Period for ADR calculation | `14` |
| `multiplier` | float | Multiplier for ADR calculation | `1.0` |

## Configuration File Example

Below is an example of a configuration file in JSON format:

```json
{
  "autoTrading": true,
  "default": {
    "enabled": true,
    "strategy": {
      "strategyType": "compound",
      "rsi": {
        "period": 14,
        "overbought": 70,
        "oversold": 30
      },
      "macd": {
        "fastPeriod": 12,
        "slowPeriod": 26,
        "signalPeriod": 9
      },
      "stochastic": {
        "kPeriod": 14,
        "dPeriod": 3,
        "slowing": 3,
        "overbought": 80,
        "oversold": 20
      },
      "bollingerBands": {
        "period": 20,
        "deviations": 2.0
      },
      "ichimoku": {
        "conversionPeriod": 9,
        "basePeriod": 26,
        "laggingSpanPeriod": 52,
        "displacement": 26
      },
      "cci": {
        "period": 20,
        "overbought": 100,
        "oversold": -100
      },
      "mfi": {
        "period": 14,
        "overbought": 80,
        "oversold": 20
      },
      "adr": {
        "period": 14,
        "multiplier": 1.0
      },
      "marketState": 0,
      "riskRewardThreshold": 1.5,
      "feeRate": 0.001,
      "desiredProfit": 0.02,
      "highestPriceFallOffMargin": 0.05,
      "candleInterval": "1h",
      "panicSell": false,
      "sellOnBearish": true
    }
  },
  "chaotic": {
    "enabled": true,
    "strategy": {
      "strategyType": "compound",
      "marketState": 1,
      "riskRewardThreshold": 2.0,
      "desiredProfit": 0.03,
      "highestPriceFallOffMargin": 0.07,
      "panicSell": true
    }
  },
  "trending": {
    "enabled": true,
    "strategy": {
      "strategyType": "compound",
      "marketState": 2,
      "riskRewardThreshold": 1.2,
      "desiredProfit": 0.015
    }
  },
  "rangeBound": {
    "enabled": true,
    "strategy": {
      "strategyType": "compound",
      "marketState": 3,
      "riskRewardThreshold": 1.8,
      "desiredProfit": 0.025
    }
  },
  "transitional": {
    "enabled": true,
    "strategy": {
      "strategyType": "compound",
      "marketState": 4,
      "riskRewardThreshold": 1.5,
      "desiredProfit": 0.02
    }
  },
  "stronglyTrending": {
    "enabled": true,
    "strategy": {
      "strategyType": "compound",
      "marketState": 5,
      "riskRewardThreshold": 1.0,
      "desiredProfit": 0.01
    }
  },
  "excludedMarkets": [],
  "excludedQuoteMarkets": ["USDT", "USDP", "FDUSD", "TUSD", "EURUSDT", "EURIUSDT"],
  "includedBaseMarkets": ["USDC"],
  "tradingLoopInterval": "15s",
  "analysisLoopInterval": "20m",
  "thresholdStartTrading": 0,
  "thresholdStopTrading": 0,
  "pendingBuyCoolDown": "2m",
  "maxDailyTrades": 40,
  "maxTotalTrades": 40,
  "analyzerConfig": {
    "emaPeriods": [9, 21, 55],
    "atrPeriod": 14,
    "adxPeriod": 14,
    "highVolatilityThreshold": 0.035,
    "strongTrendThreshold": 25,
    "ichimokuConversionPeriod": 9,
    "ichimokuBasePeriod": 26,
    "ichimokuSpanBPeriod": 52,
    "volumeThreshold": 15000,
    "fractalLookback": 20,
    "mfiPeriod": 14,
    "mfiOverbought": 80,
    "mfiOversold": 20,
    "cciPeriod": 20,
    "cciOverbought": 100,
    "cciOversold": -100
  }
}
```

This configuration file can be loaded using the `MultiTradingConfigFromJSON` function in `config/config.go`.