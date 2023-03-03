package main

import (
	"context"
	"flag"
	"os"
	"os/signal"

	"github.com/filecoin-project/lassie-event-recorder/eventrecorder"
	"github.com/ipfs/go-log/v2"
)

var logger = log.Logger("lassie/event_recorder/cmd")

func main() {

	// TODO: add flags for all options eventually.
	httpListenAddr := flag.String("httpListenAddr", "0.0.0.0:8080", "The HTTP server listen address in address:port format.")
	dbDSN := flag.String("dbDSN", "", "The database Data Source Name. Alternatively, it may be specified via LASSIE_EVENT_RECORDER_DB_DSN environment variable. If both are present, the environment variable takes precedence.")
	logLevel := flag.String("logLevel", "info", "The logging level. Only applied if GOLOG_LOG_LEVEL environment variable is unset.")
	flag.Parse()

	if _, set := os.LookupEnv("GOLOG_LOG_LEVEL"); !set {
		_ = log.SetLogLevel("*", *logLevel)
	}

	if v, set := os.LookupEnv("LASSIE_EVENT_RECORDER_DB_DSN"); set {
		dbDSN = &v
	}

	r, err := eventrecorder.NewEventRecorder(
		eventrecorder.WithHttpServerListenAddr(*httpListenAddr),
		eventrecorder.WithDatabaseDSN(*dbDSN),
	)
	if err != nil {
		logger.Fatalw("Failed to instantiate recorder", "err", err)
	}

	ctx := context.Background()
	if err = r.Start(ctx); err != nil {
		logger.Fatalw("Failed to start recorder", "err", err)
	}

	sch := make(chan os.Signal, 1)
	signal.Notify(sch, os.Interrupt)
	<-sch
	logger.Info("Terminating...")
	if err := r.Shutdown(ctx); err != nil {
		logger.Warnw("Failed to shut down server.", "err", err)
	} else {
		logger.Info("Shut down server successfully")
	}
}
