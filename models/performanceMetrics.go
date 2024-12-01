package models

import "time"

type PerformanceMetrics struct {
	Timestamp        time.Time
	TotalProfitLoss  float64
	UnrealizedProfit float64
	UnrealizedLoss   float64
}
