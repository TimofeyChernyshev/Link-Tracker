package domain

type Priority string

const (
	HighPriority   = "HIGH"
	MediumPriority = "MEDIUM"
	LowPriority    = "LOW"

	HighPriorityLevel   = 3
	MediumPriorityLevel = 2
	LowPriorityLevel    = 1
)

var (
	priorityOrder = map[Priority]int{
		HighPriority:   HighPriorityLevel,
		MediumPriority: MediumPriorityLevel,
		LowPriority:    LowPriorityLevel,
	}
)

func (p Priority) IsHigher(other Priority) bool {
	return priorityOrder[p] > priorityOrder[other]
}
