package cmd

import (
	"flag"
	"fmt"
	"github.com/M1chlCZ/bingo-bot/backtest"
	"github.com/M1chlCZ/bingo-bot/logger"
	"os"
)

// HandleBacktestCommands processes backtest-related commands
func HandleBacktestCommands(args []string) bool {
	if len(args) < 2 {
		return false
	}

	command := args[1]
	switch command {
	case "fetch-data":
		// Reset flags to parse command-specific flags
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		backtest.FetchDataCmd()
		return true
	case "run-backtest":
		// Reset flags to parse command-specific flags
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		backtest.RunBacktestCmd()
		return true
	case "help":
		printBacktestHelp()
		return true
	}

	return false
}

// printBacktestHelp prints help information for backtest commands
func printBacktestHelp() {
	fmt.Println("Backtest Commands:")
	fmt.Println("  fetch-data     Fetch historical data from Binance")
	fmt.Println("    Options:")
	fmt.Println("      -symbol string      Trading pair symbol (e.g., BTCUSDT) (default \"BTCUSDT\")")
	fmt.Println("      -interval string    Candle interval (e.g., 1h, 4h, 1d) (default \"1h\")")
	fmt.Println("      -start string       Start date (YYYY-MM-DD) (default: 1 year ago)")
	fmt.Println("      -end string         End date (YYYY-MM-DD) (default: today)")
	fmt.Println("      -api-key string     Binance API key (required)")
	fmt.Println("      -api-secret string  Binance API secret (required)")
	fmt.Println()
	fmt.Println("  run-backtest   Run a backtest with historical data")
	fmt.Println("    Options:")
	fmt.Println("      -symbol string      Trading pair symbol (e.g., BTCUSDT) (default \"BTCUSDT\")")
	fmt.Println("      -interval string    Candle interval (e.g., 1h, 4h, 1d) (default \"1h\")")
	fmt.Println("      -strategy string    Strategy to use (e.g., compound) (default \"compound\")")
	fmt.Println("      -start string       Start date (YYYY-MM-DD) (default: 6 months ago)")
	fmt.Println("      -end string         End date (YYYY-MM-DD) (default: today)")
	fmt.Println("      -balance float      Initial balance (default 10000.0)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  Fetch historical data:")
	fmt.Println("    go run main.go fetch-data -api-key=YOUR_API_KEY -api-secret=YOUR_API_SECRET -symbol=BTCUSDT -interval=1h -start=2022-01-01 -end=2022-12-31")
	fmt.Println()
	fmt.Println("  Run backtest:")
	fmt.Println("    go run main.go run-backtest -symbol=BTCUSDT -interval=1h -strategy=compound -start=2022-01-01 -end=2022-12-31 -balance=10000.0")
}

// InitBacktestLogger initializes the logger for backtest commands
func InitBacktestLogger() {
	// Set up logging
	logLevel := "info"
	colorEnabled := true
	logger.InitLogger(&logLevel, &colorEnabled)
}
