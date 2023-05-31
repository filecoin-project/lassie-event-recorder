package eventrecorder

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/filecoin-project/lassie-event-recorder/metrics"
	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/ipfs/go-log/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var logger = log.Logger("lassie/eventrecorder")

type EventRecorder struct {
	cfg *config
	db  *pgxpool.Pool

	mongo *mongo.Client
	mc    *mongo.Collection
}

func New(opts ...Option) (*EventRecorder, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to apply option: %w", err)
	}

	var recorder EventRecorder
	recorder.cfg = cfg
	return &recorder, nil
}

func (r *EventRecorder) RecordEvents(ctx context.Context, events []Event) error {
	totalLogger := logger.With("total", len(events))

	var batchQuery pgx.Batch
	for _, event := range events {
		// Create the insert query
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
				totalLogger.Warnw("Retrieval event insertion did not affect any rows", "event", event, "rowsAffected", rowsAffected)
			default:
				totalLogger.Debugw("Inserted event successfully", "event", event, "rowsAffected", rowsAffected)
			}
			return nil
		})

		// Emit a metric
		if r.cfg.metrics != nil {
			switch event.EventName {
			case types.StartedCode:
				r.cfg.metrics.HandleStartedEvent(ctx, event.RetrievalId, event.Phase, event.EventTime, event.StorageProviderId)
			case types.CandidatesFoundCode:
				r.cfg.metrics.HandleCandidatesFoundEvent(ctx, event.RetrievalId, event.EventTime, event.EventDetails)
			case types.CandidatesFilteredCode:
				r.cfg.metrics.HandleCandidatesFilteredEvent(ctx, event.RetrievalId, event.EventDetails)
			case types.FailedCode:
				r.cfg.metrics.HandleFailureEvent(ctx, event.RetrievalId, event.Phase, event.StorageProviderId, event.EventDetails)
			case types.FirstByteCode:
				r.cfg.metrics.HandleTimeToFirstByteEvent(ctx, event.RetrievalId, event.StorageProviderId, event.EventTime)
			case types.SuccessCode:
				r.cfg.metrics.HandleSuccessEvent(ctx, event.RetrievalId, event.EventTime, event.StorageProviderId, event.EventDetails)
			}
		}
	}

	// Execute the batch
	batchResult := r.db.SendBatch(ctx, &batchQuery)
	err := batchResult.Close()
	if err != nil {
		totalLogger.Errorw("At least one retrieval event insertion failed", "err", err)
		return err
	}
	totalLogger.Info("Successfully submitted batch event insertion")

	return nil
}

