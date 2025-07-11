package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-pantheon/fabrica-net/example/service"
	ws "github.com/go-pantheon/fabrica-net/websocket/server"
)

func main() {
	svc := service.New()

	svr, err := ws.NewServer(":8080", "/ws", svc)
	if err != nil {
		log.Errorf("failed to create server: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svr.Start(ctx); err != nil {
		log.Errorf("failed to start server: %v", err)
		return
	}

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	log.Info("shutting down server...")

	if err := svr.Stop(ctx); err != nil {
		log.Errorf("failed to stop server: %v", err)
	}
}
