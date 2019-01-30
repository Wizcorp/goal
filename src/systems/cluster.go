package systems

import (
	"crypto/md5"
	"net"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/go-errors/errors"

	. "github.com/Wizcorp/goal/src/api"
)

type GoalClusterNode struct {
	ID      string
	Address string
	Remote  *actor.PID
}

type GoalCluster interface {
	GoalSystem
}

type cluster struct {
	Status  Status
	Name    string
	NodeID  string
	Address string
	Remote  remote.RemotingServer
	Nodes   map[string]GoalClusterNode
	Tracker GoalDiscoveryTracker
}

func NewCluster() *cluster {
	return &cluster{
		Status: DownStatus,
	}
}

func (cluster *cluster) Setup(server GoalServer, config *GoalConfig) error {
	var err error

	isEnabled := config.Bool("enable", false)
	if !isEnabled {
		return nil
	}

	cluster.Name = config.String("name", "goal")
	cluster.Address = config.String("address", "127.0.0.1:8081")
	cluster.NodeID, err = cluster.getClusterNodeID()
	if err != nil {
		return errors.Wrap(err, 0)
	}

	logger := (*server.GetSystem("logger")).(GoalLogger).GetInstance()
	logger.WithFields(LogFields{
		"name":    cluster.Name,
		"address": cluster.Address,
		"nodeId":  cluster.NodeID,
	}).Info("Setting up cluster system")

	remote.Start(cluster.Address)

	discovery := (*server.GetSystem("discovery")).(GoalDiscovery)
	allTag := "all"

	discovery.RegisterService(cluster.Name, cluster.NodeID, []string{
		"all",
	}, cluster.Address)

	cluster.Tracker = discovery.TrackService(cluster.Name, allTag)

	go func() {
		for {
			update := <-cluster.Tracker.UpdateChannel
			if update.Add {
				cluster.AddNode(update.Info.ID, update.Info.Address)
			}
			if update.Remove {
				cluster.RemoveNode(update.Info.ID)
			}
		}
	}()

	cluster.Status = UpStatus

	return nil
}

func (cluster *cluster) Teardown(server GoalServer, config *GoalConfig) error {
	cluster.Status = DownStatus

	cluster.Tracker.Stop()
	discovery := (*server.GetSystem("discovery")).(GoalDiscovery)
	err := discovery.DeregisterService(cluster.NodeID)
	remote.Shutdown(true)

	return errors.Wrap(err, 0)
}

func (cluster *cluster) GetStatus() Status {
	return cluster.Status
}

func (cluster *cluster) AddNode(id string, address string) {
	cluster.Nodes[id] = GoalClusterNode{
		ID:      id,
		Address: address,
		Remote:  actor.NewPID(address, "cluster"),
	}
}

func (cluster *cluster) RemoveNode(id string) {
	delete(cluster.Nodes, id)
}

func (cluster *cluster) ListNodes() map[string]GoalClusterNode {
	return cluster.Nodes
}

func (cluster *cluster) getClusterNodeID() (string, error) {
	var content []byte
	copy(content[:], cluster.Address)

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return "", err
		}

		for _, addr := range addrs {
			switch info := addr.(type) {
			case *net.IPAddr:
				content = append(content, info.IP...)
			}
		}
	}

	hash := md5.Sum(content)

	return string(hash[:]), nil
}
