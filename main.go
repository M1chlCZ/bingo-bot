package main

import (
	"binance_bot/bot"
	"binance_bot/client"
	sqlite "binance_bot/db"
	"binance_bot/logger"
	"binance_bot/metrics"
	"binance_bot/types"
	"flag"
	"github.com/joho/godotenv"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Set up logging
	// Define a flag for log level
	logLevel := flag.String("log", "info", "Log level: debug, info, warn, error")
	flag.Parse()
	logger.InitLogger(logLevel)

	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	if os.Getenv("BINANCE_API_KEY") == "" || os.Getenv("BINANCE_API_SECRET") == "" {
		log.Fatal("BINANCE_API_KEY or BINANCE_API_SECRET not set")
	}

	// Initialize database
	err = sqlite.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create Binance client
	cl, err := client.NewBinanceClient(
		os.Getenv("BINANCE_API_KEY"),
		os.Getenv("BINANCE_API_SECRET"),
	)

	if err != nil {
		log.Fatalf("Failed to create Binance client: %v", err)
	}

	config := types.DefaultMultiTradingConfig()
	bt := bot.NewMultiPairTradingBot(cl, &config)

	go metrics.MonitorPerformance(cl)

	go bt.StartTrading()
	logger.Infof("/// Starting trading bot ///")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	bt.Stop()
	log.Println("Trading bot stopped")
}
