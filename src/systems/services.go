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

var services = make(GoalServices)
var handlers = make(GoalHandlers)

// RegisterService is called to register the triplet of service, controller and hooks. Hooks and services
func RegisterService(path string, service GoalService, controller GoalController, hooks *GoalHooks) {
	services[path] = service

	controllerType := reflect.TypeOf(controller)
	for i := 0; i < controllerType.NumMethod(); i++ {
		methodType := controllerType.Method(i)

		if strings.HasPrefix(methodType.Name, "Handle") {
			method := reflect.ValueOf(controller).Method(i)
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

type GoalControllers interface {
	GoalSystem
	ProcessJSONMessages(ctx context.Context, data []byte)
	EmitJSONMessages(ctx context.Context, messages ...proto.Message) error
	ProcessProtobufMessages(ctx context.Context, data []byte)
	EmitProtobufMessages(ctx context.Context, messages ...proto.Message) error
	ProcessMessages(ctx context.Context, envelope *GoalMessageEnvelope)
	GetServices() *GoalServices
	GetHandlers() *GoalHandlers
}

// Controller define the expected structure of controllers used to create services
type GoalController interface{}

type GoalControllerWithSetup interface {
	Setup(server GoalServer, config *GoalConfig) error
}

type GoalControllerWithTeardown interface {
	Teardown(server GoalServer, config *GoalConfig) error
}

type controllers struct {
	Status   Status
	Services *GoalServices
	Handlers *GoalHandlers
	Logger   GoalLogger
}

// Hooks can be used to execute logic at certain key point of a
// RPC request or message handling.
//
// See https://godoc.org/github.com/twitchtv/twirp#ServerHooks for more details
type GoalHooks = twirp.ServerHooks

type GoalHandlers map[string][]reflect.Value

type GoalServices map[string]GoalService

type GoalServiceEmitter func(ctx context.Context, messages ...proto.Message) error

// Service represent Twirp-compatible services
type GoalService interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

func NewControllers() *controllers {
	return &controllers{
		Status:   DownStatus,
		Handlers: &handlers,
		Services: &services,
	}
}

func (controllers *controllers) Setup(server GoalServer, config *GoalConfig) error {
	controllers.Logger = (*server.GetSystem("logger")).(GoalLogger)
	for name, controller := range *controllers.Handlers {
		if controller, ok := interface{}(controller).(GoalControllerWithSetup); ok {
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

	controllers.Status = UpStatus

	return nil
}

func (controllers *controllers) Teardown(server GoalServer, config *GoalConfig) error {
	for name, controller := range *controllers.Handlers {
		if controller, ok := interface{}(controller).(GoalControllerWithTeardown); ok {
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

	controllers.Status = DownStatus

	return nil
}

func (controllers *controllers) GetStatus() Status {
	return controllers.Status
}

func (controllers *controllers) GetServices() *GoalServices {
	return controllers.Services
}

func (controllers *controllers) GetHandlers() *GoalHandlers {
	return controllers.Handlers
}

func (controllers *controllers) ProcessJSONMessages(ctx context.Context, data []byte) {
	logger := controllers.Logger.GetInstance()

	var envelope GoalMessageEnvelope
	err := jsonpb.UnmarshalString(string(data), &envelope)

	if err != nil {
		logger.WithFields(LogFields{
			"data": string(data),
		}).Warn("JSON envelope could not be deserialized")
		return
	}

	controllers.ProcessMessages(ctx, &envelope)
}

func (controllers *controllers) EmitJSONMessages(ctx context.Context, messages ...proto.Message) error {
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

func (controllers *controllers) ProcessProtobufMessages(ctx context.Context, data []byte) {
	logger := controllers.Logger.GetInstance()

	var envelope GoalMessageEnvelope
	err := proto.Unmarshal(data, &envelope)

	if err != nil {
		logger.WithFields(LogFields{
			"data": data,
		}).Warn("Protobuf envelope could not be deserialized")
		return
	}

	controllers.ProcessMessages(ctx, &envelope)
}

func (controllers *controllers) EmitProtobufMessages(ctx context.Context, messages ...proto.Message) error {
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
func (controllers *controllers) ProcessMessages(ctx context.Context, envelope *GoalMessageEnvelope) {
	logger := controllers.Logger.GetInstance()

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

		controllers.processMessage(ctx, message.Message)
	}
}

func (controllers *controllers) processMessage(ctx context.Context, message proto.Message) {
	name := proto.MessageName(message)
	hook, found := (*controllers.Handlers)[name]

	if !found {
		logger := controllers.Logger.GetInstance()
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
