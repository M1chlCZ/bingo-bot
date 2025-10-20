package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

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
)

func main() {

	if cmd.HandleBacktestCommands(os.Args) {
		return
	}

	logFlag := flag.String("log", logger.EnvOrDefault("LOG_LEVEL", "info"), "Log level")
	colorFlag := flag.Bool("color", logger.EnvOrDefaultBool("COLOR_ENABLED", true), "Enable color?")
	flag.Parse()

	if err := validation.ValidateLogLevel(*logFlag); err != nil {
		errors.LogFatal(err, "Invalid log level")
	}

	logger.InitLogger(logFlag, colorFlag)

	if err := audit.InitAuditLogger(); err != nil {
		errors.LogFatal(err, "Failed to initialize audit logger")
	}

	err := godotenv.Load()
	if err != nil {
		errors.LogFatal(err, "Error loading .env file")
	}

	apiKey := os.Getenv("BINANCE_API_KEY")
	apiSecret := os.Getenv("BINANCE_API_SECRET")

	if err := validation.ValidateAPIKey(apiKey); err != nil {
		errors.LogFatal(err, "Invalid BINANCE_API_KEY")
	}
	if err := validation.ValidateAPISecret(apiSecret); err != nil {
		errors.LogFatal(err, "Invalid BINANCE_API_SECRET")
	}

	err = sqlite.InitDB()
	if err != nil {
		errors.LogFatal(err, "Failed to initialize database")
	}
	err = sqlite.SQLiteDB.RenameAllSymbolsUSDTtoUSDC()
	if err != nil {
		errors.LogFatal(err, "Failed to rename symbols in database")
	}

	err = client.InitSecureCredentials(
		os.Getenv("BINANCE_API_KEY"),
		os.Getenv("BINANCE_API_SECRET"),
	)
	if err != nil {
		errors.LogFatal(err, "Failed to initialize secure credentials")
	}

	cl, err := client.NewSecureBinanceClient()

	if err != nil {
		errors.LogFatal(err, "Failed to create Binance client")
	}

	conf := config.DefaultMultiTradingConfig()

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
