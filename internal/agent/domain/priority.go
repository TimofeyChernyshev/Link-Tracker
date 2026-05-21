package domain

type Priority string

const (
	HighPriority   = "HIGH"
	MediumPriority = "MEDIUM"
	LowPriority    = "LOW"
)

var (
	priorityOrder = map[Priority]int{
		HighPriority:   3,
		MediumPriority: 2,
		LowPriority:    1,
	}
)

func (p Priority) IsHigher(other Priority) bool {
	return priorityOrder[p] > priorityOrder[other]
}
