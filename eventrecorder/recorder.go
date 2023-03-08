package eventrecorder

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/filecoin-project/lassie/pkg/metrics"
	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/ipfs/go-log/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opencensus.io/stats"
)

var logger = log.Logger("lassie/eventrecorder")

type EventRecorder struct {
	cfg    *config
	server *http.Server
	db     *pgxpool.Pool
}

func New(opts ...option) (*EventRecorder, error) {
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
	mux.HandleFunc("/ready", r.handleReady)
	// register prometheus metrics
	mux.Handle("/metrics", metrics.NewExporter())
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
	var batch EventBatch
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

	logger = logger.With("total", len(batch.Events))
	ctx := req.Context()
	var batchQuery pgx.Batch
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
		batchQuery.Queue(query,
			event.RetrievalId.String(),
			event.InstanceId,
			event.Cid,
			event.StorageProviderId,
			event.Phase,
			event.PhaseStartTime,
			event.EventName,
			event.EventTime,
			event.EventDetails,
		).Exec(func(ct pgconn.CommandTag) error {
			rowsAffected := ct.RowsAffected()
			switch rowsAffected {
			case 0:
				logger.Warnw("Retrieval event insertion did not affect any rows", "event", event, "rowsAffected", rowsAffected)
			default:
				logger.Debugw("Inserted event successfully", "event", event, "rowsAffected", rowsAffected)
			}
			return nil
		})
	}
	batchResult := r.db.SendBatch(ctx, &batchQuery)
	if err := batchResult.Close(); err != nil {
		http.Error(res, "", http.StatusInternalServerError)
		logger.Errorw("At least one retrieval event insertion failed", "err", err)
		return
	}
	for _, event := range batch.Events {
		recordMetrics(event)
	}
	logger.Infow("Successfully submitted batch event insertion")
}

func (r *EventRecorder) handleReady(res http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		// TODO: ping DB as part of readiness check?
		res.Header().Add("Allow", http.MethodGet)
	default:
		http.Error(res, "", http.StatusMethodNotAllowed)
	}
}

func (r *EventRecorder) Shutdown(ctx context.Context) error {
	return r.server.Shutdown(ctx)
}

// Implement RetrievalSubscriber
func recordMetrics(event Event) {
	switch event.EventName {
	case types.CandidatesFoundCode:
		handleCandidatesFoundEvent(event)
	case types.CandidatesFilteredCode:
		handleCandidatesFilteredEvent(event)
	case types.StartedCode:
		handleStartedEvent(event)
	case types.FailedCode:
		handleFailureEvent(event)
	case types.QueryAskedCode: // query-ask success
		handleQueryAskEvent()
	case types.QueryAskedFilteredCode:
		handleQueryAskFilteredEvent()
	}
}

func handleQueryAskFilteredEvent() {
	stats.Record(context.Background(), metrics.RequestWithSuccessfulQueriesFilteredCount.M(1))
}

func handleQueryAskEvent() {
	stats.Record(context.Background(), metrics.RequestWithSuccessfulQueriesCount.M(1))
}

// handleFailureEvent is called when a query _or_ retrieval fails
func handleFailureEvent(event Event) {

	detailsObj, ok := event.EventDetails.(map[string]interface{})
	if !ok {
		return
	}
	msg, ok := detailsObj["Error"].(string)
	if !ok {
		return
	}
	switch event.Phase {
	case types.QueryPhase:
		var matched bool
		for substr, metric := range metrics.QueryErrorMetricMatches {
			if strings.Contains(msg, substr) {
				stats.Record(context.Background(), metric.M(1))
				matched = true
				break
			}
		}
		if !matched {
			stats.Record(context.Background(), metrics.QueryErrorOtherCount.M(1))
		}
	case types.RetrievalPhase:
		stats.Record(context.Background(), metrics.RetrievalDealFailCount.M(1))
		stats.Record(context.Background(), metrics.RetrievalDealActiveCount.M(-1))

		var matched bool
		for substr, metric := range metrics.ErrorMetricMatches {
			if strings.Contains(msg, substr) {
				stats.Record(context.Background(), metric.M(1))
				matched = true
				break
			}
		}
		if !matched {
			stats.Record(context.Background(), metrics.RetrievalErrorOtherCount.M(1))
		}
	}
}

func handleStartedEvent(event Event) {
	if event.Phase == types.RetrievalPhase {
		stats.Record(context.Background(), metrics.RetrievalRequestCount.M(1))
		stats.Record(context.Background(), metrics.RetrievalDealActiveCount.M(1))
	}
}

func handleCandidatesFilteredEvent(event Event) {
	detailsObj, ok := event.EventDetails.(map[string]interface{})
	if !ok {
		return
	}

	candidateCount, ok := detailsObj["CandidateCount"].(float64)
	if !ok {
		return
	}
	if candidateCount > 0 {
		stats.Record(context.Background(), metrics.RequestWithIndexerCandidatesFilteredCount.M(1))
	}
}

func handleCandidatesFoundEvent(event Event) {
	detailsObj, ok := event.EventDetails.(map[string]interface{})
	if !ok {
		return
	}

	candidateCount, ok := detailsObj["CandidateCount"].(float64)
	if !ok {
		return
	}
	if candidateCount > 0 {
		stats.Record(context.Background(), metrics.RequestWithIndexerCandidatesCount.M(1))
	}
	stats.Record(context.Background(), metrics.IndexerCandidatesPerRequestCount.M(int64(candidateCount)))
}
