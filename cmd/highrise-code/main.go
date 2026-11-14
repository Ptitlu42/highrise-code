package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ptitlu42/highrise-code/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	code := cli.Run(ctx, os.Args[1:], cli.Default_env())
	stop()
	os.Exit(code)
}
