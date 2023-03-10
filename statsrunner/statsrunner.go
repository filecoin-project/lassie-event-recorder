package statsrunner

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StatsRunner struct {
	pgxPoolConfig *pgxpool.Config
	db            *pgxpool.Pool
}

func New(dbDSN string) (*StatsRunner, error) {
	pgxPoolConfig, err := pgxpool.ParseConfig(dbDSN)
	if err != nil {
		return nil, err
	}
	return &StatsRunner{pgxPoolConfig: pgxPoolConfig}, nil
}

func (sr *StatsRunner) Start(ctx context.Context) error {
	var err error
	sr.db, err = pgxpool.NewWithConfig(ctx, sr.pgxPoolConfig)
	return err
}

func (sr *StatsRunner) Close() {
	sr.db.Close()
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

func (sr *StatsRunner) GetEventSummary(ctx context.Context) (*EventSummary, error) {

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

	row := sr.db.QueryRow(ctx, runQuery)
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
		return nil, err
	}
	return &summary, nil
}

func (sr *StatsRunner) WipeTable(ctx context.Context) {
	sr.db.Exec(ctx, "TRUNCATE TABLE retrieval_events")
}
