package application

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

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

	mu          sync.RWMutex
	pending     map[int64]*pendingGroup
	window      time.Duration
	flushTicker *time.Ticker
	stopCh      chan struct{}

	sendTimeout time.Duration
}

type pendingGroup struct {
	updates  []domain.RawUpdate
	priority domain.Priority
	lastSeen time.Time
}

func NewAgentService(
	stopWords, excludedAuthors, highKeywords, lowKeywords []string,
	filterMinLength, summarizationThreshold int,
	window, sendTimeout time.Duration,
	notifier Notifier,
) *AgentService {
	s := &AgentService{
		stopWords:              stopWords,
		excludedAuthors:        excludedAuthors,
		filterMinLength:        filterMinLength,
		summarizationThreshold: summarizationThreshold,
		notifier:               notifier,
		highKeywords:           highKeywords,
		lowKeywords:            lowKeywords,
		pending:                make(map[int64]*pendingGroup),
		window:                 window,
		flushTicker:            time.NewTicker(window),
		stopCh:                 make(chan struct{}),
		sendTimeout:            sendTimeout,
	}

	go s.flushLoop()

	return s
}

func (s *AgentService) HandleRawUpdate(rawUpdate domain.RawUpdate) error {
	rawDescription := rawUpdate.Description
	processedUpdate, ok := s.filter(&rawUpdate)
	if !ok {
		slog.Debug("update filtered and will not be sent", "raw update", rawUpdate)
		return nil
	}

	processedUpdate.Priority = s.determinePriority(rawDescription)

	for _, chatID := range rawUpdate.TgChatIDs {
		s.addToGroup(chatID, rawUpdate, processedUpdate.Priority)
	}

	return nil
}

func (s *AgentService) Stop() {
	close(s.stopCh)
	s.flushTicker.Stop()

	s.mu.Lock()
	remaining := s.pending
	s.pending = make(map[int64]*pendingGroup)
	s.mu.Unlock()

	for chatID, group := range remaining {
		s.sendGroupedUpdate(chatID, group)
	}
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

func (s *AgentService) addToGroup(chatID int64, update domain.RawUpdate, priority domain.Priority) {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, exists := s.pending[chatID]
	if !exists {
		s.pending[chatID] = &pendingGroup{
			updates:  []domain.RawUpdate{update},
			priority: priority,
			lastSeen: time.Now(),
		}
		return
	}

	group.updates = append(group.updates, update)
	group.lastSeen = time.Now()

	if priority.IsHigher(group.priority) {
		group.priority = priority
	}
}

func (s *AgentService) flushLoop() {
	for {
		select {
		case <-s.flushTicker.C:
			s.flushExpiredGroups()
		case <-s.stopCh:
			return
		}
	}
}

func (s *AgentService) flushExpiredGroups() {
	s.mu.Lock()
	expired := make(map[int64]*pendingGroup)
	now := time.Now()

	for chatID, group := range s.pending {
		if now.Sub(group.lastSeen) >= s.window {
			expired[chatID] = group
			delete(s.pending, chatID)
		}
	}
	s.mu.Unlock()

	for chatID, group := range expired {
		s.sendGroupedUpdate(chatID, group)
	}
}

func (s *AgentService) sendGroupedUpdate(chatID int64, group *pendingGroup) {
	var description string

	if len(group.updates) == 1 {
		description = group.updates[0].Description
	} else {
		var sb strings.Builder
		for i, u := range group.updates {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, u.Description)
		}
		description = strings.TrimRight(sb.String(), "\n")
	}

	processed := domain.ProcessedUpdate{
		ID:          group.updates[0].ID,
		Description: description,
		TgChatIDs:   []int64{chatID},
		Priority:    group.priority,
	}

	sendCtx, cancel := context.WithTimeout(context.Background(), s.sendTimeout)
	defer cancel()

	if err := s.notifier.SendUpdate(sendCtx, processed); err != nil {
		slog.Error("failed to send grouped update", "chatID", chatID, "error", err)
	}
}
