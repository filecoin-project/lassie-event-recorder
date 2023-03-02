package eventrecorder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/ipfs/go-log/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

var logger = log.Logger("lassie/event_recorder")

type EventRecorder struct {
	cfg    *config
	server *http.Server
	db     *pgxpool.Pool
}

func NewEventRecorder(ctx context.Context, opts ...option) (*EventRecorder, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, errors.New(fmt.Sprint("Failed to apply option:", err))
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

	poolConfig, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, errors.New(fmt.Sprint("Unable to parse DATABASE_URL:", err))
	}

	db, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.New(fmt.Sprint("Failed to create connection pool:", err))
	}
	recorder.db = db

	return &recorder, nil
}

func (r *EventRecorder) Start(_ context.Context) error {
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
	if req.Method != http.MethodPost {
		res.Header().Add("Allow", http.MethodPost)
		http.Error(res, "", http.StatusMethodNotAllowed)
		logger.Infof("%s %s %d", req.Method, req.URL.Path, http.StatusMethodNotAllowed)
		return
	}

	// Check if we're getting JSON content
	contentType := req.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		http.Error(res, "Not an acceptable content type. Content type must be application/json.", http.StatusBadRequest)
		logger.Infof("%s %s %d", req.Method, req.URL.Path, http.StatusBadRequest)
		return
	}

	// Decode JSON body
	var batch eventBatch
	if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		logger.Infof("%s %s %d", req.Method, req.URL.Path, http.StatusBadRequest)
		return
	}

	// Validate JSON
	if err := batch.validate(); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		logger.Infof("%s %s %d", req.Method, req.URL.Path, http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	var errs []error
	for _, event := range batch.Events {
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
		values := []interface{}{
			event.RetrievalId.String(),
			event.InstanceId,
			event.Cid,
			event.StorageProviderId,
			event.Phase,
			event.PhaseStartTime,
			event.EventName,
			event.EventTime,
			event.EventDetails,
		}
		_, err := r.db.Exec(ctx, query, values...)
		if err != nil {
			logger.Errorw("Could not execute insert query for retrieval event", "values", values, "err", err.Error())
			errs = append(errs, err)
			continue
		}

		logger.Debug("Saved retrieval event")
	}

	if len(errs) != 0 {
		http.Error(res, "", http.StatusInternalServerError)
		logger.Infof("%s %s %d", req.Method, req.URL.Path, http.StatusInternalServerError)
		return
	} else {
		logger.Infof("%s %s %d", req.Method, req.URL.Path, http.StatusOK)
	}
}

func (r *EventRecorder) Shutdown(ctx context.Context) error {
	return r.server.Shutdown(ctx)
}
