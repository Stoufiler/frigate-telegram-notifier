package main

import (
	"context"
	"log"
	"os/signal"
	"strings"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// App wires together the configuration and the Frigate/Telegram clients.
type App struct {
	cfg      *Config
	frigate  *Frigate
	notifier *Notifier
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("⚠️ %v", err)
	}

	setupI18n(cfg.Lang)

	notifier, err := NewNotifier(cfg.TelegramToken, cfg.ChatID)
	if err != nil {
		log.Fatalf("⚠️ %v", err)
	}

	app := &App{
		cfg:      cfg,
		frigate:  NewFrigate(cfg.FrigateURL, cfg.FrigateUser, cfg.FrigatePass),
		notifier: notifier,
	}

	logf(tr("bot_ready"))
	if len(cfg.ObjectFilter) > 0 {
		logf(tr("object_filter_enabled", map[string]any{"Objects": keys(cfg.ObjectFilter)}))
	}

	// Stop cleanly on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client, err := app.connectMQTT(ctx)
	if err != nil {
		log.Fatalf("⚠️ %v", err)
	}

	if err := app.notifier.SendText(tr("telegram_start_message")); err != nil {
		logf(tr("telegram_start_failed", map[string]any{"Error": err}))
	}

	<-ctx.Done()
	client.Unsubscribe(cfg.MQTTTopic)
	client.Disconnect(250)
	logf(tr("bot_stopped"))
}

// connectMQTT connects to the broker and subscribes to the reviews topic.
func (a *App) connectMQTT(ctx context.Context) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().AddBroker(a.cfg.MQTTBroker)
	if a.cfg.MQTTUser != "" && a.cfg.MQTTPass != "" {
		opts.SetUsername(a.cfg.MQTTUser)
		opts.SetPassword(a.cfg.MQTTPass)
	}
	opts.OnConnect = func(mqtt.Client) {
		logf(tr("mqtt_connected"))
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		logf(tr("mqtt_connection_lost", map[string]any{"Error": err}))
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	handler := func(_ mqtt.Client, m mqtt.Message) {
		a.dispatch(ctx, m.Payload())
	}
	if token := client.Subscribe(a.cfg.MQTTTopic, 0, handler); token.Wait() && token.Error() != nil {
		client.Disconnect(250)
		return nil, token.Error()
	}
	return client, nil
}

// keys returns the keys of a set as a comma-separated string, for logging.
func keys(set map[string]bool) string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return strings.Join(out, ", ")
}
