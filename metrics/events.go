package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/filecoin-project/lassie/pkg/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
)

// HandleFailureEvent is called when a query _or_ retrieval fails
func (m *Metrics) HandleFailureEvent(ctx context.Context, id types.RetrievalID, phase types.Phase, storageProviderID string, details interface{}) {

	detailsObj, ok := details.(map[string]interface{})
	if !ok {
		return
	}
	msg, ok := detailsObj["error"].(string)
	if !ok {
		return
	}
	switch phase {
	case types.IndexerPhase:
		tempData := m.tempDataMap.GetOrCreate(id)
		tempData.RecordFinality()
		_ = m.tempDataMap.Delete(id)
		m.requestWithIndexerFailures.Add(ctx, 1)
	case types.RetrievalPhase:
		if storageProviderID != types.BitswapIndentifier {
			m.graphsyncRetrievalFailureCount.Add(ctx, 1, attribute.String("sp_id", storageProviderID))
		}
		m.matchErrorMessage(ctx, msg, storageProviderID)
	}
}

func (m *Metrics) HandleStartedEvent(ctx context.Context, id types.RetrievalID, phase types.Phase, eventTime time.Time, storageProviderID string) {
	tempData := m.tempDataMap.GetOrCreate(id)
	switch phase {
	case types.IndexerPhase:
		tempData.RecordStartTime(eventTime)
		m.totalRequestCount.Add(ctx, 1)
	case types.RetrievalPhase:
		if storageProviderID == types.BitswapIndentifier {
			if tempData.RecordBitswapAttempt() {
				m.requestWithBitswapAttempt.Add(ctx, 1)
			}
		} else {
			if tempData.RecordGraphsyncAttempt() {
				m.requestWithGraphSyncAttempt.Add(ctx, 1, attribute.String("sp_id", storageProviderID))
			}
		}
	}
}

func (m *Metrics) HandleCandidatesFoundEvent(ctx context.Context, id types.RetrievalID, eventTime time.Time, details interface{}) {
	detailsObj, ok := details.(map[string]interface{})
	if !ok {
		return
	}

	candidateCount, ok := detailsObj["candidateCount"].(float64)
	if !ok {
		return
	}

	if candidateCount > 0 {
		tempData := m.tempDataMap.GetOrCreate(id)
		if tempData.RecordIndexerCandidates(eventTime, uint32(candidateCount)) {
			m.requestWithIndexerCandidatesCount.Add(ctx, 1)
			m.timeToFirstIndexerResult.Record(ctx, eventTime.Sub(tempData.StartTime()).Seconds())
		}
	}
}

func (m *Metrics) HandleCandidatesFilteredEvent(ctx context.Context, id types.RetrievalID, details interface{}) {
	detailsObj, ok := details.(map[string]interface{})
	if !ok {
		return
	}

	candidateCount, ok := detailsObj["candidateCount"].(float64)
	if !ok {
		return
	}

	if candidateCount > 0 {
		tempData := m.tempDataMap.GetOrCreate(id)
		if tempData.RecordIndexerFilteredCandidates(uint32(candidateCount)) {
			m.requestWithIndexerCandidatesFilteredCount.Add(ctx, 1)
		}
	}
}

func (m *Metrics) HandleTimeToFirstByteEvent(ctx context.Context, id types.RetrievalID, storageProviderId string, eventTime time.Time) {
	tempData := m.tempDataMap.GetOrCreate(id)
	if tempData.RecordTimeToFirstByte(eventTime) {
		m.requestWithFirstByteReceivedCount.Add(ctx, 1, attribute.String("protocol", protocol(storageProviderId)))
		m.timeToFirstByte.Record(ctx, eventTime.Sub(tempData.StartTime()).Seconds(), attribute.String("protocol", protocol(storageProviderId)))
	}
}

func (m *Metrics) HandleSuccessEvent(ctx context.Context, id types.RetrievalID, eventTime time.Time, storageProviderId string, details interface{}) {
	detailsObj, ok := details.(map[string]interface{})
	if !ok {
		return
	}

	receivedSize, ok := detailsObj["receivedSize"].(float64)
	if !ok {
		return
	}

	tempData := m.tempDataMap.GetOrCreate(id)
	tempData.RecordFinality()
	finalDetails := m.tempDataMap.Delete(id)
	m.requestWithSuccessCount.Add(ctx, 1)
	if storageProviderId == types.BitswapIndentifier {
		m.requestWithBitswapSuccessCount.Add(ctx, 1)
	} else {
		m.requestWithGraphSyncSuccessCount.Add(ctx, 1, attribute.String("sp_id", storageProviderId))
	}

	// stats
	m.retrievalDealDuration.Record(ctx, eventTime.Sub(finalDetails.StartTime).Seconds(), attribute.String("protocol", protocol(storageProviderId)))
	m.retrievalDealSize.Record(ctx, int64(receivedSize), attribute.String("protocol", protocol(storageProviderId)))
	transferDuration := eventTime.Sub(finalDetails.TimeToFirstByte).Seconds()
	m.bandwidthBytesPerSecond.Record(ctx, int64(receivedSize/transferDuration), attribute.String("protocol", protocol(storageProviderId)))

	// averages
	m.indexerCandidatesPerRequestCount.Record(ctx, int64(finalDetails.IndexerCandidates))
	m.indexerCandidatesFilteredPerRequestCount.Record(ctx, int64(finalDetails.IndexerFiltered))
	m.failedRetrievalsPerRequestCount.Record(ctx, int64(finalDetails.FailedCount))
}

