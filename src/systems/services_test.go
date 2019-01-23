package systems_test

import (
	"context"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"

	. "github.com/Wizcorp/goal/src/proto"
	. "github.com/Wizcorp/goal/src/systems"
)

type PingControllerWithoutMessages struct {
	Time int64
}

func (y *PingControllerWithoutMessages) Ping(ctx context.Context, message *GoalPingRequest) (*GoalPingResponse, error) {
	y.Time = message.Timestamp

	return &GoalPingResponse{
		Timestamp: message.Timestamp,
	}, nil
}

type PingController struct {
	Time int64
}

func (y *PingController) Ping(ctx context.Context, message *GoalPingRequest) (*GoalPingResponse, error) {
	y.Time = message.Timestamp

	return &GoalPingResponse{
		Timestamp: message.Timestamp,
	}, nil
}

func (y *PingController) HandleGoalPingRequest(ctx context.Context, message *GoalPingRequest) {
	y.Time = message.Timestamp
}

func setup(controller Ping, hooks *GoalHooks) (GoalServer, func()) {
	path := PingPathPrefix
	service := NewPingServer(controller, hooks)
	RegisterService(path, service, controller, hooks)

	server := NewTestServer()
	server.RegisterSystem(4, "controllers", NewControllers())
	server.Start()

	return server, func() {
		controllers := (*server.GetSystem("controllers")).(GoalControllers)
		services := controllers.GetServices()
		handlers := controllers.GetHandlers()

		server.Stop()

		for key, _ := range *handlers {
			delete(*handlers, key)
		}

		for key, _ := range *services {
			delete(*services, key)
		}
	}
}

func TestRegisterWithoutMessages(t *testing.T) {
	server, teardown := setup(&PingControllerWithoutMessages{}, nil)
	defer teardown()

	controllers := (*server.GetSystem("controllers")).(GoalControllers)

	if len(*controllers.GetServices()) != 1 {
		t.Errorf("Service was not registered")
	}

	if len(*controllers.GetHandlers()) != 0 {
		t.Errorf("Handler registered when no handlers are defined")
	}
}

func TestRegisterWithMessages(t *testing.T) {
	server, teardown := setup(&PingController{}, nil)
	defer teardown()

	controllers := (*server.GetSystem("controllers")).(GoalControllers)

	if len(*controllers.GetServices()) != 1 {
		t.Error("Service was not registered")
	}

	if len(*controllers.GetHandlers()) != 1 {
		t.Error("Handler was not registered")
	}
}

func TestProcessMessages(t *testing.T) {
	controller := &PingController{}
	server, teardown := setup(controller, nil)
	defer teardown()

	controllers := (*server.GetSystem("controllers")).(GoalControllers)

	message := &GoalPingRequest{
		Timestamp: 123,
	}
	data, _ := ptypes.MarshalAny(message)
	envelope := &GoalMessageEnvelope{
		Messages: []*any.Any{
			data,
		},
	}
	bytes, _ := proto.Marshal(envelope)

	ctx := context.Background()
	controllers.ProcessMessages(ctx, bytes)

	if controller.Time != message.Timestamp {
		t.Errorf("Times do not match: %d != %d", controller.Time, message.Timestamp)
	}
}
