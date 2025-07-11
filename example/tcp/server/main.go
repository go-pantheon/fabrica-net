package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/example/service"
	tcp "github.com/go-pantheon/fabrica-net/tcp/server"
)

func main() {
	svc := service.New()

	svr, err := tcp.NewServer("0.0.0.0:17101", svc)
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

	log.Infof("server started")

	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case <-c:
	case <-ctx.Done():
	}

	log.Infof("server stopped")
}
