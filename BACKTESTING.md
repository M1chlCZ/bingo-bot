# Backtesting Guide

This guide explains how to use the backtesting functionality to evaluate trading strategies using historical data from Binance.

## Overview

The backtesting framework allows you to:

1. Fetch historical data from Binance
2. Run backtests using the fetched data
3. Analyze the performance of trading strategies

## Prerequisites

- Binance API key and secret (for fetching historical data)
- Go installed on your system
- The Binance trading bot repository cloned to your local machine

## Fetching Historical Data

Before running a backtest, you need to fetch historical data from Binance. You can do this using the `fetch-data` command:

```bash
make fetch-data SYMBOL=BTCUSDT INTERVAL=1h START=2022-01-01 END=2022-12-31 API_KEY=your_api_key API_SECRET=your_api_secret
```

Parameters:
- `SYMBOL`: The trading pair symbol (e.g., BTCUSDT)
- `INTERVAL`: The candle interval (e.g., 1m, 5m, 15m, 1h, 4h, 1d)
- `START`: The start date in YYYY-MM-DD format
- `END`: The end date in YYYY-MM-DD format
- `API_KEY`: Your Binance API key
- `API_SECRET`: Your Binance API secret

If you don't specify a start date, it defaults to 1 year ago. If you don't specify an end date, it defaults to today.

The fetched data is stored in the `data` directory in JSON format.

## Running a Backtest

After fetching historical data, you can run a backtest using the `run-backtest` command:

```bash
make run-backtest SYMBOL=BTCUSDT INTERVAL=1h STRATEGY=compound START=2022-01-01 END=2022-12-31 BALANCE=10000.0
```

Parameters:
- `SYMBOL`: The trading pair symbol (e.g., BTCUSDT)
- `INTERVAL`: The candle interval (e.g., 1m, 5m, 15m, 1h, 4h, 1d)
- `STRATEGY`: The strategy to use (currently only 'compound' is supported)
- `START`: The start date in YYYY-MM-DD format
- `END`: The end date in YYYY-MM-DD format
- `BALANCE`: The initial balance for the backtest

If you don't specify a start date, it defaults to 6 months ago. If you don't specify an end date, it defaults to today.

## Backtest Results

The backtest results include:

- Starting and ending balance
- Total return percentage
- Number of trades (total, winning, losing, break-even)
- Win rate
- Average profit and loss
- Largest profit and loss
- Profit factor
- Final balances for each asset
- Transaction summary

## Example Workflow

1. Fetch historical data for BTCUSDT with 1-hour candles for the year 2022:

```bash
make fetch-data SYMBOL=BTCUSDT INTERVAL=1h START=2022-01-01 END=2022-12-31 API_KEY=your_api_key API_SECRET=your_api_secret
```

2. Run a backtest using the compound strategy:

```bash
make run-backtest SYMBOL=BTCUSDT INTERVAL=1h STRATEGY=compound START=2022-01-01 END=2022-12-31 BALANCE=10000.0
```

3. Analyze the results to evaluate the strategy's performance.

## Adding New Strategies

To add a new strategy for backtesting, you need to:

1. Implement the strategy in the `strategies` package
2. Update the `RunBacktestCmd` function in `backtest/cmd.go` to include your new strategy

## Limitations

- The backtesting framework uses historical data and may not perfectly simulate real-world trading conditions
- Transaction fees are simulated but may not exactly match actual exchange fees
- Market impact (slippage) is not simulated
- The framework assumes you can always execute trades at the close price of each candle