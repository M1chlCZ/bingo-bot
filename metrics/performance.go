package metrics

import (
	"context"
	"database/sql"
	"fmt"
	db2 "github.com/M1chlCZ/bingo-bot/db"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/utils"
	"time"

	"github.com/M1chlCZ/bingo-bot/logger"
)

func calculatePerformance(db *sql.DB, binanceClient interfaces.ExchangeClient) (float64, float64, float64, error) {

	queryCompleted := `
		SELECT 
			SUM(profit_loss) as total_profit_loss
		FROM completed_trades`
	var totalProfitLoss float64
	err := db.QueryRow(queryCompleted).Scan(&totalProfitLoss)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("error calculating completed trades profit/loss: %w", err)
	}

	queryActive := `
		SELECT 
			symbol, buy_price, quantity 
		FROM active_trades`
	rows, err := db.Query(queryActive)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("error querying active trades: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Errorf("Failed to close rows: %v", err)
		}
	}(rows)

	var unrealizedProfit, unrealizedLoss float64
	for rows.Next() {
		var symbol string
		var buyPrice, quantity float64

		err := rows.Scan(&symbol, &buyPrice, &quantity)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("error scanning active trades: %w", err)
		}

		currentPrice, err := binanceClient.GetCurrentPrice(symbol) // Assuming this function exists
		if err != nil {
			logger.Warnf("Failed to fetch current price for %s: %v", symbol, err)
			continue
		}

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

			totalProfitLoss, unrealizedProfit, unrealizedLoss, err := calculatePerformance(db2.SQLiteDB.DB, binanceClient)
			if err != nil {
				logger.Errorf("Failed to calculate performance: %v", err)
				continue
			}

			metric := models.PerformanceMetrics{
				Timestamp:        time.Now(),
				TotalProfitLoss:  totalProfitLoss,
				UnrealizedProfit: unrealizedProfit,
				UnrealizedLoss:   unrealizedLoss,
			}

			if err := utils.AppendMetricsToCSV("/app/data/metrics.csv", metric); err != nil {
				logger.Errorf("Failed to append metrics to CSV: %v", err)
			}

			logger.Infof("\n")
			logger.Infof("=========================================")
			logger.Infof("Performance Summary (Hourly):")
			logger.Infof("Total Profit/Loss from Completed Trades: %.2f USDT", totalProfitLoss)
			logger.Infof("Unrealized Profit from Active Trades: %.2f USDT", unrealizedProfit)
			logger.Infof("Unrealized Loss from Active Trades: %.2f USDT", unrealizedLoss)
			logger.Infof("=========================================")
			logger.Infof("\n")
		case <-ctx.Done():
			return
		}
	}
}
