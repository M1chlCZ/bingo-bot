package main

import (
	"flag"
	"github.com/M1chlCZ/bingo-bot/audit"
	"github.com/M1chlCZ/bingo-bot/bot"
	"github.com/M1chlCZ/bingo-bot/client"
	"github.com/M1chlCZ/bingo-bot/cmd"
	"github.com/M1chlCZ/bingo-bot/config"
	sqlite "github.com/M1chlCZ/bingo-bot/db"
	"github.com/M1chlCZ/bingo-bot/errors"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/metrics"
	"github.com/M1chlCZ/bingo-bot/validation"
	"github.com/joho/godotenv"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Check if we're running a backtest command
	if cmd.HandleBacktestCommands(os.Args) {
		return
	}

	// Set up logging
	// Define a flag for log level
	logFlag := flag.String("log", logger.EnvOrDefault("LOG_LEVEL", "info"), "Log level")
	colorFlag := flag.Bool("color", logger.EnvOrDefaultBool("COLOR_ENABLED", true), "Enable color?")
	flag.Parse()

	// Validate log level
	if err := validation.ValidateLogLevel(*logFlag); err != nil {
		errors.LogFatal(err, "Invalid log level")
	}

	logger.InitLogger(logFlag, colorFlag)

	// Initialize audit logging
	if err := audit.InitAuditLogger(); err != nil {
		errors.LogFatal(err, "Failed to initialize audit logger")
	}

	err := godotenv.Load()
	if err != nil {
		errors.LogFatal(err, "Error loading .env file")
	}

	apiKey := os.Getenv("BINANCE_API_KEY")
	apiSecret := os.Getenv("BINANCE_API_SECRET")

	// Validate API credentials
	if err := validation.ValidateAPIKey(apiKey); err != nil {
		errors.LogFatal(err, "Invalid BINANCE_API_KEY")
	}
	if err := validation.ValidateAPISecret(apiSecret); err != nil {
		errors.LogFatal(err, "Invalid BINANCE_API_SECRET")
	}

	// Initialize database
	err = sqlite.InitDB()
	if err != nil {
		errors.LogFatal(err, "Failed to initialize database")
	}

	// Initialize secure credentials
	err = client.InitSecureCredentials(
		os.Getenv("BINANCE_API_KEY"),
		os.Getenv("BINANCE_API_SECRET"),
	)
	if err != nil {
		errors.LogFatal(err, "Failed to initialize secure credentials")
	}

	// Create secure Binance client
	cl, err := client.NewSecureBinanceClient()

	if err != nil {
		errors.LogFatal(err, "Failed to create Binance client")
	}

	conf := config.DefaultMultiTradingConfig()
	// Load config from JSON
	//	conf, err := config.MultiTradingConfigFromJSON("config.json")
	//if err != nil {
	//	errors.LogFatal(err, "Failed to load config")
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
