package systems

import (
	"context"
	"time"

	"github.com/go-errors/errors"
	consul "github.com/hashicorp/consul/api"

	. "github.com/Wizcorp/goal/src/api"
)

type GoalDiscovery interface {
	GoalSystem
	RegisterService(name string, id string, tags []string, address string) error
	DeregisterService(id string) error
	TrackService(name string, tag string) GoalDiscoveryTracker
}

type discovery struct {
	Status Status
	Consul *consul.Client
	Logger GoalLogger
}

type GoalDiscoveryUpdate struct {
	Add    bool
	Remove bool
	Info   *consul.CatalogService
}

type GoalDiscoveryTracker struct {
	UpdateChannel chan GoalDiscoveryUpdate
	Stop          func()
}

func NewDiscovery() *discovery {
	return &discovery{
		Status: DownStatus,
	}
}

func (discovery *discovery) Setup(server GoalServer, config *GoalConfig) error {
	isEnabled := config.Bool("enable", false)
	if !isEnabled {
		return nil
	}

	consulConfig := consul.DefaultConfig()
	consulConfig.Address = config.String("address", "127.0.0.1:8500")
	consulConfig.Scheme = config.String("scheme", "http")

	discovery.Logger = (*server.GetSystem("logger")).(GoalLogger)
	discovery.Logger.GetInstance().WithFields(LogFields{
		"address": consulConfig.Address,
		"scheme":  consulConfig.Scheme,
	}).Info("Setting up discovery system")

	consul, err := consul.NewClient(consulConfig)
	discovery.Consul = consul

	if err != nil {
		return errors.Wrap(err, 0)
	}

	discovery.Status = UpStatus

	return nil
}

func (discovery *discovery) Teardown(server GoalServer, config *GoalConfig) error {
	logger := (*server.GetSystem("logger")).(GoalLogger).GetInstance()
	logger.Info("Tearing down discovery system")
	discovery.Status = DownStatus

	return nil
}

func (discovery *discovery) GetStatus() Status {
	return UpStatus
}

func (discovery *discovery) RegisterService(name string, id string, tags []string, address string) error {
	service := &consul.AgentServiceRegistration{
		ID:   id,
		Name: name,
		Tags: tags,
		Check: &consul.AgentServiceCheck{
			TTL: (10 * time.Second).String(),
		},
	}

	err := discovery.Consul.Agent().ServiceRegister(service)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (discovery *discovery) DeregisterService(id string) error {
	return discovery.Consul.Agent().ServiceDeregister(id)
}

func (discovery *discovery) TrackService(name string, tag string) GoalDiscoveryTracker {
	stop := false
	ctx, cancel := context.WithCancel(context.Background())
	updateChannel := make(chan GoalDiscoveryUpdate)

	go func() {
		logger := discovery.Logger.GetInstance()

		if stop {
			return
		}

		knownInstances := []*consul.CatalogService{}
		for {
			ctx, cancel = context.WithCancel(context.Background())
			opts := &consul.QueryOptions{
				RequireConsistent: true,
			}
			opts.WithContext(ctx)
			newInstances, _, err := discovery.Consul.Catalog().Service(name, "all", opts)

			if err != nil {
				logger.WithFields(LogFields{
					"service": name,
					"tag":     tag,
					"error":   err,
				}).Error("Error during discovery")
			}

			for _, knownInstance := range findMissingEntriesFrom(knownInstances, newInstances) {
				updateChannel <- GoalDiscoveryUpdate{
					Remove: true,
					Info:   knownInstance,
				}
			}

			for _, newInstance := range findMissingEntriesFrom(newInstances, knownInstances) {
				updateChannel <- GoalDiscoveryUpdate{
					Add:  true,
					Info: newInstance,
				}
			}

			knownInstances = newInstances
		}
	}()

	return GoalDiscoveryTracker{
		UpdateChannel: updateChannel,
		Stop: func() {
			cancel()
		},
	}
}

func findMissingEntriesFrom(source []*consul.CatalogService, compare []*consul.CatalogService) []*consul.CatalogService {
	missing := []*consul.CatalogService{}
	for _, newInstance := range compare {
		for _, knownInstance := range source {
			found := false
			if knownInstance.Address == newInstance.Address {
				found = true
				break
			}

			if !found {
				missing = append(missing, newInstance)
			}
		}
	}

	return missing
}
