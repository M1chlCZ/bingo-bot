# Makefile for Binance Trading Bot

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=bingo-bot
MAIN_PATH=./main.go

# Linting and formatting
GOLINT=golangci-lint
GOFMT=gofmt

.PHONY: all build clean test fmt lint tidy help backtest-help fetch-data run-backtest

all: lint fmt build

build:
	$(GOBUILD) -o $(BINARY_NAME) $(MAIN_PATH)

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

test:
	$(GOTEST) -v ./...

fmt:
	$(GOFMT) -w -s .

lint:
	$(GOLINT) run

tidy:
	$(GOMOD) tidy

# Install required tools
tools:
	@echo "Installing required tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run the application
run: build
	./$(BINARY_NAME)

help:
	@echo "Available commands:"
	@echo "  make all       - Run lint, fmt, and build"
	@echo "  make build     - Build the application"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make test      - Run tests"
	@echo "  make fmt       - Format code using gofmt"
	@echo "  make lint      - Run linters using golangci-lint"
	@echo "  make tidy      - Tidy go.mod file"
	@echo "  make tools     - Install required tools"
	@echo "  make run       - Build and run the application"
	@echo "  make help      - Show this help message"
	@echo ""
	@echo "Backtest commands:"
	@echo "  make backtest-help   - Show help for backtest commands"
	@echo "  make fetch-data      - Fetch historical data from Binance"
	@echo "  make run-backtest    - Run a backtest with historical data"

# Backtest commands
backtest-help:
	@$(GOBUILD) -o $(BINARY_NAME) $(MAIN_PATH)
	@./$(BINARY_NAME) help

fetch-data:
	@echo "Fetching historical data from Binance..."
	@echo "Usage: make fetch-data SYMBOL=BTCUSDT INTERVAL=1h START=2022-01-01 END=2022-12-31 API_KEY=your_api_key API_SECRET=your_api_secret"
	@$(GOBUILD) -o $(BINARY_NAME) $(MAIN_PATH)
	@./$(BINARY_NAME) fetch-data -symbol=$(SYMBOL) -interval=$(INTERVAL) -start=$(START) -end=$(END) -api-key=$(API_KEY) -api-secret=$(API_SECRET)

run-backtest:
	@echo "Running backtest..."
	@echo "Usage: make run-backtest SYMBOL=BTCUSDT INTERVAL=1h STRATEGY=compound START=2022-01-01 END=2022-12-31 BALANCE=10000.0"
	@$(GOBUILD) -o $(BINARY_NAME) $(MAIN_PATH)
	@./$(BINARY_NAME) run-backtest -symbol=$(SYMBOL) -interval=$(INTERVAL) -strategy=$(STRATEGY) -start=$(START) -end=$(END) -balance=$(BALANCE)
