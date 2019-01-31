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

type GoalRunlevel map[string]GoalSystem

type systemRecord struct {
	Runlevel int
	Name     string
	System   GoalSystem
}

var systems = []systemRecord{}

func RegisterSystem(runlevel int, name string, system GoalSystem) {
	systems = append(systems, systemRecord{
		Runlevel: runlevel,
		Name:     name,
		System:   system,
	})
}
