package service

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"

type Bot interface {
	SendMessage(response *domain.Response)
}
