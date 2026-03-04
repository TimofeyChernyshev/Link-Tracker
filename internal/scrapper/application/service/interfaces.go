package service

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"

// Storage - контракт хранилища ссылок
type Storage interface {
	RegisterChat(chatId int64)
	DeleteChat(chatId int64)
	ChatExists(chatId int64) bool
	AddLink(chatId int64, url string, tags []string) (domain.Link, error)
	RemoveLink(chatId int64, url string) (domain.Link, error)
	GetLinks(chatId int64) []domain.Link
}
