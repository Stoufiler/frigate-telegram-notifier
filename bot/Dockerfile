# Use the official Go image as a base image
FROM golang:1.24-alpine

# Set the current working directory inside the container
WORKDIR /app

# Copy the entire bot directory into the container
COPY bot/ .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/frigate_bot .

# Start a new stage from scratch
FROM alpine:latest

WORKDIR /app

# Copy the compiled executable from the previous stage
COPY --from=0 /usr/local/bin/frigate_bot /usr/local/bin/frigate_bot

# Expose any ports if your application listens on them
# EXPOSE 8080

# Command to run the executable
ENTRYPOINT ["/usr/local/bin/frigate_bot"]