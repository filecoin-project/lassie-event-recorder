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
	cfg      *config
	recorder *eventrecorder.EventRecorder
	server   *http.Server
}

func NewHttpServer(recorder *eventrecorder.EventRecorder, opts ...option) (*HttpServer, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to apply option: %w", err)
	}

	var httpServer HttpServer
	httpServer.cfg = cfg
	httpServer.server = &http.Server{
		Addr:              httpServer.cfg.httpServerListenAddr,
		Handler:           httpServer.httpServerMux(),
		ReadTimeout:       httpServer.cfg.httpServerReadTimeout,
		ReadHeaderTimeout: httpServer.cfg.httpServerReadHeaderTimeout,
		WriteTimeout:      httpServer.cfg.httpServerWriteTimeout,
		IdleTimeout:       httpServer.cfg.httpServerWriteTimeout,
		MaxHeaderBytes:    httpServer.cfg.httpServerMaxHeaderBytes,
	}

	httpServer.recorder = recorder

	return &httpServer, nil
}

func (h HttpServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", h.server.Addr)
	if err != nil {
		return err
	}

	// Start the event recorder
	h.recorder.Start(ctx)

	// Shutdown the event recorder when the server shutdowns
	h.server.RegisterOnShutdown(func() {
		h.recorder.Shutdown()
	})

	go func() { _ = h.server.Serve(ln) }()
	logger.Infow("Server started", "addr", ln.Addr())
	return nil
}

func (h HttpServer) Shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

func (r *HttpServer) httpServerMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/retrieval-events", r.handleRetrievalEvents)
	mux.HandleFunc("/v2/retrieval-events", r.handleRetrievalEventsV2)
	mux.HandleFunc("/ready", r.handleReady)
	return mux
}

func (h *HttpServer) handleRetrievalEvents(res http.ResponseWriter, req *http.Request) {
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

	err := h.recorder.RecordEvents(req.Context(), batch.Events)
	if err != nil {
		http.Error(res, "", http.StatusInternalServerError)
		return
	}
}

func (h *HttpServer) handleRetrievalEventsV2(res http.ResponseWriter, req *http.Request) {
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

	err := h.recorder.RecordAggregateEvents(req.Context(), batch.Events)
	if err != nil {
		http.Error(res, "", http.StatusInternalServerError)
		return
	}
}

func (r *HttpServer) handleReady(res http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		// TODO: ping DB as part of readiness check?
		res.Header().Add("Allow", http.MethodGet)
	default:
		http.Error(res, "", http.StatusMethodNotAllowed)
	}
}
