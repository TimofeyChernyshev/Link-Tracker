package handlers

import "time"

const (
	trackStepAwaitingLink   = 0
	trackStepAwaitingTags   = 1
	trackStepSaving         = 2
	untrackStepAwaitingLink = 0
	untrackStepRemoving     = 1
	listStepAwaitingTags    = 0
	listStepAwaitingLimit   = 1
	listStepAwaitingOffset  = 2
	listStepListing         = 3

	timeoutCheckLink = 10 * time.Second
)
