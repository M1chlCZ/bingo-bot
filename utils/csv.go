package utils

import (
	"encoding/csv"
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
	"os"
	"path/filepath"
	"time"
)

func AppendMetricsToCSV(filename string, metric models.PerformanceMetrics) error {

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if stat.Size() == 0 {

		header := []string{"Timestamp", "TotalProfitLoss", "UnrealizedProfit", "UnrealizedLoss"}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %v", err)
		}
	}

	record := []string{
		metric.Timestamp.Format(time.RFC3339),
		fmt.Sprintf("%.2f", metric.TotalProfitLoss),
		fmt.Sprintf("%.2f", metric.UnrealizedProfit),
		fmt.Sprintf("%.2f", metric.UnrealizedLoss),
	}
	if err := writer.Write(record); err != nil {
		return fmt.Errorf("failed to write CSV record: %v", err)
	}

	return nil
}
