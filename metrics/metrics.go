package metrics

import (
	"github.com/filecoin-project/lassie-event-recorder/metrics/tempdata"
	logging "github.com/ipfs/go-log/v2"

	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
)

var log = logging.Logger("metrics")

type Metrics struct {
	stats
	tempDataMap *tempdata.TempDataMap
}

func New() *Metrics {
	return &Metrics{
		tempDataMap: tempdata.NewTempDataMap(),
	}
}

func (m *Metrics) Start() error {
	// The exporter embeds a default OpenTelemetry Reader and
	// implements prometheus.Collector, allowing it to be used as
	// both a Reader and Collector.
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal(err)
	}
	meterName := "lassie-event-recorder"
	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		// histogram buckets
		metric.WithView(metric.NewView(
			metric.Instrument{
				Name:  "failed_retrievals_per_request_total",
				Scope: instrumentation.Scope{Name: meterName},
			},
			metric.Stream{
				Aggregation: aggregation.ExplicitBucketHistogram{
					Boundaries: []float64{0, 1, 2, 3, 4, 5, 10, 20, 40},
				},
			},
		),
			metric.NewView(
				metric.Instrument{
					Name:  "indexer_candidates_per_request_total",
					Scope: instrumentation.Scope{Name: meterName},
				},
				metric.Stream{
					Aggregation: aggregation.ExplicitBucketHistogram{
						Boundaries: []float64{0, 1, 2, 3, 4, 5, 10, 20, 40},
					},
				},
			),
			metric.NewView(
				metric.Instrument{
					Name:  "indexer_candidates_filtered_per_request_total",
					Scope: instrumentation.Scope{Name: meterName},
				},
				metric.Stream{
					Aggregation: aggregation.ExplicitBucketHistogram{
						Boundaries: []float64{0, 1, 2, 3, 4, 5, 10, 20, 40},
					},
				},
			),
			metric.NewView(
				metric.Instrument{
					Name:  "retrieval_deal_duration_seconds",
					Scope: instrumentation.Scope{Name: meterName},
				},
				metric.Stream{
					Aggregation: aggregation.ExplicitBucketHistogram{
						Boundaries: []float64{0, 0.04, 0.2, 1, 5, 25, 125, 625},
					},
				},
			),
			metric.NewView(
				metric.Instrument{
					Name:  "retrieval_deal_duration_seconds",
					Scope: instrumentation.Scope{Name: meterName},
				},
				metric.Stream{
					Aggregation: aggregation.ExplicitBucketHistogram{
						Boundaries: []float64{0, 0.04, 0.2, 1, 5, 25, 125, 625},
					},
				},
			),
			metric.NewView(
				metric.Instrument{
					Name:  "time_to_first_indexer_result",
					Scope: instrumentation.Scope{Name: meterName},
				},
				metric.Stream{
					Aggregation: aggregation.ExplicitBucketHistogram{
						Boundaries: []float64{0, 0.01, 0.05, 0.25, 0.5, 1, 5, 25},
					},
				},
			),
			metric.NewView(
				metric.Instrument{
					Name:  "time_to_first_byte",
					Scope: instrumentation.Scope{Name: meterName},
				},
				metric.Stream{
					Aggregation: aggregation.ExplicitBucketHistogram{
						Boundaries: []float64{0, 0.01, 0.05, 0.25, 0.5, 1, 5, 25, 75},
					},
				},
			),
			metric.NewView(
				metric.Instrument{
					Name:  "retrieval_deal_size_bytes",
					Scope: instrumentation.Scope{Name: meterName},
				},
				metric.Stream{
					Aggregation: aggregation.ExplicitBucketHistogram{
						Boundaries: []float64{0, 1 << 18, 1 << 20, 1 << 22, 1 << 24, 1 << 28, 1 << 30, 1 << 35},
					},
				},
			),
			metric.NewView(
				metric.Instrument{
					Name:  "bandwidth_bytes_per_second",
					Scope: instrumentation.Scope{Name: meterName},
				},
				metric.Stream{
					Aggregation: aggregation.ExplicitBucketHistogram{
						Boundaries: []float64{0, 1 << 14, 1 << 18, 1 << 20, 1 << 22, 1 << 24, 1 << 27},
					},
				},
			),
		),
	)
	meter := provider.Meter(meterName)

	// funnel

	if m.totalRequestCount, err = meter.Int64Counter("total_request_count",
		instrument.WithDescription("distinc retrievals sent to Lassie on Saturn"),
	); err != nil {
		return err
	}
	if m.requestWithIndexerFailures, err = meter.Int64Counter("requests_with_indexer_failures",
		instrument.WithDescription("failures at the indexer phase"),
	); err != nil {
		return err
	}
	if m.requestWithIndexerCandidatesCount, err = meter.Int64Counter("request_with_indexer_candidates_total",
		instrument.WithDescription("The number of requests that result in non-zero candidates from the indexer"),
	); err != nil {
		return err
	}
	if m.requestWithIndexerCandidatesFilteredCount, err = meter.Int64Counter("request_with_indexer_candidates_filtered_total",
		instrument.WithDescription("The number of requests that result in non-zero candidates from the indexer after filtering"),
	); err != nil {
		return err
	}
	if m.requestWithBitswapAttempt, err = meter.Int64Counter("request_with_bitswap_attempts",
		instrument.WithDescription("The number of requests where a bitswap retrieval was attempted"),
	); err != nil {
		return err
	}
	if m.requestWithGraphSyncAttempt, err = meter.Int64Counter("request_with_graphsync_attempts",
		instrument.WithDescription("The number of requests where a graphsync retrieval was attempted"),
	); err != nil {
		return err
	}
	if m.requestWithFirstByteReceivedCount, err = meter.Int64Counter("request_with_first_byte_received",
		instrument.WithDescription("The number of requests where a non-zero number of bytes were received"),
	); err != nil {
		return err
	}
	if m.requestWithSuccessCount, err = meter.Int64Counter("request_with_success",
		instrument.WithDescription("The number of successful retrievals via lassie (all bytes received)"),
	); err != nil {
		return err
	}
	if m.requestWithBitswapSuccessCount, err = meter.Int64Counter("request_with_bitswap_success",
		instrument.WithDescription("The number of successful retrievals via lassie (all bytes received) over bitswap"),
	); err != nil {
		return err
	}
	if m.requestWithGraphSyncSuccessCount, err = meter.Int64Counter("request_with_graphsync_success",
		instrument.WithDescription("The number of successful retrievals via lassie (all bytes received) over graphsync"),
	); err != nil {
		return err
	}

	// stats

	if m.timeToFirstIndexerResult, err = meter.Float64Histogram("time_to_first_indexer_result",
		instrument.WithDescription("The time to to first indexer result in seconds"),
		instrument.WithUnit("seconds"),
	); err != nil {
		return err
	}
	if m.timeToFirstByte, err = meter.Float64Histogram("time_to_first_byte",
		instrument.WithDescription("The time to to first byte in seconds"),
		instrument.WithUnit("seconds"),
	); err != nil {
		return err
	}
	if m.bandwidthBytesPerSecond, err = meter.Int64Histogram("bandwidth_bytes_per_second",
		instrument.WithDescription("average bytes transferred per second"),
		instrument.WithUnit("seconds"),
	); err != nil {
		return err
	}
	if m.retrievalDealSize, err = meter.Int64Histogram("retrieval_deal_size_bytes",
		instrument.WithDescription("The size in bytes of a retrieval deal with a storage provider"),
		instrument.WithUnit("bytes"),
	); err != nil {
		return err
	}
	if m.retrievalDealDuration, err = meter.Float64Histogram("retrieval_deal_duration_seconds",
		instrument.WithDescription("The duration in seconds of a retrieval deal with a storage provider"),
		instrument.WithUnit("seconds"),
	); err != nil {
		return err
	}

	// errors
	if m.retrievalErrorRejectedCount, err = meter.Int64Counter("retrieval_error_rejected_total",
		instrument.WithDescription("The number of retrieval errors for 'response rejected'"),
	); err != nil {
		return err
	}
	if m.retrievalErrorTooManyCount, err = meter.Int64Counter("retrieval_error_toomany_total",
		instrument.WithDescription("The number of retrieval errors for 'Too many retrieval deals received'"),
	); err != nil {
		return err
	}
	if m.retrievalErrorACLCount, err = meter.Int64Counter("retrieval_error_acl_total",
		instrument.WithDescription("The number of retrieval errors for 'Access Control'"),
	); err != nil {
		return err
	}
	if m.retrievalErrorMaintenanceCount, err = meter.Int64Counter("retrieval_error_maintenance_total",
		instrument.WithDescription("The number of retrieval errors for 'Under maintenance, retry later'"),
	); err != nil {
		return err
	}
	if m.retrievalErrorNoOnlineCount, err = meter.Int64Counter("retrieval_error_noonline_total",
		instrument.WithDescription("The number of retrieval errors for 'miner is not accepting online retrieval deals'"),
	); err != nil {
		return err
	}
	if m.retrievalErrorUnconfirmedCount, err = meter.Int64Counter("retrieval_error_unconfirmed_total",
		instrument.WithDescription("The number of retrieval errors for 'unconfirmed block transfer'"),
	); err != nil {
		return err
	}
	if m.retrievalErrorTimeoutCount, err = meter.Int64Counter("retrieval_error_timeout_total",
		instrument.WithDescription("The number of retrieval errors for 'timeout after X'"),
	); err != nil {
		return err
	}
	if m.retrievalErrorOtherCount, err = meter.Int64Counter("retrieval_error_other_total",
		instrument.WithDescription("The number of retrieval errors with uncategorized causes"),
	); err != nil {
		return err
	}

	// averages
	if m.indexerCandidatesPerRequestCount, err = meter.Int64Histogram("indexer_candidates_per_request_total",
		instrument.WithDescription("The number of indexer candidates received per request"),
	); err != nil {
		return err
	}
	if m.indexerCandidatesFilteredPerRequestCount, err = meter.Int64Histogram("indexer_candidates_filtered_per_request_total",
		instrument.WithDescription("The number of filtered indexer candidates received per request"),
	); err != nil {
		return err
	}
	if m.failedRetrievalsPerRequestCount, err = meter.Int64Histogram("failed_retrievals_per_request_total",
		instrument.WithDescription("The number of failed retrieval attempts per request"),
	); err != nil {
		return err
	}

	return nil
}

