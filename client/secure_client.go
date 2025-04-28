package client

import (
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/security"
	"time"
)

// NewSecureBinanceClient creates a new Binance client using securely stored credentials
func NewSecureBinanceClient() (interfaces.ExchangeClient, error) {
	// Get the secure Binance client
	binanceClient, err := security.GetSecureBinanceClient()
	if err != nil {
		return nil, err
	}

	logger.Info("Started trading using Binance with secure credentials")
	b := &BinanceClient{
		client:            binanceClient,
		pairs:             make(map[string]*models.TradingPair),
		candleCache:       make(map[string][]models.CandleStick),
		maxRequests:       1,                    // max requests
		interval:          1 * time.Millisecond, // time window
		highPriorityQueue: make(chan BinanceRequest, 200),
		normalQueue:       make(chan BinanceRequest, 600),
		workerDone:        make(chan struct{}),
	}
	// Start the worker goroutine
	b.startWorker()
	b.StartQueueMonitor()

	return b, nil
}

// InitSecureCredentials initializes the secure storage with API credentials
func InitSecureCredentials(apiKey, apiSecret string) error {
	return security.InitSecureBinanceProvider(apiKey, apiSecret)
}
