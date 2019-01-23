package systems

import (
	"log"
	"sort"

	"github.com/go-errors/errors"

	. "github.com/Wizcorp/goal/src/api"
)

type GoalServer interface {
	RegisterSystem(runlevel int, name string, system GoalSystem)
	GetSystem(name string) *GoalSystem
	Start() error
	Stop() error
}

type server struct {
	Config    *GoalConfig
	Systems   map[string]GoalSystem
	runlevels map[int]Runlevel
}

type Runlevel map[string]GoalSystem

func NewServer(config *GoalConfig) *server {
	return &server{
		Config:    config,
		Systems:   map[string]GoalSystem{},
		runlevels: map[int]Runlevel{},
	}
}

func NewTestServer() *server {
	config := NewEmptyConfig("test")
	config.Set("goal.logger.level", "panic")

	testServer := NewServer(config)
	testServer.RegisterSystem(0, "logger", NewLogger())

	return testServer
}

func (server *server) RegisterSystem(runlevel int, name string, system GoalSystem) {
	if server.Systems[name] != nil {
		log.Panicf("System %s already registered (current: %v, submitted: %v)", name, server.Systems[name], system)
	}

	server.Systems[name] = system

	if server.runlevels[runlevel] == nil {
		server.runlevels[runlevel] = Runlevel{}
	}

	server.runlevels[runlevel][name] = system
}

func (server *server) OverrideSystem(name string, system GoalSystem) {
	for _, systems := range server.runlevels {
		if systems[name] != nil {
			systems[name] = system
			server.Systems[name] = system
			return
		}
	}

	log.Panicf("Cannot override system %s since it was never registered", name)
}

func (server *server) GetRunlevels() []Runlevel {
	runlevels := []Runlevel{}
	keys := []int{}

	for key := range server.runlevels {
		keys = append(keys, key)
	}

	sort.Ints(keys)

	for _, key := range keys {
		runlevels = append(runlevels, server.runlevels[key])
	}

	return runlevels
}

func (server *server) GetSystem(name string) *GoalSystem {
	system := server.Systems[name]

	if system == nil {
		log.Panicf("System %s is not registered", name)
	}

	return &system
}

func (server *server) RegisterDefaultSystems() {
	server.RegisterSystem(0, "logger", NewLogger())
	server.RegisterSystem(1, "metrics", NewMetrics())
	server.RegisterSystem(2, "discovery", NewDiscovery())
	server.RegisterSystem(3, "cluster", NewCluster())
	server.RegisterSystem(4, "services", NewControllers())
	server.RegisterSystem(5, "http", NewHTTP())
}

func (server *server) Start() error {
	for runlevel, systems := range server.GetRunlevels() {
		err := server.setupLevel(runlevel, systems)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	logger := (*server.GetSystem("logger")).(GoalLogger).GetInstance()
	logger.Info("Goal server is up and running")

	return nil
}

func (server *server) setupLevel(level int, systems Runlevel) error {
	for name, system := range systems {
		subconfig, err := GetSubconfig(name, server.Config)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		err = system.Setup(server, subconfig)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return nil
}

func (server *server) Stop() error {
	logger := (*server.GetSystem("logger")).(GoalLogger).GetInstance()
	logger.Info("Stopping Goal server")

	runlevels := server.GetRunlevels()

	for runlevel := len(runlevels) - 1; runlevel >= 0; runlevel-- {
		systems := runlevels[runlevel]
		err := server.teardownLevel(runlevel, systems)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	logger.Info("Goal server stopped")

	return nil
}

func (server *server) teardownLevel(level int, systems Runlevel) error {
	for name, system := range systems {
		if system.GetStatus() != UpStatus {
			continue
		}

		subconfig, err := GetSubconfig(name, server.Config)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		err = system.Teardown(server, subconfig)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return nil
}
