package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/filecoin-project/lassie-event-recorder/eventrecorder"
	"github.com/ipfs/go-log/v2"
)

var logger = log.Logger("lassie/httpserver")

type HttpServer struct {
	cfg     *config
	server  *http.Server
	handler *HttpHandler
}

func NewHttpServer(recorder *eventrecorder.EventRecorder, opts ...option) (*HttpServer, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to apply option: %w", err)
	}

	var httpServer HttpServer
	httpServer.cfg = cfg
	httpServer.handler = NewHttpHandler(recorder)
	httpServer.server = &http.Server{
		Addr:              httpServer.cfg.httpServerListenAddr,
		Handler:           httpServer.handler.Handler(),
		ReadTimeout:       httpServer.cfg.httpServerReadTimeout,
		ReadHeaderTimeout: httpServer.cfg.httpServerReadHeaderTimeout,
		WriteTimeout:      httpServer.cfg.httpServerWriteTimeout,
		IdleTimeout:       httpServer.cfg.httpServerWriteTimeout,
		MaxHeaderBytes:    httpServer.cfg.httpServerMaxHeaderBytes,
	}

	return &httpServer, nil
}

func (hs HttpServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", hs.server.Addr)
	if err != nil {
		return err
	}

	// Start the event recorder
	if err := hs.handler.Start(ctx); err != nil {
		return err
	}

	// Shutdown the event recorder when the server shutdowns
	hs.server.RegisterOnShutdown(func() {
		hs.handler.Shutdown()
	})

	go func() { _ = hs.server.Serve(ln) }()
	logger.Infow("Server started", "addr", ln.Addr())
	return nil
}

func (hs HttpServer) Shutdown(ctx context.Context) error {
	return hs.server.Shutdown(ctx)
}

type HttpHandler struct {
	recorder *eventrecorder.EventRecorder
}

func NewHttpHandler(recorder *eventrecorder.EventRecorder) *HttpHandler {
	return &HttpHandler{recorder}
}

func (hh HttpHandler) Start(ctx context.Context) error {
	return hh.recorder.Start(ctx)
}

func (hh HttpHandler) Shutdown() {
	hh.recorder.Shutdown()
}

func (hh *HttpHandler) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/retrieval-events", hh.handleRetrievalEvents)
	mux.HandleFunc("/v2/retrieval-events", hh.handleRetrievalEventsV2)
	mux.HandleFunc("/ready", hh.handleReady)
	return mux
}

func (hh *HttpHandler) handleRetrievalEvents(res http.ResponseWriter, req *http.Request) {
	logger := logger.With("method", req.Method, "path", req.URL.Path)
	if req.Method != http.MethodPost {
		res.Header().Add("Allow", http.MethodPost)
		http.Error(res, "", http.StatusMethodNotAllowed)
		logger.Warn("Rejected disallowed method")
		return
	}

	// Check if we're getting JSON content
	contentType := req.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		http.Error(res, "Not an acceptable content type. Content type must be application/json.", http.StatusBadRequest)
		logger.Warn("Rejected bad request with non-json content type")
		return
	}

	// Decode JSON body
	var batch eventrecorder.EventBatch
	if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		logger.Warn("Rejected bad request with undecodable json body")
		return
	}

	// Validate JSON
	if err := batch.Validate(); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		logger.Warnf("Rejected bad request with invalid event: %s", err.Error())
		return
	}

	err := hh.recorder.RecordEvents(req.Context(), batch.Events)
	if err != nil {
		http.Error(res, "", http.StatusInternalServerError)
		return
	}
}

func (hh *HttpHandler) handleRetrievalEventsV2(res http.ResponseWriter, req *http.Request) {
	logger := logger.With("method", req.Method, "path", req.URL.Path)
	if req.Method != http.MethodPost {
		res.Header().Add("Allow", http.MethodPost)
		http.Error(res, "", http.StatusMethodNotAllowed)
		logger.Warn("Rejected disallowed method")
		return
	}

	// Check if we're getting JSON content
	contentType := req.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		http.Error(res, "Not an acceptable content type. Content type must be application/json.", http.StatusBadRequest)
		logger.Warn("Rejected bad request with non-json content type")
		return
	}

	// Decode JSON body
	var batch eventrecorder.AggregateEventBatch
	if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		logger.Warn("Rejected bad request with undecodable json body")
		return
	}

	// Validate JSON
	if err := batch.Validate(); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		logger.Warnf("Rejected bad request with invalid event: %s", err.Error())
		return
	}

	err := hh.recorder.RecordAggregateEvents(req.Context(), batch.Events)
	if err != nil {
		http.Error(res, "", http.StatusInternalServerError)
		return
	}
}

func (hh *HttpHandler) handleReady(res http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		// TODO: ping DB as part of readiness check?
		res.Header().Add("Allow", http.MethodGet)
	default:
		http.Error(res, "", http.StatusMethodNotAllowed)
	}
}
