package dnsproxy

import (
	"github.com/ray-g/dnsproxy/api"
	"github.com/ray-g/dnsproxy/blocker"
	mem "github.com/ray-g/dnsproxy/cache/memcache"
	conf "github.com/ray-g/dnsproxy/config"
	"github.com/ray-g/dnsproxy/logger"
	r "github.com/ray-g/dnsproxy/resolver"
	"github.com/ray-g/dnsproxy/stats"
)

func Serve(configPath string) {
	config, err := conf.LoadConfig(configPath)
	if err != nil {
		logger.Fatal(err)
	}

	logger.InitLogger("DNSProxy", config.DebugMode)

	cache := mem.NewCache()

	server := r.NewServer(config.DNSServer.BindAddr, r.NewHandler(&config.Resolver, cache))
	server.Run()

	blocker.PerformUpdate(&config.Blocker, cache, false)

	if config.APIServer.Enable {
		if err := api.StartAPIServer(config.APIServer.BindAddr, config.DebugMode, cache); err != nil {
			logger.Fatalf("Cannot start the API server %s", err)
		}
	}

	stats.Activate()
}
