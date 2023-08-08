package eventrecorder_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/filecoin-project/lassie-event-recorder/eventrecorder"
	"github.com/filecoin-project/lassie-event-recorder/httpserver"
	"github.com/filecoin-project/lassie-event-recorder/metrics"
	"github.com/filecoin-project/lassie-event-recorder/spmap"
	spmaptestutil "github.com/filecoin-project/lassie-event-recorder/spmap/testutil"
	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/stretchr/testify/require"
)

var expectedEvents = []ae{
	// bitswap success
	{
		timeToFirstIndexerResult: 10 * time.Millisecond,
		timeToFirstByte:          40 * time.Millisecond,
		success:                  true,
		storageProviderID:        "Bitswap",
		filSPID:                  "",
		startTime:                time.Unix(0, 0).In(time.FixedZone("UTC+10", 10*60*60)),
		endTime:                  time.Unix(0, 0).Add(90 * time.Millisecond).In(time.FixedZone("UTC+10", 10*60*60)),
		bandwidth:                200000,
		bytesTransferred:         10000,
		indexerCandidates:        5,
		indexerFiltered:          3,
		protocolSucceeded:        "transport-bitswap",
		attempts: map[string]metrics.Attempt{
			"12D3KooWEqwTBN3GE4vT6DWZiKpq24UtSBmhhwM73vg7SfTjYWaF": {
				Error:           "",
				Protocol:        "transport-graphsync-filecoinv1",
				TimeToFirstByte: 50 * time.Millisecond,
			},
			"12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNzbm5akpqu": {
				Error:    "failed to dial",
				Protocol: "transport-graphsync-filecoinv1",
			},
			"Bitswap": {
				Error:           "",
				Protocol:        "transport-bitswap",
				TimeToFirstByte: 20 * time.Millisecond,
			},
		},
	},
	// failure
	{
		timeToFirstIndexerResult: time.Duration(0),
		timeToFirstByte:          time.Duration(0),
		success:                  false,
		storageProviderID:        "",
		filSPID:                  "",
		startTime:                time.Unix(0, 0).In(time.FixedZone("UTC+10", 10*60*60)),
		endTime:                  time.Unix(0, 0).In(time.FixedZone("UTC+10", 10*60*60)),
		bandwidth:                0,
		bytesTransferred:         0,
		indexerCandidates:        0,
		indexerFiltered:          0,
		protocolSucceeded:        "",
		attempts:                 map[string]metrics.Attempt{},
	},
	// http success
	{
		timeToFirstIndexerResult: 20 * time.Millisecond,
		timeToFirstByte:          80 * time.Millisecond,
		success:                  true,
		storageProviderID:        "12D3KooWDGBkHBZye7rN6Pz9ihEZrHnggoVRQh6eEtKP4z1K4KeE", // same as smpap/testutil/TestPeerID
		filSPID:                  "f01228000",                                            // from Heyfil, same as smpap/testutil/TestSPID
		startTime:                time.Unix(0, 0).Add(20 * time.Millisecond).In(time.FixedZone("UTC+10", 10*60*60)),
		endTime:                  time.Unix(0, 0).Add(180 * time.Millisecond).In(time.FixedZone("UTC+10", 10*60*60)),
		bandwidth:                300000,
		bytesTransferred:         20000,
		indexerCandidates:        10,
		indexerFiltered:          6,
		protocolSucceeded:        "transport-ipfs-gateway-http",
		attempts: map[string]metrics.Attempt{
			"12D3KooWDGBkHBZye7rN6Pz9ihEZrHnggoVRQh6eEtKP4z1K4KeE": {
				Error:           "",
				Protocol:        "transport-ipfs-gateway-http",
				TimeToFirstByte: 100 * time.Millisecond,
			},
			"12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNzbm5akpqu": {
				Error:    "failed to dial",
				Protocol: "transport-graphsync-filecoinv1",
			},
			"Bitswap": {
				Error:           "",
				Protocol:        "transport-bitswap",
				TimeToFirstByte: 200 * time.Millisecond,
			},
		},
	},
}

func TestRecorderMetrics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := require.New(t)

	spmapts := httptest.NewServer(spmaptestutil.MockHeyfilHandler)
	defer spmapts.Close()

	mm := &mockMetrics{t: t}
	recorder, err := eventrecorder.New(eventrecorder.WithMetrics(mm), eventrecorder.WithSPMapOptions(spmap.WithHeyFil(spmapts.URL)))
	req.NoError(err)

	handler := httpserver.NewHttpHandler(recorder)
	evtts := httptest.NewServer(handler.Handler())
	defer evtts.Close()

	req.NoError(handler.Start(ctx))

	encEventBatch, err := os.ReadFile("../testdata/aggregategood.json")
	req.NoError(err)

	resp, err := http.Post(
		evtts.URL+"/v2/retrieval-events",
		"application/json",
		bytes.NewReader(encEventBatch))
	req.NoError(err)
	body, err := io.ReadAll(resp.Body)
	req.NoError(err)
	req.Equal(http.StatusOK, resp.StatusCode, string(body))
	req.Len(body, 0)

	req.Len(mm.aggregatedEvents, len(expectedEvents))
	for ii, ee := range expectedEvents {
		req.Equal(ee.timeToFirstIndexerResult, mm.aggregatedEvents[ii].timeToFirstIndexerResult)
		req.Equal(ee.timeToFirstByte, mm.aggregatedEvents[ii].timeToFirstByte)
		req.Equal(ee.success, mm.aggregatedEvents[ii].success)
		req.Equal(ee.storageProviderID, mm.aggregatedEvents[ii].storageProviderID)
		req.Equal(ee.filSPID, mm.aggregatedEvents[ii].filSPID)
		req.Equal(ee.startTime.UTC(), mm.aggregatedEvents[ii].startTime.UTC())
		req.Equal(ee.endTime.UTC(), mm.aggregatedEvents[ii].endTime.UTC())
		req.Equal(ee.bandwidth, mm.aggregatedEvents[ii].bandwidth)
		req.Equal(ee.bytesTransferred, mm.aggregatedEvents[ii].bytesTransferred)
		req.Equal(ee.indexerCandidates, mm.aggregatedEvents[ii].indexerCandidates)
		req.Equal(ee.indexerFiltered, mm.aggregatedEvents[ii].indexerFiltered)
		req.Equal(ee.protocolSucceeded, mm.aggregatedEvents[ii].protocolSucceeded)
		req.Len(mm.aggregatedEvents[ii].attempts, len(ee.attempts))
		for k, aa := range ee.attempts {
			req.Contains(mm.aggregatedEvents[ii].attempts, k)
			req.Equal(aa.Error, mm.aggregatedEvents[ii].attempts[k].Error)
			req.Equal(aa.Protocol, mm.aggregatedEvents[ii].attempts[k].Protocol)
			req.Equal(aa.TimeToFirstByte, mm.aggregatedEvents[ii].attempts[k].TimeToFirstByte)
		}
	}
}

