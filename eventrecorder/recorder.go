package eventrecorder

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/ipfs/go-log/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

var logger = log.Logger("lassie/eventrecorder")

type EventRecorder struct {
	cfg    *config
	server *http.Server
	db     *pgxpool.Pool
}

func NewEventRecorder(opts ...Option) (*EventRecorder, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to apply option: %w", err)
	}

	var recorder EventRecorder
	recorder.cfg = cfg
	recorder.server = &http.Server{
		Addr:              recorder.cfg.httpServerListenAddr,
		Handler:           recorder.httpServerMux(),
		ReadTimeout:       recorder.cfg.httpServerReadTimeout,
		ReadHeaderTimeout: recorder.cfg.httpServerReadHeaderTimeout,
		WriteTimeout:      recorder.cfg.httpServerWriteTimeout,
		IdleTimeout:       recorder.cfg.httpServerWriteTimeout,
		MaxHeaderBytes:    recorder.cfg.httpServerMaxHeaderBytes,
	}
	return &recorder, nil
}

func (r *EventRecorder) Start(ctx context.Context) error {
	var err error
	r.db, err = pgxpool.NewWithConfig(ctx, r.cfg.pgxPoolConfig)
	if err != nil {
		return fmt.Errorf("failed to instantiate dabase connection: %w", err)
	}
	r.server.RegisterOnShutdown(func() {
		logger.Info("Closing database connection...")
		r.db.Close()
		logger.Info("Database connection closed successfully.")
	})
	ln, err := net.Listen("tcp", r.server.Addr)
	if err != nil {
		return err
	}
	go func() { _ = r.server.Serve(ln) }()
	logger.Infow("Server started", "addr", ln.Addr())
	return nil
}

func (r *EventRecorder) httpServerMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/retrieval-events", r.handleRetrievalEvents)
	return mux
}

func (r *EventRecorder) handleRetrievalEvents(res http.ResponseWriter, req *http.Request) {
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
	var batch eventBatch
	if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		logger.Warn("Rejected bad request with undecodable json body")
		return
	}

	// Validate JSON
	if err := batch.validate(); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		logger.Warn("Rejected bad request with invalid event")
		return
	}

	logger = logger.With("total", len(batch.Events))
	ctx := req.Context()
	errs := make([]error, 0, len(batch.Events))
	for _, event := range batch.Events {
		// TODO: use db batching; this will not be performant at scale.
		query := `
		INSERT INTO retrieval_events(
			retrieval_id,
			instance_id,
			cid,
			storage_provider_id,
			phase,
			phase_start_time,
			event_name,
			event_time,
			event_details
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`
		_, err := r.db.Exec(ctx, query,
			event.RetrievalId.String(),
			event.InstanceId,
			event.Cid,
			event.StorageProviderId,
			event.Phase,
			event.PhaseStartTime,
			event.EventName,
			event.EventTime,
			event.EventDetails,
		)
		if err != nil {
			logger.Errorw("Failed to insert retrieval event", "event", event, "err", err)
			errs = append(errs, err)
			continue
		}
		logger.Debug("Saved retrieval event")
	}

	if len(errs) != 0 {
		http.Error(res, "", http.StatusInternalServerError)
		logger.Infow("At least one retrieval event insertion failed", "failed", len(errs))
		return
	}
	logger.Infow("Successfully inserted events")
}

func (r *EventRecorder) Shutdown(ctx context.Context) error {
	return r.server.Shutdown(ctx)
}
