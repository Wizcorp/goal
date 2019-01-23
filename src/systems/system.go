package systems

import . "github.com/Wizcorp/goal/src/api"

type Status int

const (
	UpStatus Status = iota + 1
	DownStatus
)

type GoalSystem interface {
	Setup(server GoalServer, config *GoalConfig) error
	Teardown(server GoalServer, config *GoalConfig) error
	GetStatus() Status
}
