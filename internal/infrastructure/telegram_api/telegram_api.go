package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

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
