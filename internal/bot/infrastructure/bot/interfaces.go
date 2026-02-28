package bot

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

// CommandDispatcher интерфейс для обработки команд
type CommandDispatcher interface {
	Dispatch(msg *domain.Message) (*domain.Response, error)
	GetCommands() []domain.BotCommand
}
