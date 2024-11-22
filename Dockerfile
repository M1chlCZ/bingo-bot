# Use the official Golang image as a base
FROM golang:latest

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the application
RUN go build -o trading-bot .

LABEL authors="m1chl"

# Command to run the bot
CMD ["./trading-bot"]


