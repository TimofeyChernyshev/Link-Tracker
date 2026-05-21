package application

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type AgentService struct {
	stopWords              []string
	excludedAuthors        []string
	filterMinLength        int
	summarizationThreshold int
	notifier               Notifier

	highKeywords []string
	lowKeywords  []string
}

func NewAgentService(
	stopWords, excludedAuthors, highKeywords, lowKeywords []string,
	filterMinLength, summarizationThreshold int,
	notifier Notifier,
) *AgentService {
	return &AgentService{
		stopWords:              stopWords,
		excludedAuthors:        excludedAuthors,
		filterMinLength:        filterMinLength,
		summarizationThreshold: summarizationThreshold,
		notifier:               notifier,
		highKeywords:           highKeywords,
		lowKeywords:            lowKeywords,
	}
}

func (s *AgentService) HandleRawUpdate(ctx context.Context, rawUpdate domain.RawUpdate) error {
	rawDescription := rawUpdate.Description
	processedUpdate, ok := s.filter(&rawUpdate)
	if ok {
		processedUpdate.Priority = s.determinePriority(rawDescription)

		if err := s.notifier.SendUpdate(ctx, processedUpdate); err != nil {
			return fmt.Errorf("failed to send processed message: %w", err)
		}
	} else {
		slog.Debug("update filtered and will not be sent", "raw update", rawUpdate)
	}

	return nil
}

func (s *AgentService) filter(rawUpdate *domain.RawUpdate) (domain.ProcessedUpdate, bool) {
	if len(rawUpdate.Description) < s.filterMinLength {
		return domain.ProcessedUpdate{}, false
	}

	for _, excluded := range s.excludedAuthors {
		if strings.EqualFold(rawUpdate.Author, excluded) {
			return domain.ProcessedUpdate{}, false
		}
	}

	lowerDesc := strings.ToLower(rawUpdate.Description)
	for _, stopWord := range s.stopWords {
		if strings.Contains(lowerDesc, strings.ToLower(stopWord)) {
			return domain.ProcessedUpdate{}, false
		}
	}

	processed := domain.ProcessedUpdate{
		ID:          rawUpdate.ID,
		Description: s.summarize(rawUpdate.Description),
		TgChatIDs:   rawUpdate.TgChatIDs,
	}

	return processed, true
}

func (s *AgentService) summarize(text string) string {
	if len(text) <= s.summarizationThreshold {
		return text
	}
	return text[:s.summarizationThreshold] + "..."
}

func (s *AgentService) determinePriority(text string) domain.Priority {
	words := extractWords(strings.ToLower(text))

	for _, kw := range s.highKeywords {
		kwLower := strings.ToLower(kw)
		if slices.Contains(words, kwLower) {
			return domain.HighPriority
		}
	}

	for _, kw := range s.lowKeywords {
		kwLower := strings.ToLower(kw)
		if slices.Contains(words, kwLower) {
			return domain.LowPriority
		}
	}

	return domain.MediumPriority
}

func extractWords(text string) []string {
	// Поиск последовательности букв и цифр
	re := regexp.MustCompile(`[a-zA-Z0-9]+`)
	return re.FindAllString(text, -1)
}
