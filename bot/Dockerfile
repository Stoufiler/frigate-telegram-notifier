# Use the official Go image as a base image
FROM golang:1.22-alpine

# Set the current working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY bot/go.mod bot/go.sum ./bot/

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY bot/main.go ./bot/
COPY bot/locales ./bot/locales

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/frigate_bot ./bot/main.go

# Start a new stage from scratch
FROM alpine:latest

WORKDIR /app

# Copy the compiled executable from the previous stage
COPY --from=0 /usr/local/bin/frigate_bot /usr/local/bin/frigate_bot

# Copy the locales directory
COPY --from=0 /app/bot/locales ./locales

# Expose any ports if your application listens on them
# EXPOSE 8080

# Command to run the executable
ENTRYPOINT ["/usr/local/bin/frigate_bot"]