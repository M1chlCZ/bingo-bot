package main

import (
	"binance_bot/bot"
	"binance_bot/client"
	sqlite "binance_bot/db"
	"binance_bot/logger"
	"binance_bot/models"
	"binance_bot/strategies"
	"binance_bot/utils"
	"flag"
	"fmt"
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
		fmt.Println("Error loading .env file")
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

	// Create trading strategy
	strategy := &strategies.RSIMACDStrategy{
		RSI: &strategies.RSIStrategy{
			Overbought: 65,
			Oversold:   40,
			Period:     18,
		},
		MACD: &strategies.MACDStrategy{
			FastPeriod:   15, // Short-term EMA
			SlowPeriod:   30, // Long-term EMA
			SignalPeriod: 10, // Signal line EMA
		},
		FeeRate:                   0.001,
		DesiredProfit:             20.0,
		HighestPriceFallOffMargin: 5.0,
	}

	//strategy := &strategies.SpikeStrategy{
	//	VolumeThreshold: 5000,
	//}

	bt := bot.NewMultiPairTradingBot(cl, strategy, "15m")

	// Trading pairs
	pairs := []models.TradingPair{
		models.NewTradingPair("BTCUSDT"),
		models.NewTradingPair("ETHUSDT"),
		models.NewTradingPair("DOGEUSDT"),
		models.NewTradingPair("XRPUSDT"),
		models.NewTradingPair("SOLUSDT"),
		models.NewTradingPair("FTMUSDT"),
		models.NewTradingPair("ADAUSDT"),
		models.NewTradingPair("HBARUSDT"),
		models.NewTradingPair("POWRUSDT"),
		models.NewTradingPair("OGUSDT"),
		models.NewTradingPair("BNBUSDT"),
		models.NewTradingPair("CTXCUSDT"),
		models.NewTradingPair("SCRTUSDT"),
		models.NewTradingPair("XLMUSDT"),
		models.NewTradingPair("AVAXUSDT"),
		models.NewTradingPair("ALGOUSDT"),
	}

	for _, pair := range pairs {
		if err := cl.AddTradingPair(pair); err != nil {
			logger.Infof("Failed to add trading pair %s: %v", pair.Symbol, err)
		}
	}

	go utils.MonitorPerformance(cl)

	go bt.StartTrading()
	logger.Infof("/// Starting trading bot ///")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	bt.Stop()
	log.Println("Trading bot stopped")
}
