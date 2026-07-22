package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsHigher(t *testing.T) {
	tests := []struct {
		name             string
		priority1        Priority
		priority2        Priority
		expectedIsHigher bool
	}{
		{
			name:             "same",
			priority1:        HighPriority,
			priority2:        HighPriority,
			expectedIsHigher: false,
		},
		{
			name:             "high > low",
			priority1:        HighPriority,
			priority2:        LowPriority,
			expectedIsHigher: true,
		},
		{
			name:             "medium > low",
			priority1:        MediumPriority,
			priority2:        LowPriority,
			expectedIsHigher: true,
		},
		{
			name:             "medium < high",
			priority1:        MediumPriority,
			priority2:        HighPriority,
			expectedIsHigher: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isHigher := tt.priority1.IsHigher(tt.priority2)
			assert.Equal(t, tt.expectedIsHigher, isHigher)
		})
	}
}
