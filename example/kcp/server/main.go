package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-net/example/service"
	"github.com/go-pantheon/fabrica-net/kcp/frame"
	kcp "github.com/go-pantheon/fabrica-net/kcp/server"
	"github.com/go-pantheon/fabrica-net/server"
)

var (
	config = conf.MOBAConfig()
)

func main() {
	// config.KCP.Smux = false

	if err := frame.InitMOBARingPool(); err != nil {
		log.Errorf("failed to initialize MOBA ring pool: %+v", err)
	}

	svc := service.New()

	svr, err := kcp.NewServer("0.0.0.0:17201", svc,
		server.WithConf(config),
	)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svr.Start(ctx); err != nil {
		panic(err)
	}

	defer func() {
		if err := svr.Stop(ctx); err != nil {
			log.Errorf("stop server failed. %+v", err)
		}
	}()

	log.Infof("KCP server started with gaming-optimized configuration")

	// Print performance statistics periodically
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Print codec and pool statistics for monitoring
				codecStats := frame.GetCodecStats()
				poolStats := frame.GetPoolStats()

				log.Infof("KCP Performance - Encodes: %d, Decodes: %d, Hit Ratio: %.2f%%, Avg Packet: %.1f bytes",
					codecStats.TotalEncodes,
					codecStats.TotalDecodes,
					codecStats.HitRatio*100,
					codecStats.AvgPacketSize)

				if len(poolStats) > 0 {
					for size, stats := range poolStats {
						if stats.AllocCount > 0 {
							log.Debugf("Pool[%d]: Allocs=%d, Available=%d/%d",
								size, stats.AllocCount, stats.Available, stats.RingSize)
						}
					}
				}
			}

			// Sleep for 30 seconds between stats
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
		}
	}()

	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case <-c:
	case <-ctx.Done():
	}

	log.Infof("KCP server stopped")
}
