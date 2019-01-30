package systems

import (
	"context"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-errors/errors"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/twitchtv/twirp"

	. "github.com/Wizcorp/goal/src/api"
	. "github.com/Wizcorp/goal/src/proto"
)

var serviceServers = make(map[string]GoalServiceServer)
var servicesRegistry = make(map[string]GoalService)
var handlers = make(map[string]GoalServiceHandler)

// RegisterService is called to register the triplet of service, controller and hooks. Hooks and services
func RegisterService(path string, server GoalServiceServer, service GoalService, hooks *GoalHooks) {
	serviceServers[path] = server
	servicesRegistry[path] = service

	serviceType := reflect.TypeOf(service)
	for i := 0; i < serviceType.NumMethod(); i++ {
		methodType := serviceType.Method(i)

		if strings.HasPrefix(methodType.Name, "Handle") {
			method := reflect.ValueOf(service).Method(i)
			registerMesssageHandler(methodType.Type, method)
		}
	}
}

func registerMesssageHandler(methodType reflect.Type, method reflect.Value) {
	messageType := methodType.In(2)
	messageInstance := reflect.New(messageType).Elem()

	// Todo: add sanity check
	fullname := proto.MessageName(messageInstance.Interface().(proto.Message))

	if handlers[fullname] == nil {
		handlers[fullname] = []reflect.Value{}
	}

	handlers[fullname] = append(handlers[fullname], method)
}

type GoalServices interface {
	GoalSystem
	ProcessJSONMessages(ctx context.Context, data []byte)
	EmitJSONMessages(ctx context.Context, messages ...proto.Message) error
	ProcessProtobufMessages(ctx context.Context, data []byte)
	EmitProtobufMessages(ctx context.Context, messages ...proto.Message) error
	ProcessMessages(ctx context.Context, envelope *GoalMessageEnvelope)
	GetServiceServers() *map[string]GoalServiceServer
	GetServices() *map[string]GoalService
	GetHandlers() *map[string]GoalServiceHandler
}

// Controller define the expected structure of controllers used to create services
type GoalService interface{}

// Service represent Twirp-compatible services
type GoalServiceServer interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type GoalServiceHandler []reflect.Value

type GoalServiceWithSetup interface {
	Setup(server GoalServer, config *GoalConfig) error
}

type GoalServiceWithTeardown interface {
	Teardown(server GoalServer, config *GoalConfig) error
}

type services struct {
	Status   Status
	Servers  *map[string]GoalServiceServer
	Services *map[string]GoalService
	Handlers *map[string]GoalServiceHandler
	Logger   GoalLogger
}

// Hooks can be used to execute logic at certain key point of a
// RPC request or message handling.
//
// See https://godoc.org/github.com/twitchtv/twirp#ServerHooks for more details
type GoalHooks = twirp.ServerHooks

type GoalServiceEmitter func(ctx context.Context, messages ...proto.Message) error

func NewControllers() *services {
	return &services{
		Status:   DownStatus,
		Handlers: &handlers,
		Servers:  &serviceServers,
		Services: &servicesRegistry,
	}
}

func (services *services) Setup(server GoalServer, config *GoalConfig) error {
	services.Logger = (*server.GetSystem("logger")).(GoalLogger)
	for name, controller := range *services.Services {
		if controller, ok := interface{}(controller).(GoalServiceWithSetup); ok {
			subconfig, err := GetSubconfig(name, config)
			if err != nil {
				return errors.Wrap(err, 0)
			}

			err = controller.Setup(server, subconfig)
			if err != nil {
				return errors.Wrap(err, 0)
			}
		}
	}

	services.Status = UpStatus

	return nil
}

func (services *services) Teardown(server GoalServer, config *GoalConfig) error {
	for name, controller := range *services.Services {
		if controller, ok := interface{}(controller).(GoalServiceWithTeardown); ok {
			subconfig, err := GetSubconfig(name, config)
			if err != nil {
				return errors.Wrap(err, 0)
			}

			err = controller.Teardown(server, subconfig)
			if err != nil {
				return errors.Wrap(err, 0)
			}
		}
	}

	services.Status = DownStatus

	return nil
}

func (services *services) GetStatus() Status {
	return services.Status
}

func (services *services) GetServiceServers() *map[string]GoalServiceServer {
	return services.Servers
}

func (services *services) GetServices() *map[string]GoalService {
	return services.Services
}

func (services *services) GetHandlers() *map[string]GoalServiceHandler {
	return services.Handlers
}

func (services *services) ProcessJSONMessages(ctx context.Context, data []byte) {
	logger := services.Logger.GetInstance()

	var envelope GoalMessageEnvelope
	err := jsonpb.UnmarshalString(string(data), &envelope)

	if err != nil {
		logger.WithFields(LogFields{
			"data": string(data),
		}).Warn("JSON envelope could not be deserialized")
		return
	}

	services.ProcessMessages(ctx, &envelope)
}

func (services *services) EmitJSONMessages(ctx context.Context, messages ...proto.Message) error {
	envelope, err := packEnvelope(messages)
	if err != nil {
		return err
	}

	marshaler := jsonpb.Marshaler{}
	conn := ctx.Value("conn").(GoalMessageStreamConnection)
	writer, err := conn.NextWriter(1)
	if err != nil {
		return err
	}

	return marshaler.Marshal(writer, envelope)
}

func (services *services) ProcessProtobufMessages(ctx context.Context, data []byte) {
	logger := services.Logger.GetInstance()

	var envelope GoalMessageEnvelope
	err := proto.Unmarshal(data, &envelope)

	if err != nil {
		logger.WithFields(LogFields{
			"data": data,
		}).Warn("Protobuf envelope could not be deserialized")
		return
	}

	services.ProcessMessages(ctx, &envelope)
}

func (services *services) EmitProtobufMessages(ctx context.Context, messages ...proto.Message) error {
	envelope, err := packEnvelope(messages)
	if err != nil {
		return err
	}

	data, err := proto.Marshal(envelope)
	if err != nil {
		return err
	}

	conn := ctx.Value("conn").(GoalMessageStreamConnection)
	return conn.WriteMessage(1, data)
}

// ProcessMessages is used to process received GoalEnvelopes
func (services *services) ProcessMessages(ctx context.Context, envelope *GoalMessageEnvelope) {
	logger := services.Logger.GetInstance()

	for _, data := range envelope.Messages {
		var message ptypes.DynamicAny
		err := ptypes.UnmarshalAny(data, &message)

		if err != nil {
			// Todo: send error to client
			logger.WithFields(LogFields{
				"message": message,
			}).Warn("Message could not be deserialized")
			continue
		}

		services.processMessage(ctx, message.Message)
	}
}

func (services *services) processMessage(ctx context.Context, message proto.Message) {
	name := proto.MessageName(message)
	hook, found := (*services.Handlers)[name]

	if !found {
		logger := services.Logger.GetInstance()
		logger.WithFields(LogFields{
			"type":    name,
			"message": message,
		}).Warnf("Not hooks are registered to process message, ignoring")
		return
	}

	for _, h := range hook {
		h.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(message),
		})
	}
}

func packEnvelope(messages []proto.Message) (*GoalMessageEnvelope, error) {
	anyMessages := []*any.Any{}

	for _, message := range messages {
		anyMessage, err := ptypes.MarshalAny(message)
		if err != nil {
			return nil, err
		}
		anyMessages = append(anyMessages, anyMessage)
	}

	envelope := &GoalMessageEnvelope{
		Messages: anyMessages,
	}

	return envelope, nil
}
