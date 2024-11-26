package main

import (
	"binance_bot/bot"
	"binance_bot/client"
	sqlite "binance_bot/db"
	"binance_bot/models"
	"binance_bot/strategies"
	"binance_bot/utils"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

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
		FeeRate: 0.001,
	}

	//strategy := &strategies.SpikeStrategy{
	//	VolumeThreshold: 5000,
	//}

	bt := bot.NewMultiPairTradingBot(cl, strategy, "15m")

	// Trading pairs
	pairs := []models.TradingPair{
		{"BTCUSDT", "BTC", "USDT", 50, 100, 2, 6},
		{"ETHUSDT", "ETH", "USDT", 50, 20, 2, 5},
		{"DOGEUSDT", "DOGE", "USDT", 50, 10, 5, 0},
		{"XRPUSDT", "XRP", "USDT", 50, 10, 5, 1},
		{"SOLUSDT", "SOL", "USDT", 50, 10, 2, 3},
		{"FTMUSDT", "FTM", "USDT", 50, 10, 4, 1},
		{"ADAUSDT", "ADA", "USDT", 50, 10, 4, 1},
		{"HBARUSDT", "HBAR", "USDT", 50, 10, 4, 1},
		{"POWRUSDT", "POWR", "USDT", 50, 10, 4, 1},
		{"OGUSDT", "OG", "USDT", 50, 10, 4, 1},
		{"BNBUSDT", "BNB", "USDT", 50, 10, 4, 1},
		{"CTXCUSDT", "CTXC", "USDT", 50, 10, 4, 1},
		{"SCRTUSDT", "SCRT", "USDT", 50, 10, 4, 1},
		{"XLMUSDT", "XLM", "USDT", 50, 10, 4, 1},
		{"AVAXUSDT", "AVAX", "USDT", 50, 10, 4, 1},
		{"ALGOUSDT", "ALGO", "USDT", 50, 10, 4, 1},
	}

	fmt.Println("/// Starting trading bot ///")

	for _, pair := range pairs {
		if err := cl.AddTradingPair(pair); err != nil {
			log.Printf("Failed to add trading pair %s: %v", pair.Symbol, err)
		}
	}

	go utils.MonitorPerformance()

	go bt.StartTrading()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	bt.Stop()
	log.Println("Trading bot stopped")
}
