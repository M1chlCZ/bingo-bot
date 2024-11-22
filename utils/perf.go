package utils

import (
	db2 "binance_bot/db"
	"database/sql"
	"log"
	"time"
)

func calculatePerformance(db *sql.DB) (float64, float64, error) {
	query := `
    SELECT 
        side, SUM(amount * price) as total_value 
    FROM trades 
    GROUP BY side`

	rows, err := db.Query(query)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var totalBuy, totalSell float64
	for rows.Next() {
		var side string
		var totalValue float64
		err := rows.Scan(&side, &totalValue)
		if err != nil {
			return 0, 0, err
		}
		if side == "BUY" {
			totalBuy += totalValue
		} else if side == "SELL" {
			totalSell += totalValue
		}
	}

	return totalSell, totalBuy, nil
}

func MonitorPerformance() {
	for {
		time.Sleep(1 * time.Hour)
		totalSell, totalBuy, err := calculatePerformance(db2.SQLiteDB.DB)
		if err != nil {
			log.Printf("Failed to calculate performance: %v", err)
			continue
		}
		profitLoss := totalSell - totalBuy
		log.Printf("Performance: Total Buy = %.2f, Total Sell = %.2f, P/L = %.2f", totalBuy, totalSell, profitLoss)
	}
}
