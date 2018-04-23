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
	logger.Debug().
		Msg("DetectZombies called")

	servers, err := z.servers.List(ctx)
	if err != nil {
		return err
	}

	for _, server := range servers {
		z.wg.Add(1)
		go func(server *autoscaler.Server) {
			z.detectZombieAndDelete(ctx, server)
			z.wg.Done()
		}(server)
	}
	return nil
}

func (z *zombie) detectZombieAndDelete(ctx context.Context, instance *autoscaler.Server) error {
	logger := log.Ctx(ctx).With().
		Str("ip", instance.Address).
		Str("name", instance.Name).
		Logger()

	_, err := z.client(instance)
	if err == nil {
		return nil // we were able to connect to the docker instance. not a zombie
	}

	// check if the agent is older than 10 minutes. if so schedule for removal
	if time.Now().Before(time.Unix(instance.Created, 0).Add(z.minAge)) {
		instance.State = autoscaler.StateShutdown
		err := z.servers.Update(ctx, instance)
		if err != nil {
			logger.Error().
				Err(err).
				Str("server", instance.Name).
				Str("state", "shutdown").
				Msg("cannot update server state")
			return nil
		}
		logger.Info().
			Str("server", instance.Name).
			Msg("Zombie detected. Set to state shutdown so it will get deleted")
	}
	return nil
}
