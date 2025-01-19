package main

import (
	"flag"
	"github.com/M1chlCZ/bingo-bot/bot"
	"github.com/M1chlCZ/bingo-bot/client"
	"github.com/M1chlCZ/bingo-bot/config"
	sqlite "github.com/M1chlCZ/bingo-bot/db"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/metrics"
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
	colorEnabled := flag.Bool("color", true, "Enable colorized output")
	flag.Parse()
	logger.InitLogger(logLevel, colorEnabled)

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

	conf := config.DefaultMultiTradingConfig()
	// Load config from JSON
	//	conf, err := config.MultiTradingConfigFromJSON("config.json")
	//if err != nil {
	//	log.Fatalf("Failed to load config: %v", err)
	//}
	//pr, err := utils.PrettyJson(conf)
	//if err == nil {
	//	fmt.Println(pr)
	//}
	bt := bot.NewMultiPairTradingBot(cl, &conf)

	go metrics.MonitorPerformance(cl)

	go bt.StartTrading()
	logger.Infof("/// Starting trading bot ///")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	bt.Stop()
	log.Println("Trading bot stopped")
}
