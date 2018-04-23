package engine

import (
	"context"
	"sync"
	"time"

	"github.com/drone/autoscaler"

	"github.com/rs/zerolog/log"
)

type zombie struct {
	wg sync.WaitGroup

	servers  autoscaler.ServerStore
	provider autoscaler.Provider
	client   clientFunc
	minAge   time.Duration
}

func (z *zombie) DetectZombies(ctx context.Context) error {
	logger := log.Ctx(ctx)

	servers, err := i.servers.List(ctx)
	if err != nil {
		return err
	}

	for _, server := range servers {
		i.wg.Add(1)
		go func(server *autoscaler.Server) {
			z.detectIfZombie(ctx, server)
			z.wg.Done()
		}(server)
	}
}

func (z *zombie) detectIfZombie(ctx context.Context, instance *autoscaler.Server) error {
	logger := log.Ctx(ctx).With().
	Str("ip", instance.Address).
	Str("name", instance.Name).
	Logger()

	client, err := z.client(instance)
	if err == nil {
		return nil // we were able to connect to the docker instance. not a zombie
	}

	// check if the agent is older than 10 minutes. if so schedule for removal
	if time.Now().Before(time.Unix(instance.Created, 0).Add(z.minAge)) {
		server.State = autoscaler.StateShutdown
		err := p.servers.Update(ctx, server)
		if err != nil {
			logger.Error().
				Err(err).
				Str("server", server.Name).
				Str("state", "shutdown").
				Msg("cannot update server state")
		}
	}
}