func (m *Metrics) HandleAggregatedEvent(ctx context.Context,
	timeToFirstIndexerResult time.Duration,
	timeToFirstByte time.Duration,
	success bool,
	storageProviderID string,
	startTime time.Time,
	endTime time.Time,
	bandwidth int64,
	bytesTransferred int64,
	indexerCandidates int64,
	indexerFiltered int64,
	attempts map[string]string) {
	m.totalRequestCount.Add(ctx, 1)
	failureCount := 0
	var recordedGraphSync, recordedBitswap bool
	for storageProviderID, msg := range attempts {
		if storageProviderID == types.BitswapIndentifier {
			if !recordedBitswap {
				recordedBitswap = true
				m.requestWithBitswapAttempt.Add(ctx, 1)
			}
		} else {
			if !recordedGraphSync {
				recordedGraphSync = true
				m.requestWithGraphSyncAttempt.Add(ctx, 1, attribute.String("sp_id", storageProviderID))
			}
		}
		if msg != "" {
			if storageProviderID != types.BitswapIndentifier {
				m.graphsyncRetrievalFailureCount.Add(ctx, 1, attribute.String("sp_id", storageProviderID))
			}
			m.matchErrorMessage(ctx, msg, storageProviderID)
			failureCount += 0
		}
	}
	if timeToFirstIndexerResult > 0 {
		m.timeToFirstIndexerResult.Record(ctx, timeToFirstIndexerResult.Seconds())
	}
	if indexerCandidates > 0 {
		m.requestWithIndexerCandidatesCount.Add(ctx, 1)
	}
	if indexerFiltered > 0 {
		m.requestWithIndexerCandidatesFilteredCount.Add(ctx, 1)
	}
	if timeToFirstByte > 0 {
		m.requestWithFirstByteReceivedCount.Add(ctx, 1, attribute.String("protocol", protocol(storageProviderID)))
		m.timeToFirstByte.Record(ctx, timeToFirstByte.Seconds(), attribute.String("protocol", protocol(storageProviderID)))
	}
	if success {
		m.requestWithSuccessCount.Add(ctx, 1)
		if storageProviderID == types.BitswapIndentifier {
			m.requestWithBitswapSuccessCount.Add(ctx, 1)
		} else {
			m.requestWithGraphSyncSuccessCount.Add(ctx, 1, attribute.String("sp_id", storageProviderID))
		}

		m.retrievalDealDuration.Record(ctx, endTime.Sub(startTime).Seconds(), attribute.String("protocol", protocol(storageProviderID)))
		m.retrievalDealSize.Record(ctx, bytesTransferred, attribute.String("protocol", protocol(storageProviderID)))
		m.bandwidthBytesPerSecond.Record(ctx, bandwidth, attribute.String("protocol", protocol(storageProviderID)))

		m.indexerCandidatesPerRequestCount.Record(ctx, indexerCandidates)
		m.indexerCandidatesFilteredPerRequestCount.Record(ctx, indexerFiltered)
		m.failedRetrievalsPerRequestCount.Record(ctx, int64(failureCount))
	} else if len(attempts) == 0 {
		m.requestWithIndexerFailures.Add(ctx, 1)
	}
}

func (m *Metrics) matchErrorMessage(ctx context.Context, msg string, storageProviderID string) {
	var errorMetricMatches = map[string]instrument.Int64Counter{
		"response rejected":                                 m.retrievalErrorRejectedCount,
		"Too many retrieval deals received":                 m.retrievalErrorTooManyCount,
		"Access Control":                                    m.retrievalErrorACLCount,
		"Under maintenance, retry later":                    m.retrievalErrorMaintenanceCount,
		"miner is not accepting online retrieval deals":     m.retrievalErrorNoOnlineCount,
		"unconfirmed block transfer":                        m.retrievalErrorUnconfirmedCount,
		"timeout after ":                                    m.retrievalErrorTimeoutCount,
		"there is no unsealed piece containing payload cid": m.retrievalErrorNoUnsealedCount,
		"getting pieces for cid":                            m.retrievalErrorDAGStoreCount,
		"graphsync request failed to complete: request failed - unknown reason": m.retrievalErrorGraphsyncCount,
		"failed to dial": m.retrievalErrorFailedToDialCount,
	}

	var matched bool
	for substr, metric := range errorMetricMatches {
		if strings.Contains(msg, substr) {
			metric.Add(ctx, 1, attribute.String("protocol", protocol(storageProviderID)))
			matched = true
			break
		}
	}
	if !matched {
		m.retrievalErrorOtherCount.Add(ctx, 1, attribute.String("protocol", protocol(storageProviderID)))
	}
}
func protocol(storageProviderId string) string {
	if storageProviderId == types.BitswapIndentifier {
		return "bitswap"
	}
	return "graphsync"
}
