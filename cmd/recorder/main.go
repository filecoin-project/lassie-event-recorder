package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/filecoin-project/lassie-event-recorder/eventrecorder"
	"github.com/ipfs/go-log/v2"
)

var logger = log.Logger("lassie/event_recorder/cmd")

func main() {
	log.SetAllLoggers(log.LevelInfo)
	ctx := context.Background()

	// TODO: add flags for all options eventually.
	httpListenAddr := flag.String("httpListenAddr", "0.0.0.0:40080", "The HTTP server listen address in address:port format.")
	flag.Parse()

	addrOpt := eventrecorder.WithHttpServerListenAddr(*httpListenAddr)
	r, err := eventrecorder.NewEventRecorder(ctx, addrOpt)
	if err != nil {
		logger.Fatalw("Failed to instantiate recorder", "err", err)
	}

	if err = r.Start(ctx); err != nil {
		logger.Fatalw("Failed to start recorder", "err", err)
	}

	sch := make(chan os.Signal, 1)
	signal.Notify(sch, os.Interrupt)
	<-sch
	fmt.Println()
	logger.Info("Terminating...")
	if err := r.Shutdown(ctx); err != nil {
		logger.Warnw("Failed to shut down server.", "err", err)
	} else {
		logger.Info("Shut down server successfully")
	}
}
