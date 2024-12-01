package metrics

import (
	db2 "binance_bot/db"
	"binance_bot/interfaces"
	"binance_bot/models"
	"binance_bot/utils"
	"context"
	"database/sql"
	"fmt"
	"time"

	"binance_bot/logger"
)

// calculatePerformance calculates the performance metrics by fetching current prices dynamically.
func calculatePerformance(db *sql.DB, binanceClient interfaces.ExchangeClient) (float64, float64, float64, error) {
	// Query total profit/loss from completed trades
	queryCompleted := `
		SELECT 
			SUM(profit_loss) as total_profit_loss
		FROM completed_trades`
	var totalProfitLoss float64
	err := db.QueryRow(queryCompleted).Scan(&totalProfitLoss)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("error calculating completed trades profit/loss: %v", err)
	}

	// Query active trades
	queryActive := `
		SELECT 
			symbol, buy_price, quantity 
		FROM active_trades`
	rows, err := db.Query(queryActive)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("error querying active trades: %v", err)
	}
	defer rows.Close()

	var unrealizedProfit, unrealizedLoss float64
	for rows.Next() {
		var symbol string
		var buyPrice, quantity float64

		err := rows.Scan(&symbol, &buyPrice, &quantity)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("error scanning active trades: %v", err)
		}

		// Fetch the current price dynamically
		currentPrice, err := binanceClient.GetCurrentPrice(symbol) // Assuming this function exists
		if err != nil {
			logger.Warnf("Failed to fetch current price for %s: %v", symbol, err)
			continue
		}

		// Calculate profit/loss for this trade
		diff := (currentPrice - buyPrice) * quantity
		if diff > 0 {
			unrealizedProfit += diff
		} else {
			unrealizedLoss += -diff
		}
	}

	return totalProfitLoss, unrealizedProfit, unrealizedLoss, nil
}

func MonitorPerformance(binanceClient interfaces.ExchangeClient) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Fetch performance metrics
			totalProfitLoss, unrealizedProfit, unrealizedLoss, err := calculatePerformance(db2.SQLiteDB.DB, binanceClient)
			if err != nil {
				logger.Errorf("Failed to calculate performance: %v", err)
				continue
			}

			// Create a new metric entry
			metric := models.PerformanceMetrics{
				Timestamp:        time.Now(),
				TotalProfitLoss:  totalProfitLoss,
				UnrealizedProfit: unrealizedProfit,
				UnrealizedLoss:   unrealizedLoss,
			}

			// Append to CSV file
			if err := utils.AppendMetricsToCSV("/app/data/metrics.csv", metric); err != nil {
				logger.Errorf("Failed to append metrics to CSV: %v", err)
			}

			// Log the performance summary
			logger.Infof("Performance Summary (Hourly):")
			logger.Infof("Total Profit/Loss from Completed Trades: %.2f USDT", totalProfitLoss)
			logger.Infof("Unrealized Profit from Active Trades: %.2f USDT", unrealizedProfit)
			logger.Infof("Unrealized Loss from Active Trades: %.2f USDT", unrealizedLoss)

		case <-ctx.Done():
			return
		}

	}
}
