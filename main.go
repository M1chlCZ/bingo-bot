package main

import (
	"binance_bot/bot"
	"binance_bot/client"
	sqlite "binance_bot/db"
	"binance_bot/logger"
	"binance_bot/metrics"
	"binance_bot/strategies"
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

	// Create trading strategy
	strategy := &strategies.CompoundStrategy{
		RSI: &strategies.RSIStrategy{
			Overbought: 70,
			Oversold:   35,
			Period:     14,
		},
		MACD: &strategies.MACDStrategy{
			FastPeriod:   12,
			SlowPeriod:   26,
			SignalPeriod: 9,
		},
		Stochastic: &strategies.StochasticOscillator{
			Overbought: 75,
			Oversold:   25,
			Period:     14,
		},
		FeeRate:                   0.001, // Binance's default fee rate
		DesiredProfit:             30.0,
		HighestPriceFallOffMargin: 1.5,
		CandleInterval:            "1h",
	}

	//strategy := &strategies.SpikeStrategy{
	//	VolumeThreshold: 5000,
	//}

	bt := bot.NewMultiPairTradingBot(cl, strategy)

	// Trading pairs
	pairs := []types.TradingPair{
		types.NewTradingPair("BTCUSDT"),
		types.NewTradingPair("ETHUSDT"),
		types.NewTradingPair("DOGEUSDT"),
		types.NewTradingPair("XRPUSDT"),
		types.NewTradingPair("SOLUSDT"),
		types.NewTradingPair("FTMUSDT"),
		types.NewTradingPair("ADAUSDT"),
		types.NewTradingPair("HBARUSDT"),
		types.NewTradingPair("POWRUSDT"),
		types.NewTradingPair("OGUSDT"),
		types.NewTradingPair("BNBUSDT"),
		types.NewTradingPair("CTXCUSDT"),
		types.NewTradingPair("SCRTUSDT"),
		types.NewTradingPair("XLMUSDT"),
		types.NewTradingPair("AVAXUSDT"),
		types.NewTradingPair("ALGOUSDT"),
		types.NewTradingPair("DEGOUSDT"),
		types.NewTradingPair("IOTAUSDT"),
		types.NewTradingPair("EOSUSDT"),
		types.NewTradingPair("DGBUSDT"),
		types.NewTradingPair("THETAUSDT"),
		types.NewTradingPair("HOTUSDT"),
		types.NewTradingPair("FIDAUSDT"),
		types.NewTradingPair("WLDUSDT"),
		types.NewTradingPair("LUMIAUSDT"),
		types.NewTradingPair("TRXUSDT"),
		types.NewTradingPair("SHIBUSDT"),
		types.NewTradingPair("DOTUSDT"),
		types.NewTradingPair("LTCUSDT"),
		types.NewTradingPair("ICPUSDT"),
		types.NewTradingPair("POLUSDT"),
		types.NewTradingPair("ETCUSDT"),
		types.NewTradingPair("TAOUSDT"),
		types.NewTradingPair("APTUSDT"),
		types.NewTradingPair("CRVUSDT"),
		types.NewTradingPair("ACTUSDT"),
		types.NewTradingPair("CETUSUSDT"),
		types.NewTradingPair("FILUSDT"),
		types.NewTradingPair("SUIUSDT"),
		types.NewTradingPair("ORDIUSDT"),
		types.NewTradingPair("WIFUSDT"),
		types.NewTradingPair("FLOWUSDT"),
		types.NewTradingPair("DOTUSDT"),
		types.NewTradingPair("RSRUSDT"),
		types.NewTradingPair("FUNUSDT"),
		types.NewTradingPair("VETUSDT"),
		types.NewTradingPair("KDAUSDT"),
	}

	for _, pair := range pairs {
		if err := cl.AddTradingPair(pair); err != nil {
			logger.Infof("Failed to add trading pair %s: %v", pair.Symbol, err)
		}
	}

	go metrics.MonitorPerformance(cl)

	go bt.StartTrading()
	logger.Infof("/// Starting trading bot ///")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	bt.Stop()
	log.Println("Trading bot stopped")
}
