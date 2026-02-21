package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

// RealTelegramAPI реальная реализация
type RealTelegramAPI struct {
	bot *tgbotapi.BotAPI
}

func NewRealTelegramAPI(token string) (*RealTelegramAPI, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &RealTelegramAPI{bot: bot}, nil
}

func (r *RealTelegramAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return r.bot.GetUpdatesChan(config)
}

func (r *RealTelegramAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return r.bot.Send(c)
}

func (r *RealTelegramAPI) StopReceivingUpdates() {
	r.bot.StopReceivingUpdates()
}

func (r *RealTelegramAPI) Self() tgbotapi.User {
	return r.bot.Self
}

func (r *RealTelegramAPI) SetCommands(cmds []domain.BotCommand) error {
	tgCommands := []tgbotapi.BotCommand{}
	for _, c := range cmds {
		tgCommands = append(tgCommands, tgbotapi.BotCommand{Command: c.Name, Description: c.Description})
	}

	cfg := tgbotapi.NewSetMyCommands(tgCommands...)
	_, err := r.bot.Request(cfg)
	return err
}
