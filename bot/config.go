package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime settings loaded from environment variables.
type Config struct {
	TelegramToken string
	ChatID        int64

	MQTTBroker string
	MQTTUser   string
	MQTTPass   string
	MQTTTopic  string

	FrigateURL  string
	FrigateUser string // optional, enables API authentication when set with FrigatePass
	FrigatePass string

	AllowedCameras map[string]bool // empty = all cameras allowed
	ObjectFilter   map[string]bool // empty = all objects allowed
	Lang           string
}

// LoadConfig reads and validates the configuration from the environment.
//
// Frigate and MQTT are configured by host (IP or hostname) plus an optional
// port; the bot builds the URLs itself. This lets docker-compose pass a plain
// IP that targets Frigate's internal, unauthenticated port (5000) directly.
func LoadConfig() (*Config, error) {
	c := &Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		MQTTUser:      strings.TrimSpace(os.Getenv("MQTT_USERNAME")),
		MQTTPass:      strings.TrimSpace(os.Getenv("MQTT_PASSWORD")),
		MQTTTopic:     os.Getenv("MQTT_TOPIC"),
		FrigateUser:   strings.TrimSpace(os.Getenv("FRIGATE_USERNAME")),
		FrigatePass:   os.Getenv("FRIGATE_PASSWORD"),
		Lang:          os.Getenv("BOT_LANGUAGE"),
	}
	if c.Lang == "" {
		c.Lang = "fr"
	}

	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	frigateHost := strings.TrimSpace(os.Getenv("FRIGATE_HOST"))
	mqttHost := strings.TrimSpace(os.Getenv("MQTT_HOST"))
	if c.TelegramToken == "" || chatIDStr == "" || mqttHost == "" || c.MQTTTopic == "" || frigateHost == "" {
		return nil, fmt.Errorf("missing required environment variables (need TELEGRAM_TOKEN, TELEGRAM_CHAT_ID, MQTT_HOST, MQTT_TOPIC, FRIGATE_HOST)")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid TELEGRAM_CHAT_ID: %w", err)
	}
	c.ChatID = chatID

	scheme := "http"
	if tls, _ := strconv.ParseBool(os.Getenv("FRIGATE_TLS")); tls {
		scheme = "https"
	}
	c.FrigateURL = fmt.Sprintf("%s://%s:%s", scheme, frigateHost, envOr("FRIGATE_PORT", "5000"))
	c.MQTTBroker = fmt.Sprintf("tcp://%s:%s", mqttHost, envOr("MQTT_PORT", "1883"))

	c.AllowedCameras = toSet(os.Getenv("CAMERA_LIST"))
	c.ObjectFilter = toSet(os.Getenv("MQTT_OBJECT_FILTER"))

	return c, nil
}

// envOr returns the environment variable value, or def when unset/empty.
func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// toSet turns a comma-separated list into a lookup set. Empty input yields an empty set.
func toSet(csv string) map[string]bool {
	set := map[string]bool{}
	for item := range strings.SplitSeq(csv, ",") {
		if v := strings.TrimSpace(item); v != "" {
			set[v] = true
		}
	}
	return set
}