type mockMetrics struct {
	t                *testing.T
	aggregatedEvents []ae
}

func (mm *mockMetrics) HandleStartedEvent(context.Context, types.RetrievalID, types.Phase, time.Time, string) {
	require.Fail(mm.t, "unexpected HandleStartedEvent call")
}

func (mm *mockMetrics) HandleCandidatesFoundEvent(context.Context, types.RetrievalID, time.Time, any) {
	require.Fail(mm.t, "unexpected HandleCandidatesFoundEvent call")
}

func (mm *mockMetrics) HandleCandidatesFilteredEvent(context.Context, types.RetrievalID, any) {
	require.Fail(mm.t, "unexpected HandleCandidatesFilteredEvent call")
}

func (mm *mockMetrics) HandleFailureEvent(context.Context, types.RetrievalID, types.Phase, string, any) {
	require.Fail(mm.t, "unexpected HandleFailureEvent call")
}

func (mm *mockMetrics) HandleTimeToFirstByteEvent(context.Context, types.RetrievalID, string, time.Time) {
	require.Fail(mm.t, "unexpected HandleTimeToFirstByteEvent call")
}

func (mm *mockMetrics) HandleSuccessEvent(context.Context, types.RetrievalID, time.Time, string, any) {
	require.Fail(mm.t, "unexpected HandleSuccessEvent call")
}

func (mm *mockMetrics) HandleAggregatedEvent(
	ctx context.Context,
	timeToFirstIndexerResult time.Duration,
	timeToFirstByte time.Duration,
	success bool,
	storageProviderID string, // Lassie Peer ID
	filSPID string, // Heyfil Filecoin SP ID
	startTime time.Time,
	endTime time.Time,
	bandwidth int64,
	bytesTransferred int64,
	indexerCandidates int64,
	indexerFiltered int64,
	attempts map[string]metrics.Attempt,
	protocolSucceeded string,
) {
	if mm.aggregatedEvents == nil {
		mm.aggregatedEvents = make([]ae, 0)
	}
	mm.aggregatedEvents = append(mm.aggregatedEvents, ae{
		timeToFirstIndexerResult,
		timeToFirstByte,
		success,
		storageProviderID,
		filSPID,
		startTime,
		endTime,
		bandwidth,
		bytesTransferred,
		indexerCandidates,
		indexerFiltered,
		attempts,
		protocolSucceeded,
	})
}

type ae struct {
	timeToFirstIndexerResult time.Duration
	timeToFirstByte          time.Duration
	success                  bool
	storageProviderID        string // Lassie Peer ID
	filSPID                  string // Heyfil Filecoin SP ID
	startTime                time.Time
	endTime                  time.Time
	bandwidth                int64
	bytesTransferred         int64
	indexerCandidates        int64
	indexerFiltered          int64
	attempts                 map[string]metrics.Attempt
	protocolSucceeded        string
}

func (a ae) String() string {
	attempts := strings.Builder{}
	for k, v := range a.attempts {
		attempts.WriteString(fmt.Sprintf("\t%s: error=%v, protocol=%v, ttfb=%v\n", k, v.Error, v.Protocol, v.TimeToFirstByte))
	}

	return fmt.Sprintf(
		"timeToFirstIndexerResult: %v\n"+
			"timeToFirstByte: %v\n"+
			"success: %v\n"+
			"storageProviderID: %s\n"+
			"filSPID: %s\n"+
			"startTime: %v\n"+
			"endTime: %v\n"+
			"bandwidth: %d\n"+
			"bytesTransferred: %d\n"+
			"indexerCandidates: %d\n"+
			"indexerFiltered: %d\n"+
			"attempts:\n%s"+
			"protocolSucceeded: %s",
		a.timeToFirstIndexerResult,
		a.timeToFirstByte,
		a.success,
		a.storageProviderID,
		a.filSPID,
		a.startTime,
		a.endTime,
		a.bandwidth,
		a.bytesTransferred,
		a.indexerCandidates,
		a.indexerFiltered,
		attempts.String(),
		a.protocolSucceeded,
	)
}
