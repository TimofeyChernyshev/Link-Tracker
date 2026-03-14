package service

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"

// Storage - контракт хранилища ссылок
type Storage interface {
	RegisterChat(chatID int64)
	DeleteChat(chatID int64)
	ChatExists(chatID int64) bool
	AddLink(chatID int64, URL string, tags []string) (domain.Link, error)
	RemoveLink(chatID int64, URL string) (domain.Link, error)
	GetLinks(chatID int64) []domain.Link
}
