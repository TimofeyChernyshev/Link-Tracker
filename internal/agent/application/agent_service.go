package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type AgentService struct {
	stopWords              []string
	excludedAuthors        []string
	filterMinLength        int
	summarizationThreshold int
	notifier               Notifier
}

func NewAgentService(stopWords, excludedAuthors []string, filterMinLength, summarizationThreshold int, notifier Notifier) *AgentService {
	return &AgentService{
		stopWords:              stopWords,
		excludedAuthors:        excludedAuthors,
		filterMinLength:        filterMinLength,
		summarizationThreshold: summarizationThreshold,
		notifier:               notifier,
	}
}

func (s *AgentService) HandleRawUpdate(ctx context.Context, rawUpdate domain.RawUpdate) error {
	processedUpdate, ok := s.filter(&rawUpdate)
	if ok {
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
		Priority:    domain.HighPriority,
	}

	return processed, true
}

func (s *AgentService) summarize(text string) string {
	if len(text) <= s.summarizationThreshold {
		return text
	}
	return text[:s.summarizationThreshold] + "..."
}
