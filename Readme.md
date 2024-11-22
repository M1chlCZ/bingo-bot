
---

# Bingo-Bot 🚀
Bingo-Bot (Binance GO Bot) is a **work-in-progress**, **experimental trading bot** for Binance, built with **Go** and designed to be flexible, extensible, and easy to use. Whether you're testing strategies or building a robust trading system, Bingo-Bot is here to help!

---

## ⚡ Features
- **Automated Trading**: Works with Binance for spot trading.
- **Custom Strategies**: Easily implement your own strategies in the `./strategies/` folder.
- **Pluggable Exchanges**: Add other exchanges by adhering to the shared interface in `./interfaces/shared.go`.
- **Stop-Loss and Take-Profit**: Dynamic risk management for trades.
- **Trend Filtering**: Combines indicators like RSI and MACD for smarter trades.
- **Docker Support**: Deploy quickly with Docker Compose.
- **SQLite Logging**: Tracks your trades for performance analysis.

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
3. Register your strategy in `main.go`.

### Adding New Exchanges
To integrate a new exchange:
1. Implement the `Exchange` interface in `./interfaces/shared.go`.
2. Provide methods for fetching market data, creating orders, and managing balances.

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
- [ ] Improve logging and analytics.
- [ ] Integrate more exchanges.
- [ ] Add more strategies (Bollinger Bands, Stochastic Oscillator, etc.).

---

## 📜 License
MIT License.

---

### Happy Trading! 🚀

---