package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/filecoin-project/lassie-event-recorder/eventrecorder"
	"github.com/filecoin-project/lassie-event-recorder/httpserver"
	"github.com/filecoin-project/lassie-event-recorder/metrics"
	"github.com/ipfs/go-log/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var logger = log.Logger("lassie/event_recorder/cmd")

func main() {
	// TODO: add flags for all options eventually.
	httpListenAddr := flag.String("httpListenAddr", "0.0.0.0:8080", "The HTTP server listen address in address:port format.")
	dbDSN := flag.String("dbDSN", "", "The database Data Source Name. Alternatively, it may be specified via LASSIE_EVENT_RECORDER_DB_DSN environment variable. If both are present, the environment variable takes precedence.")
	logLevel := flag.String("logLevel", "info", "The logging level. Only applied if GOLOG_LOG_LEVEL environment variable is unset.")
	metricsListenAddr := flag.String("metricsListenAddr", "0.0.0.0:7777", "The metrics server listen address in address:port format.")
	mongoAddr := flag.String("mongo", "", "A Mongo endpoint to write to.")
	mongoDB := flag.String("mongoDB", "", "The Mongo DB to write to.")
	mongoCollection := flag.String("mongoCollection", "", "The Mongo Collection to write to.")
	mongoPercent := flag.Float64("mongoPercent", 0.0, "Percentage chance that a write will push to mongo [0,1]")

	flag.Parse()

	if _, set := os.LookupEnv("GOLOG_LOG_LEVEL"); !set {
		_ = log.SetLogLevel("*", *logLevel)
	}

	if v, set := os.LookupEnv("LASSIE_EVENT_RECORDER_DB_DSN"); set {
		dbDSN = &v
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:      *metricsListenAddr,
		Handler:   metricsMux,
		TLSConfig: nil,
	}

	metrics := metrics.New()

	opts := []eventrecorder.Option{
		eventrecorder.WithDatabaseDSN(*dbDSN),
		eventrecorder.WithMetrics(metrics),
	}
	if *mongoAddr != "" {
		mOpt := eventrecorder.WithMongoSubmissions(*mongoAddr, *mongoDB, *mongoCollection, float32(*mongoPercent))
		opts = append(opts, mOpt)
	}
	recorder, err := eventrecorder.New(opts...)
	if err != nil {
		logger.Fatalw("Failed to instantiate recorder", "err", err)
	}

	addr := httpserver.WithHttpServerListenAddr(*httpListenAddr)
	server, err := httpserver.NewHttpServer(recorder, addr)
	if err != nil {
		logger.Fatalw("Failed to instantiate server", "err", err)
	}

	ctx := context.Background()

	if err = metrics.Start(); err != nil {
		logger.Fatalw("Failed to start metrics", "err", err)
	}
	ln, err := net.Listen("tcp", metricsServer.Addr)
	if err != nil {
		logger.Fatalw("Failed to start listening on metrics addr", "err", err)
	}
	go func() { _ = metricsServer.Serve(ln) }()

	if err = server.Start(ctx); err != nil {
		logger.Fatalw("Failed to start server", "err", err)
	}

	sch := make(chan os.Signal, 1)
	signal.Notify(sch, os.Interrupt)
	<-sch
	logger.Info("Terminating...")
	if err := server.Shutdown(ctx); err != nil {
		logger.Warnw("Failed to shut down server.", "err", err)
	} else {
		logger.Info("Shut down server successfully")
	}
}
