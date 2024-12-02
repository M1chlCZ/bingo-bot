

# Bingo-Bot 🚀
Bingo-Bot (Binance GO Bot) is a **work-in-progress**, **experimental trading bot** for Binance, built with **Go** and designed to be flexible, extensible, and easy to use. Whether you're testing strategies or building a robust trading system, Bingo-Bot is here to help!

![Bingo-Bot Preview](https://raw.githubusercontent.com/M1chlCZ/bingo-bot/refs/heads/main/github_assets/screenshot.png)

---

<a href="https://www.buymeacoffee.com/xDujIfYZVt" target="_blank">
    <img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me a Coffee" width="150"/>
</a>

## ⚡ Features
- **Automated Trading**: Works with Binance for spot trading. More exchanges coming soon!
- **Custom Strategies**: Easily implement your own strategies in the `./strategies/` folder.
- **Pluggable Exchanges**: Add other exchanges by adhering to the shared interface in `./interfaces/shared.go`.
- **Stop-Loss and Take-Profit**: Dynamic risk management for trades.
- **Multi-Pair Trading**: Manage multiple trading pairs with thread-safe operations.
- **Trend Filtering**: Combines indicators like RSI and MACD for smarter trades.
- **Docker Support**: Deploy quickly with Docker Compose.
- **Performance Logging**: Tracks your trades for performance analysis.

---

## 🚀 Getting Started

### Prerequisites
- **Go** (1.23)
- **Docker** and **Docker Compose** (optional)
- A **Binance API Key and Secret**

---

### Installation
1. **Clone the Repository**
   ```bash
   git clone https://github.com/your-repo/bingo-bot.git
   cd bingo-bot
   ```

2. **Prepare Environment Variables**
    - Rename `.env.sample` to `.env`:
      ```bash
      mv .env.sample .env
      ```
    - Fill in your Binance API Key and Secret in the `.env` file:
      ```env
      BINANCE_API_KEY=your_api_key
      BINANCE_API_SECRET=your_api_secret
      ```

3. **Run with Docker Compose**:
    - Update the database volume in the `docker-compose.yml` file if needed:
      ```yaml
      volumes:
        - /path/to/local/folder:/app/sqlite_data
      ```
    - Start the bot:
      ```bash
      docker-compose up --build
      ```

4. **Run Directly with Go**:
    - Build and run:
      ```bash
      go build -o bingo-bot main.go
      ./bingo-bot
      ```
    - Or use `go run`:
      ```bash
      go run main.go
      ```

---

## ⚙️ Configuration

### Strategies

You can find the default strategies in the `./strategies/` folder. To add your own:
1. Implement a new struct that adheres to the `Strategy` interface in `./interfaces/shared.go`.
2. Add your logic for signal generation (e.g., RSI, MACD, Moving Averages).
3. The bot's trading logic manages multiple pairs using `MultiPairTradingBot`. Ensure your strategy is compatible with this multi-pair setup.

**Example**:
```go
type MyCustomStrategy struct {}

func (self *MyCustomStrategy) Calculate(candles []models.CandleStick, pair string, trend bool) (int, error) {
    // Custom logic here
    return 0, nil
}
```

### Exchanges
1. **Binance** is currently supported. More exchanges are coming soon!
2. To add a new exchange, implement the `ExchangeClient` interface in `./interfaces/shared.go`.
3. PRs are welcome for new exchange integrations.

### Adding New Exchanges
To integrate a new exchange:
1. Implement the `Exchange` interface in `./interfaces/shared.go`.
2. Provide methods for fetching market data, creating orders, and managing balances.

### Mutex and Thread Safety

The bot manages multiple trading pairs using internal thread-safe mechanisms.
- Mutexes are used to handle concurrent access to shared resources such as trading pairs and market data.
- No manual mutex handling is required for users implementing new strategies or adding pairs. The bot's `MultiPairTradingBot` handles this automatically.

For advanced users integrating new exchanges or modifying the bot, ensure proper thread safety by leveraging `sync.RWMutex` where applicable.

---

## 🌟 Example Use Cases
1. **Day Trading with RSI and MACD**:
    - Uses a combination of RSI and MACD for smarter trading decisions.
    - Stop-loss and take-profit are configured dynamically.
2. **Backtesting Strategies**:
    - Simulate trading strategies on historical data.

---

## 📂 Project Structure

```plaintext
bingo-bot/
├── bot/               # Core bot logic for trading
├── client/            # Binance API client
├── db/                # SQLite integration for logging trades
├── interfaces/        # Shared interfaces for strategies and exchanges
├── strategies/        # Default and custom trading strategies
├── logger/            # Logging
├── utils/             # Utility functions (Performance, Time, etc.)
├── main.go            # Entry point for the bot
├── Dockerfile         # Docker file for building the bot
└── docker-compose.yml # Docker Compose for easy deployment
```

---

## ⚠️ Experimental 🚨
- **Bingo-Bot is experimental and should NOT be used with real money unless fully tested!**
- Trading involves risk. Use at your own discretion.

---

## 🤝 Contributing
Contributions are **welcome and encouraged**!  
Feel free to submit **pull requests**, **bug reports**, or **feature requests**.

---

## 🔧 TODO
- [ ] Add backtesting framework.
- [x] Improve logging and analytics.
- [ ] Integrate more exchanges.
- [ ] Add more strategies (Bollinger Bands, Stochastic Oscillator, etc.).

---

## 📜 License
MIT License.

---

### Happy Trading! 🚀

---