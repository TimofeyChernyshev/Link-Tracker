package domain

import (
	"sort"
	"time"
)

type Event struct {
	OccurredAt  time.Time
	Description string
	Author      string
}

type Events []Event

func (e Events) Sort() {
	sort.Slice(e, func(i, j int) bool {
		return e[i].OccurredAt.Before(e[j].OccurredAt)
	})
}
