# Frigate Telegram Bot

This project provides a Go-based Telegram bot that integrates with Frigate (a local NVR with AI object detection) to send real-time alerts, including snapshots and animated previews, directly to your Telegram chat.

## Features

*   **Real-time Notifications:** Receive instant alerts on Telegram when motion or objects are detected by Frigate.
*   **Snapshot Alerts:** Get a quick snapshot of the detection event as soon as it starts.
*   **Animated Previews (GIFs):** Receive a short animated preview (GIF) of the event once it concludes, providing more context.
*   **Object Filtering:** Configure the bot to only send notifications for specific detected objects (e.g., `person`, `car`, `dog`).
*   **Camera Filtering:** Limit notifications to specific cameras configured in Frigate.
*   **Internationalization (i18n):** Supports multiple languages for Telegram messages (currently English and French), configurable via environment variables.
*   **Robust Image/Video Retrieval:** Implements retry mechanisms to ensure successful retrieval of snapshots and previews from Frigate's API.

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes.

### Prerequisites

Before you begin, ensure you have the following installed:

*   **Go (1.18 or higher):** [https://golang.org/doc/install](https://golang.org/doc/install)
*   **Docker & Docker Compose:** [https://docs.docker.com/get-docker/](https://docs.docker.com/get-docker/)
*   **Frigate:** A running instance of Frigate with MQTT enabled.
*   **Telegram Bot Token:** Obtain a bot token from BotFather on Telegram.
*   **Telegram Chat ID:** Get your chat ID (you can use a bot like `@userinfobot`).

### Installation

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/YOUR_USERNAME/FrigateSignalDetect.git
    cd FrigateSignalDetect
    ```

2.  **Navigate to the bot directory:**
    ```bash
    cd bot
    ```

3.  **Install Go dependencies:**
    ```bash
    go mod tidy
    ```

### Configuration

Create a `.env` file in the root directory of the project (`FrigateSignalDetect/.env`) with the following content. Replace the placeholder values with your actual credentials and settings.

```env
TELEGRAM_TOKEN=YOUR_TELEGRAM_BOT_TOKEN
TELEGRAM_CHAT_ID=YOUR_TELEGRAM_CHAT_ID

MQTT_BROKER=mqtt://YOUR_MQTT_BROKER_IP:1883
MQTT_USERNAME=YOUR_MQTT_USERNAME # Optional, if your MQTT broker requires authentication
MQTT_PASSWORD=YOUR_MQTT_PASSWORD # Optional

# The MQTT topic where Frigate publishes event updates (e.g., frigate/events or frigate/reviews)
MQTT_TOPIC=frigate/reviews 

# Optional: Filter notifications by object type (comma-separated, e.g., person,car,cat)
# MQTT_OBJECT_FILTER=person,car

FRIGATE_URL=http://YOUR_FRIGATE_IP:5000

# Optional: Filter notifications by camera name (comma-separated, e.g., front_door,backyard)
# CAMERA_LIST=front_door,backyard

# Optional: Set the bot's language (e.g., en, fr). Defaults to 'fr'.
# BOT_LANGUAGE=en
```

### Running the Bot

1.  **Build the Go application:**
    ```bash
    cd bot
    go build -o frigate_bot
    ```

2.  **Run the bot:**
    ```bash
    # From the 'bot' directory
    env $(cat ../.env | grep -v '^#' | xargs) ./frigate_bot
    ```
    *Note: The `env $(cat ../.env | grep -v '^#' | xargs)` part is a common way to load environment variables from a `.env` file in a shell. Ensure your shell supports it.*

### Docker Deployment (CI/CD)

The project includes a `Dockerfile` for building a Docker image of the bot, and a GitHub Actions workflow for automated builds and deployments.

#### Building the Docker Image Locally

```bash
# From the project root directory (FrigateSignalDetect/)
docker build -t frigate-telegram-bot -f bot/Dockerfile .
```

#### Running with Docker Compose

You can integrate the bot into your existing `docker-compose.yaml` file. Here's an example service definition:

```yaml
version: '3.8'

services:
  frigate-telegram-bot:
    build:
      context: .
      dockerfile: bot/Dockerfile
    container_name: frigate-telegram-bot
    restart: unless-stopped
    environment:
      - TELEGRAM_TOKEN=${TELEGRAM_TOKEN}
      - TELEGRAM_CHAT_ID=${TELEGRAM_CHAT_ID}
      - MQTT_BROKER=${MQTT_BROKER}
      - MQTT_USERNAME=${MQTT_USERNAME}
      - MQTT_PASSWORD=${MQTT_PASSWORD}
      - MQTT_TOPIC=${MQTT_TOPIC}
      - MQTT_OBJECT_FILTER=${MQTT_OBJECT_FILTER}
      - FRIGATE_URL=${FRIGATE_URL}
      - CAMERA_LIST=${CAMERA_LIST}
      - BOT_LANGUAGE=${BOT_LANGUAGE}
    volumes:
      - /tmp:/tmp # Required for temporary image/video files
```

## CI/CD (GitHub Actions)

This project uses GitHub Actions for continuous integration and continuous deployment. The workflow is defined in `.github/workflows/main.yml`.

### Workflow Overview

*   **Build Go Binary:** Builds the Go application for Linux, macOS, and Windows.
*   **Build & Push Docker Image:** Builds a Docker image and pushes it to GitHub Container Registry (GHCR) upon new releases or pushes to `main` branch.

### Setup for GitHub Container Registry (GHCR) Deployment

To enable Docker image pushes to GHCR, you don't need to configure any special GitHub Secrets beyond the default `GITHUB_TOKEN` which is automatically provided by GitHub Actions.

The image will be available at `ghcr.io/YOUR_USERNAME/YOUR_REPOSITORY_NAME:latest` (replace `YOUR_USERNAME` and `YOUR_REPOSITORY_NAME` with your actual GitHub details).

## Contributing

Contributions are welcome! Please feel free to open issues or submit pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details (if you create one).