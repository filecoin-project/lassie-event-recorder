//go:build integration

package eventrecorder_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/lassie-event-recorder/eventrecorder"
	"github.com/filecoin-project/lassie-event-recorder/httpserver"
	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestPostEvent(t *testing.T) {
	var dbdsn = "postgres://user:passwd@localhost:5432/LassieEvents"
	if v, ok := os.LookupEnv("LASSIE_EVENT_RECORDER_DB_DSN"); ok {
		dbdsn = v
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbdsn)
	require.NoError(t, err)
	defer db.Close()

	schema, err := os.ReadFile("../schema.sql")
	require.NoError(t, err)
	_, err = db.Exec(ctx, string(schema))
	require.NoError(t, err)

	recorder, err := eventrecorder.New(
		eventrecorder.WithDatabaseDSN(dbdsn),
	)
	require.NoError(t, err)

	server, err := httpserver.NewHttpServer(recorder)
	require.NoError(t, err)

	require.NoError(t, server.Start(ctx))
	defer func() {
		server.Shutdown(ctx)
	}()

	encEventBatch, err := os.ReadFile("../testdata/good.json")
	require.NoError(t, err)

	resp, err := http.Post(
		"http://localhost:8080/v1/retrieval-events",
		"application/json",
		bytes.NewReader(encEventBatch))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	rows, err := db.Query(ctx, `select 
										retrieval_id,
										instance_id,
										cid,
										storage_provider_id,
										phase,
										phase_start_time,
										event_name,
										event_time from retrieval_events`)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())
	var e struct {
		RetrievalId       pgtype.UUID
		InstanceId        string
		Cid               string
		StorageProviderId string
		Phase             types.Phase
		PhaseStartTime    time.Time
		EventName         types.EventCode
		EventTime         time.Time
	}
	require.NoError(t, rows.Scan(
		&e.RetrievalId,
		&e.InstanceId,
		&e.Cid,
		&e.StorageProviderId,
		&e.Phase,
		&e.PhaseStartTime,
		&e.EventName,
		&e.EventTime,
	))

	var wantEventBatch eventrecorder.EventBatch
	require.NoError(t, json.Unmarshal(encEventBatch, &wantEventBatch))
	require.Len(t, wantEventBatch.Events, 2)
	wantEvent := wantEventBatch.Events[0]

	require.Equal(t, [16]byte(wantEvent.RetrievalId), e.RetrievalId.Bytes)
	require.Equal(t, wantEvent.InstanceId, e.InstanceId)
	require.Equal(t, wantEvent.Cid, e.Cid)
	require.Equal(t, wantEvent.StorageProviderId, e.StorageProviderId)
	require.Equal(t, wantEvent.Phase, e.Phase)
	require.Equal(t, wantEvent.PhaseStartTime.UnixMicro(), e.PhaseStartTime.UnixMicro())
	require.Equal(t, wantEvent.EventName, e.EventName)
	require.Equal(t, wantEvent.EventTime.UnixMicro(), e.EventTime.UnixMicro())

	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(
		&e.RetrievalId,
		&e.InstanceId,
		&e.Cid,
		&e.StorageProviderId,
		&e.Phase,
		&e.PhaseStartTime,
		&e.EventName,
		&e.EventTime,
	))

	wantEvent = wantEventBatch.Events[1]
	require.Equal(t, [16]byte(wantEvent.RetrievalId), e.RetrievalId.Bytes)
	require.Equal(t, wantEvent.InstanceId, e.InstanceId)
	require.Equal(t, wantEvent.Cid, e.Cid)
	require.Equal(t, wantEvent.StorageProviderId, e.StorageProviderId)
	require.Equal(t, wantEvent.Phase, e.Phase)
	require.Equal(t, wantEvent.PhaseStartTime.UnixMicro(), e.PhaseStartTime.UnixMicro())
	require.Equal(t, wantEvent.EventName, e.EventName)
	require.Equal(t, wantEvent.EventTime.UnixMicro(), e.EventTime.UnixMicro())

	require.False(t, rows.Next())
}
