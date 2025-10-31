package validation

import (
	"fmt"
	"regexp"
	"strings"
)

func ValidateSymbol(symbol string) error {
	if symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}

	pattern := regexp.MustCompile(`^[A-Z0-9]+$`)
	if !pattern.MatchString(symbol) {
		return fmt.Errorf("invalid symbol format: %s", symbol)
	}

	return nil
}

func ValidateInterval(interval string) error {
	if interval == "" {
		return fmt.Errorf("interval cannot be empty")
	}

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

func ValidateLimit(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("limit must be greater than 0")
	}

	if limit > 1000 {
		return fmt.Errorf("limit exceeds maximum allowed value of 1000")
	}

	return nil
}

func ValidateAsset(asset string) error {
	if asset == "" {
		return fmt.Errorf("asset cannot be empty")
	}

	pattern := regexp.MustCompile(`^[A-Z0-9]+$`)
	if !pattern.MatchString(asset) {
		return fmt.Errorf("invalid asset format: %s", asset)
	}

	return nil
}

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

func ValidateQuantity(quantity string) error {
	if quantity == "" {
		return fmt.Errorf("quantity cannot be empty")
	}

	pattern := regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)
	if !pattern.MatchString(quantity) {
		return fmt.Errorf("invalid quantity format: %s", quantity)
	}

	return nil
}

func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	if len(apiKey) < 10 {
		return fmt.Errorf("API key is too short")
	}

	return nil
}

func ValidateAPISecret(apiSecret string) error {
	if apiSecret == "" {
		return fmt.Errorf("API secret cannot be empty")
	}

	if len(apiSecret) < 10 {
		return fmt.Errorf("API secret is too short")
	}

	return nil
}
