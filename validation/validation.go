package validation

import (
	"fmt"
	"regexp"
	"strings"
)

// Common validation functions for external data

// ValidateSymbol checks if a trading symbol is valid
func ValidateSymbol(symbol string) error {
	if symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}

	// Most trading symbols follow the pattern BASE-QUOTE (e.g., BTCUSDT)
	// They typically consist of uppercase letters
	pattern := regexp.MustCompile(`^[A-Z0-9]+$`)
	if !pattern.MatchString(symbol) {
		return fmt.Errorf("invalid symbol format: %s", symbol)
	}

	return nil
}

// ValidateInterval checks if a candle interval is valid
func ValidateInterval(interval string) error {
	if interval == "" {
		return fmt.Errorf("interval cannot be empty")
	}

	// Valid intervals: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M
	validIntervals := map[string]bool{
		"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
		"1h": true, "2h": true, "4h": true, "6h": true, "8h": true, "12h": true,
		"1d": true, "3d": true, "1w": true, "1M": true,
	}

	if !validIntervals[interval] {
		return fmt.Errorf("invalid interval: %s", interval)
	}

	return nil
}

// ValidateLimit checks if a limit value is valid
func ValidateLimit(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("limit must be greater than 0")
	}

	// Binance typically has a maximum limit of 1000 for klines
	if limit > 1000 {
		return fmt.Errorf("limit exceeds maximum allowed value of 1000")
	}

	return nil
}

// ValidateAsset checks if an asset name is valid
func ValidateAsset(asset string) error {
	if asset == "" {
		return fmt.Errorf("asset cannot be empty")
	}

	// Asset names typically consist of uppercase letters
	pattern := regexp.MustCompile(`^[A-Z0-9]+$`)
	if !pattern.MatchString(asset) {
		return fmt.Errorf("invalid asset format: %s", asset)
	}

	return nil
}

// ValidateOrderSide checks if an order side is valid
func ValidateOrderSide(side string) error {
	if side == "" {
		return fmt.Errorf("order side cannot be empty")
	}

	side = strings.ToUpper(side)
	if side != "BUY" && side != "SELL" {
		return fmt.Errorf("invalid order side: %s (must be BUY or SELL)", side)
	}

	return nil
}

// ValidateQuantity checks if a quantity string is valid
func ValidateQuantity(quantity string) error {
	if quantity == "" {
		return fmt.Errorf("quantity cannot be empty")
	}

	// Quantity should be a valid decimal number
	pattern := regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)
	if !pattern.MatchString(quantity) {
		return fmt.Errorf("invalid quantity format: %s", quantity)
	}

	return nil
}

// ValidateAPIKey checks if an API key is valid
func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// API keys are typically long alphanumeric strings
	if len(apiKey) < 10 {
		return fmt.Errorf("API key is too short")
	}

	return nil
}

// ValidateAPISecret checks if an API secret is valid
func ValidateAPISecret(apiSecret string) error {
	if apiSecret == "" {
		return fmt.Errorf("API secret cannot be empty")
	}

	// API secrets are typically long alphanumeric strings
	if len(apiSecret) < 10 {
		return fmt.Errorf("API secret is too short")
	}

	return nil
}