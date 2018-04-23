package engine

import (
	"context"
	"sync"
	"time"
	"net"

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
		Msg("detect zombies called")

	servers, err := z.servers.List(ctx)
	if err != nil {
		return err
	}

	logger.Debug().
		Int("server count", len(servers)).
		Msg("detect zombies server count")

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

	logger.Debug().
		Msg("detect zombie called")

	conn, err := net.Dial("tcp", instance.Address + "2376")
	if err == nil {
		logger.Debug().
			Msg("Instance was found alive. No zombie")
		conn.Close()
		return nil
	} 

	// Ignore StatePending and StateCreating. These fields do not have a "Created" field
	// so we cannot (yet) assert that they are of age.
	if instance.State == autoscaler.StatePending || instance.State == autoscaler.StateCreating {
		logger.Debug().
			Msg("Instance in state Pening or Creating. Not viable for for Zombie check")
		return nil
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
