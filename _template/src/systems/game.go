package systems

import (
	. "github.com/Wizcorp/goal/_template/src/api"

	. "github.com/Wizcorp/goal/src/api"
	. "github.com/Wizcorp/goal/src/systems"
)

func init() {
	RegisterSystem(3, "game", NewGame())
}

type Game interface {
	GoalSystem
	SayHello(name string) string
}

type game struct {
	Status Status
}

func NewGame() *game {
	return &game{}
}

func (game *game) Setup(server GoalServer, config *GoalConfig) error {
	configData := config.Get("")
	logger := (*server.GetSystem("logger")).(GoalLogger).GetInstance()
	logger.WithFields(LogFields{
		"config": configData,
	}).Info("Game configuration")
	game.Status = UpStatus

	return nil
}

func (game *game) Teardown(server GoalServer, config *GoalConfig) error {
	logger := (*server.GetSystem("logger")).(GoalLogger).GetInstance()
	logger.Info("Tearing down game")
	game.Status = DownStatus

	return nil
}

func (game *game) GetStatus() Status {
	return game.Status
}

func (game *game) SayHello(name string) string {
	return Concat("Hello, " + name)
}
