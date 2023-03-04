package eventrecorder

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_EventValidate(t *testing.T) {
	tests := []struct {
		name              string
		path              string
		batch             bool
		wantDecodeErr     bool
		wantValidationErr bool
	}{
		{
			name:              "bad-eventName",
			path:              "../testdata/bad-eventName.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "bad-phase",
			path:              "../testdata/bad-phase.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "bad-retrievalid",
			path:              "../testdata/bad-retrievalid.json",
			batch:             true,
			wantDecodeErr:     true,
			wantValidationErr: true,
		},
		{
			name:  "good",
			path:  "../testdata/good.json",
			batch: true,
		},
		{
			name:              "invalid-cid",
			path:              "../testdata/invalid-cid.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "future-start-time",
			path:              "../testdata/future-start-time.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "future-end-time",
			path:              "../testdata/future-event-time.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "invalid-provider-id",
			path:              "../testdata/invalid-providerid.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "missing-events",
			path:              "../testdata/missing-events.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "missing-events-empty-array",
			path:              "../testdata/missing-events-empty-array.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "missing-event",
			path:              "../testdata/missing-events.json",
			wantValidationErr: true,
		},
		{
			name:              "missing-instanceid",
			path:              "../testdata/missing-instanceid.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "missing-event-time",
			path:              "../testdata/missing-event-time.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			name:              "missing-retrievalid",
			path:              "../testdata/missing-retrievalid.json",
			batch:             true,
			wantValidationErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, err := os.Open(test.path)
			t.Cleanup(func() {
				require.NoError(t, f.Close())
			})
			require.NoError(t, err)

			given, err := io.ReadAll(f)
			require.NoError(t, err)

			var subject interface {
				Validate() error
			}
			var gotDecodeErr error
			if test.batch {
				var eb EventBatch
				gotDecodeErr = json.Unmarshal(given, &eb)
				subject = eb
			} else {
				var e Event
				gotDecodeErr = json.Unmarshal(given, &e)
				subject = e
			}

			if test.wantDecodeErr {
				require.NotNil(t, gotDecodeErr)
			} else {
				require.NoError(t, gotDecodeErr)
				if gotErr := subject.Validate(); test.wantValidationErr {
					require.NotNil(t, gotErr)
				} else {
					require.NoError(t, gotErr)
				}
			}
		})
	}

}
