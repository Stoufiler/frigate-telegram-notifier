# --- Build stage ---
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

# Cache dependencies first for faster rebuilds.
COPY bot/go.mod bot/go.sum ./
RUN go mod download

COPY bot/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /usr/local/bin/frigate-bot .

# --- Runtime stage ---
FROM alpine:latest

# CA certificates are required for HTTPS calls to Telegram and Frigate.
RUN apk add --no-cache ca-certificates

COPY --from=builder /usr/local/bin/frigate-bot /usr/local/bin/frigate-bot

ENTRYPOINT ["/usr/local/bin/frigate-bot"]
