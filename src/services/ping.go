package services

import (
	"context"
	"log"

	. "github.com/Wizcorp/goal/src/proto"
	. "github.com/Wizcorp/goal/src/systems"
)

type PingService struct{}

func (y *PingService) Ping(ctx context.Context, message *GoalPingRequest) (*GoalPingResponse, error) {
	return &GoalPingResponse{
		Timestamp: message.Timestamp,
	}, nil
}

func (y *PingService) HandlePing(ctx context.Context, message *GoalPingRequest) {
	emitter := ctx.Value("emitter").(GoalServiceEmitter)
	err := emitter(ctx, &GoalPingResponse{
		Timestamp: 123,
	})

	if err != nil {
		log.Printf("%v", err)
	}
}

func init() {
	hooks := (*GoalHooks)(nil)
	service := &PingService{}
	server := NewPingServer(service, hooks)

	RegisterService(PingPathPrefix, server, service, hooks)
}
