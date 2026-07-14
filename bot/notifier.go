package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Notifier sends messages, photos and animations to a fixed Telegram chat.
type Notifier struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

// NewNotifier creates a Telegram notifier for the given bot token and chat.
func NewNotifier(token string, chatID int64) (*Notifier, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Notifier{bot: bot, chatID: chatID}, nil
}

// SendText sends a plain text message.
func (n *Notifier) SendText(text string) error {
	_, err := n.bot.Send(tgbotapi.NewMessage(n.chatID, text))
	return err
}

// SendPhoto sends a local image file with a caption.
func (n *Notifier) SendPhoto(path, caption string) error {
	photo := tgbotapi.NewPhoto(n.chatID, tgbotapi.FilePath(path))
	photo.Caption = caption
	_, err := n.bot.Send(photo)
	return err
}

// SendAnimation sends a local animated file (GIF) with a caption.
func (n *Notifier) SendAnimation(path, caption string) error {
	animation := tgbotapi.NewAnimation(n.chatID, tgbotapi.FilePath(path))
	animation.Caption = caption
	_, err := n.bot.Send(animation)
	return err
}
