package utils

import (
	"binance_bot/models"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AppendMetricsToCSV appends performance metrics to a CSV file.
func AppendMetricsToCSV(filename string, metric models.PerformanceMetrics) error {
	// Ensure the directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// Open the file in append mode, create it if it doesn't exist
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	// Check if the file is empty to write the header
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	// Create a new CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	if stat.Size() == 0 {
		// Write the header if the file is empty
		header := []string{"Timestamp", "TotalProfitLoss", "UnrealizedProfit", "UnrealizedLoss"}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %v", err)
		}
	}

	// Append the new data
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
