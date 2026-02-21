package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

// TelegramAPI интерфейс для работы с Telegram
type TelegramAPI interface {
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	StopReceivingUpdates()
	Self() tgbotapi.User
	SetCommands(cmds []domain.BotCommand) error
}

// CommandDispatcher интерфейс для обработки команд
type CommandDispatcher interface {
	Dispatch(msg *domain.Message) (*domain.Response, error)
	GetCommands() []domain.BotCommand
}
