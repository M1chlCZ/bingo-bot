import pandas as pd
import matplotlib.pyplot as plt

# Load data
df = pd.read_csv("metrics.csv", parse_dates=["Timestamp"])

# Plot
plt.figure(figsize=(10, 6))
plt.plot(df["Timestamp"], df["TotalProfitLoss"], label="Total Profit/Loss")
plt.plot(df["Timestamp"], df["UnrealizedProfit"], label="Unrealized Profit")
plt.plot(df["Timestamp"], df["UnrealizedLoss"], label="Unrealized Loss")
plt.xlabel("Time")
plt.ylabel("Value")
plt.title("Performance Metrics Over Time")
plt.legend()
plt.show()