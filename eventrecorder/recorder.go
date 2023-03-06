package eventrecorder

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/ipfs/go-log/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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
	mux.HandleFunc("/v1/summarize-and-clear", r.handleStats)
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
	logger.Infow("Successfully submitted batch event insertion")
}

type EventSummary struct {
	TotalAttempts              uint64    `json:"totalAttempts"`
	AttemptedBitswap           uint64    `json:"attemptedBitswap"`
	AttemptedGraphSync         uint64    `json:"attemptedGraphSync"`
	AttemptedBoth              uint64    `json:"attemptedBoth"`
	AttemptedEither            uint64    `json:"attemptedEither"`
	BitswapSuccesses           uint64    `json:"bitswapSuccesses"`
	GraphSyncSuccesses         uint64    `json:"graphSyncSuccesses"`
	AvgBandwidth               *float64  `json:"avgBandwidth"`
	FirstByte                  []float64 `json:"firstByte"`
	DownloadSize               []float64 `json:"downloadSize"`
	GraphsyncAttemptsPastQuery uint64    `json:"graphsyncAttemptsPastQuery"`
}

func (r *EventRecorder) handleStats(res http.ResponseWriter, req *http.Request) {
	logger := logger.With("method", req.Method, "path", req.URL.Path)
	if req.Method != http.MethodGet {
		res.Header().Add("Allow", http.MethodGet)
		http.Error(res, "", http.StatusMethodNotAllowed)
		logger.Warn("Rejected disallowed method")
		return
	}

	ctx := req.Context()
	runQuery := `
	select count(all_attempts.retrieval_id) as total_attempts, 
  count(bitswap_retrievals.retrieval_id) as attempted_bitswap, 
  count(graphsync_retrievals.retrieval_id) as attempted_graphsync, 
  sum(case when bitswap_retrievals.retrieval_id IS NOT NULL and graphsync_retrievals.retrieval_id IS NOT NULL then 1 else 0 end) as attempted_both,
  sum(case when bitswap_retrievals.retrieval_id IS NOT NULL or graphsync_retrievals.retrieval_id IS NOT NULL then 1 else 0 end) as attempted_either,
  sum(case when successful_retrievals.storage_provider_id = 'Bitswap' then 1 else 0 end) as bitswap_successes,
  sum(case when successful_retrievals.storage_provider_id <> 'Bitswap' and successful_retrievals.retrieval_id IS NOT NULL then 1 else 0 end) as graphsync_successes,
  case when extract('epoch' from sum(successful_retrievals.event_time - first_byte_retrievals.event_time)) = 0 then 0 else sum(successful_retrievals.received_size)::float / extract('epoch' from sum(successful_retrievals.event_time - first_byte_retrievals.event_time))::float end as avg_bandwidth,
  percentile_cont('{0.5, 0.9, 0.95}'::double precision[]) WITHIN GROUP (ORDER BY (extract ('epoch' from first_byte_retrievals.event_time - all_attempts.event_time))) as p50_p90_p95_first_byte,
  percentile_cont('{0.5, 0.9, 0.95}'::double precision[]) WITHIN GROUP (ORDER BY (successful_retrievals.received_size)) as p50_p90_p95_download_size,
	count(graphsync_retrieval_attempts.retrieval_id) as graphsync_retrieval_attempts_past_query
  from (
    select distinct on (retrieval_id) retrieval_id, event_time from retrieval_events order by retrieval_id, event_time
    ) as all_attempts left join (
      select distinct retrieval_id from retrieval_events where storage_provider_id = 'Bitswap'
      ) as bitswap_retrievals on all_attempts.retrieval_id = bitswap_retrievals.retrieval_id left join (
        select distinct retrieval_id from retrieval_events where storage_provider_id <> 'Bitswap' and phase <> 'indexer'
        ) as graphsync_retrievals on graphsync_retrievals.retrieval_id = all_attempts.retrieval_id left join (
          select distinct on (retrieval_id) retrieval_id, event_time, storage_provider_id, (event_details ->	'receivedSize')::int8 as received_size from retrieval_events where event_name = 'success' order by retrieval_id, event_time
        ) as successful_retrievals on  successful_retrievals.retrieval_id = all_attempts.retrieval_id left join (
          select retrieval_id, event_time, storage_provider_id from retrieval_events where event_name = 'first-byte-received'
        ) as first_byte_retrievals on successful_retrievals.retrieval_id = first_byte_retrievals.retrieval_id and successful_retrievals.storage_provider_id = first_byte_retrievals.storage_provider_id left join (
					select distinct retrieval_id from retrieval_events where storage_provider_id <> 'Bitswap' and phase = 'retrieval'
        ) as graphsync_retrieval_attempts on graphsync_retrievals.retrieval_id = graphsync_retrieval_attempts.retrieval_id
	`

	row := r.db.QueryRow(ctx, runQuery)
	var summary EventSummary
	err := row.Scan(&summary.TotalAttempts,
		&summary.AttemptedBitswap,
		&summary.AttemptedGraphSync,
		&summary.AttemptedBoth,
		&summary.AttemptedEither,
		&summary.BitswapSuccesses,
		&summary.GraphSyncSuccesses,
		&summary.AvgBandwidth,
		&summary.FirstByte,
		&summary.DownloadSize,
		&summary.GraphsyncAttemptsPastQuery)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		logger.Errorw("Failure to execute query", "err", err)
		return
	}
	err = json.NewEncoder(res).Encode(summary)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		logger.Errorw("failed encoding result", "err", err)
		return
	}

	r.db.Exec(ctx, "TRUNCATE TABLE retrieval_events")
	logger.Infow("Successfully ran summary and cleared DB")
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
