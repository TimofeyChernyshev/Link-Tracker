package linkchecker

import (
	"context"
	"log/slog"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type LinkChecker struct {
	clinets  []Client
	notifier Notifier
	storage  Storage
}

func New(c []Client, n Notifier, s Storage) *LinkChecker {
	return &LinkChecker{
		clinets:  c,
		notifier: n,
		storage:  s,
	}
}

func (lc *LinkChecker) CheckUpdates(ctx context.Context) {
	links := lc.storage.GetAllLinks()

	for _, link := range links {
		for _, client := range lc.clinets {
			changed, desc, err := client.Check(ctx, link)
			if err != nil {
				slog.Warn("error during checking client", "client", client, "link", link, "error", err)
				continue
			}
			if !changed {
				continue
			}

			lc.storage.UpdateTimestamp(link.URL, time.Now())

			chatIDs := lc.storage.GetSubscribers(link.URL)

			_ = lc.notifier.SendUpdate(ctx, domain.LinkUpdate{
				ID:          link.ID,
				URL:         link.URL,
				Description: desc,
				ChatIDs:     chatIDs,
			})
		}
	}
}