func (r *EventRecorder) RecordAggregateEvents(ctx context.Context, events []AggregateEvent) error {
	totalLogger := logger.With("total", len(events))

	var batchQuery pgx.Batch
	var batchRetrievalAttempts pgx.Batch
	for _, event := range events {
		var timeToFirstByte time.Duration
		if event.TimeToFirstByte != "" {
			timeToFirstByte, _ = time.ParseDuration(event.TimeToFirstByte)
		}
		var timeToFirstIndexerResult time.Duration
		if event.TimeToFirstIndexerResult != "" {
			timeToFirstIndexerResult, _ = time.ParseDuration(event.TimeToFirstIndexerResult)
		}
		query := `
		INSERT INTO aggregate_retrieval_events(
			instance_id,
			retrieval_id,
			storage_provider_id,
			time_to_first_byte,
			bandwidth_bytes_sec,
			bytes_transferred,
			success,
			start_time,
			end_time,
			time_to_first_indexer_result,
			indexer_candidates_received,
			indexer_candidates_filtered,
			protocols_allowed,
			protocols_attempted,
			protocol_succeeded
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		`
		batchQuery.Queue(query,
			event.InstanceID,
			event.RetrievalID,
			event.StorageProviderID,
			timeToFirstByte,
			event.Bandwidth,
			event.BytesTransferred,
			event.Success,
			event.StartTime,
			event.EndTime,
			timeToFirstIndexerResult,
			event.IndexerCandidatesReceived,
			event.IndexerCandidatesFiltered,
			event.ProtocolsAllowed,
			event.ProtocolsAttempted,
			event.ProtocolSucceeded,
		).Exec(func(ct pgconn.CommandTag) error {
			rowsAffected := ct.RowsAffected()
			switch rowsAffected {
			case 0:
				totalLogger.Warnw("Aggregated event insertion did not affect any rows", "event", event, "rowsAffected", rowsAffected)
			default:
				totalLogger.Debugw("Inserted aggregated event successfully", "event", event, "rowsAffected", rowsAffected)
			}
			return nil
		})
		attempts := make(map[string]metrics.Attempt, len(event.RetrievalAttempts))
		for storageProviderID, retrievalAttempt := range event.RetrievalAttempts {

			var timeToFirstByte time.Duration
			if retrievalAttempt.TimeToFirstByte != "" {
				timeToFirstByte, _ = time.ParseDuration(retrievalAttempt.TimeToFirstByte)
			}
			attempts[storageProviderID] = metrics.Attempt{
				Error:           retrievalAttempt.Error,
				Protocol:        retrievalAttempt.Protocol,
				TimeToFirstByte: timeToFirstByte,
			}
			query := `
		  INSERT INTO retrieval_attempts(
			  retrieval_id,
			  storage_provider_id,
			  time_to_first_byte,
			  error,
			  protocol
		  )
		  VALUES ($1, $2, $3, $4, $5)
		  `
			batchRetrievalAttempts.Queue(query,
				event.RetrievalID,
				storageProviderID,
				timeToFirstByte,
				retrievalAttempt.Error,
				retrievalAttempt.Protocol,
			).Exec(func(ct pgconn.CommandTag) error {
				rowsAffected := ct.RowsAffected()
				switch rowsAffected {
				case 0:
					totalLogger.Warnw("Retrieval attempt insertion did not affect any rows", "retrievalID", event.RetrievalID, "retrievalAttempt", retrievalAttempt, "storageProviderID", storageProviderID, "rowsAffected", rowsAffected)
				default:
					totalLogger.Debugw("Inserted retrieval attempt successfully", "retrievalID", event.RetrievalID, "retrievalAttempt", retrievalAttempt, "storageProviderID", storageProviderID, "rowsAffected", rowsAffected)
				}
				return nil
			})
		}
		if r.cfg.metrics != nil {
			r.cfg.metrics.HandleAggregatedEvent(ctx,
				timeToFirstIndexerResult,
				timeToFirstByte,
				event.Success,
				event.StorageProviderID,
				event.StartTime,
				event.EndTime,
				int64(event.Bandwidth),
				int64(event.BytesTransferred),
				int64(event.IndexerCandidatesReceived),
				int64(event.IndexerCandidatesFiltered),
				attempts,
				event.ProtocolSucceeded,
			)
		}

		if r.mc != nil &&
			rand.Float32() < r.cfg.mongoPercentile &&
			event.StorageProviderID != "Bitswap" &&
			event.StorageProviderID != "" {
			report := RetrievalReport{
				RetrievalID:       event.RetrievalID,
				InstanceID:        event.InstanceID,
				StorageProviderID: event.StorageProviderID,
				TTFB:              timeToFirstByte.Milliseconds(),
				Bandwidth:         int64(event.Bandwidth),
				Success:           event.Success,
				StartTime:         event.StartTime,
				EndTime:           event.EndTime,
			}
			_, err := r.mc.InsertOne(ctx, report)
			if err != nil {
				logger.Infof("failed to report to mongo: %w", err)
			}
		}
	}
	batchResult := r.db.SendBatch(ctx, &batchQuery)
	err := batchResult.Close()
	if err != nil {
		totalLogger.Errorw("At least one aggregated event insertion failed", "err", err)
		return err
	}
	batchResult = r.db.SendBatch(ctx, &batchRetrievalAttempts)
	err = batchResult.Close()
	if err != nil {
		totalLogger.Errorw("At least one retrieval attempt insertion failed", "err", err)
		return err
	}
	totalLogger.Info("Successfully submitted batch event insertion")
	return nil
}

type RetrievalReport struct {
	RetrievalID       string    `bson:"retrieval_id"`
	InstanceID        string    `bson:"instance_id"`
	StorageProviderID string    `bson:"storage_provider_id"`
	TTFB              int64     `bson:"time_to_first_byte_ms"`
	Bandwidth         int64     `bson:"bandwidth_bytes_sec"`
	Success           bool      `bson:"success"`
	StartTime         time.Time `bson:"start_time"`
	EndTime           time.Time `bson:"end_time"`
}

func (r *EventRecorder) Start(ctx context.Context) error {
	var err error
	r.db, err = pgxpool.NewWithConfig(ctx, r.cfg.pgxPoolConfig)
	if err != nil {
		return fmt.Errorf("failed to instantiate database connection: %w", err)
	}

	if r.cfg.mongoEndpoint != "" {
		r.mongo, err = mongo.NewClient(options.Client().ApplyURI(r.cfg.mongoEndpoint))
		if err != nil {
			return fmt.Errorf("failed to instantiate mongo database connection: %w", err)
		}
		timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = r.mongo.Connect(timeout)
		if err != nil {
			return fmt.Errorf("failed to connect to mongo: %w", err)
		}
		r.mc = r.mongo.Database(r.cfg.mongoDB).Collection(r.cfg.mongoCollection)
	}
	return nil
}

func (r *EventRecorder) Shutdown() {
	logger.Info("Closing database connection...")
	r.db.Close()
	logger.Info("Database connection closed successfully.")
	if r.mongo != nil {
		timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := r.mongo.Disconnect(timeout)
		if err != nil {
			logger.Warn("failed to close mongo connection: %v", err)
		}
	}
}