// Measures
type stats struct {
	// funnel
	totalRequestCount                         instrument.Int64Counter
	requestWithIndexerFailures                instrument.Int64Counter
	requestWithIndexerCandidatesCount         instrument.Int64Counter
	requestWithIndexerCandidatesFilteredCount instrument.Int64Counter
	requestWithBitswapAttempt                 instrument.Int64Counter
	requestWithGraphSyncAttempt               instrument.Int64Counter
	requestWithFirstByteReceivedCount         instrument.Int64Counter
	requestWithSuccessCount                   instrument.Int64Counter
	requestWithBitswapSuccessCount            instrument.Int64Counter
	requestWithGraphSyncSuccessCount          instrument.Int64Counter

	// stats
	timeToFirstIndexerResult instrument.Float64Histogram
	timeToFirstByte          instrument.Float64Histogram
	retrievalDealDuration    instrument.Float64Histogram
	bandwidthBytesPerSecond  instrument.Int64Histogram
	retrievalDealSize        instrument.Int64Histogram

	// error kinds
	retrievalErrorRejectedCount    instrument.Int64Counter
	retrievalErrorTooManyCount     instrument.Int64Counter
	retrievalErrorACLCount         instrument.Int64Counter
	retrievalErrorMaintenanceCount instrument.Int64Counter
	retrievalErrorNoOnlineCount    instrument.Int64Counter
	retrievalErrorUnconfirmedCount instrument.Int64Counter
	retrievalErrorTimeoutCount     instrument.Int64Counter
	retrievalErrorOtherCount       instrument.Int64Counter

	// averages
	indexerCandidatesPerRequestCount         instrument.Int64Histogram
	indexerCandidatesFilteredPerRequestCount instrument.Int64Histogram
	failedRetrievalsPerRequestCount          instrument.Int64Histogram
}
