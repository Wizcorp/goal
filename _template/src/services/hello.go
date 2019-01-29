package services

import (
	"context"
	"log"

	. "github.com/Wizcorp/goal/_template/src/proto"
	. "github.com/Wizcorp/goal/_template/src/systems"

	. "github.com/Wizcorp/goal/src/api"
	. "github.com/Wizcorp/goal/src/systems"
)

type HelloService struct {
	game Game
}

func (hello *HelloService) Setup(server GoalServer, config *GoalConfig) error {
	hello.game = (*server.GetSystem("game")).(Game)

	return nil
}

func (hello *HelloService) HelloWorld(ctx context.Context, message *HelloRequest) (*HelloResponse, error) {
	return &HelloResponse{
		Message: hello.game.SayHello(message.Name),
	}, nil
}

func (hello *HelloService) HandleHello(ctx context.Context, message *HelloRequest) {
	emitter := ctx.Value("emitter").(GoalServiceEmitter)
	err := emitter(ctx, &HelloResponse{
		Message: hello.game.SayHello(message.Name),
	})

	if err != nil {
		log.Printf("%v", err)
	}
}

func init() {
	hooks := (*GoalHooks)(nil)
	service := &HelloService{}
	server := NewHelloServer(service, hooks)

	RegisterService(HelloPathPrefix, server, service, hooks)
}
