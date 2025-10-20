package client

import (
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/security"
	"time"
)

func NewSecureBinanceClient() (interfaces.ExchangeClient, error) {

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

	b.startWorker()
	b.StartQueueMonitor()

	return b, nil
}

func InitSecureCredentials(apiKey, apiSecret string) error {
	return security.InitSecureBinanceProvider(apiKey, apiSecret)
}
