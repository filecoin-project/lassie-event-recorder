package eventrecorder

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_EventValidate(t *testing.T) {
	tests := []struct {
		path              string
		batch             bool
		wantDecodeErr     bool
		wantValidationErr bool
	}{
		{
			path:              "../testdata/bad-eventName.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			path:              "../testdata/bad-phase.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			path:              "../testdata/bad-retrievalid.json",
			batch:             true,
			wantDecodeErr:     true,
			wantValidationErr: true,
		},
		{
			path:  "../testdata/good.json",
			batch: true,
		},
		{
			path:              "../testdata/missing-events.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			path:              "../testdata/missing-instanceid.json",
			batch:             true,
			wantValidationErr: true,
		},
		{
			path:              "../testdata/missing-retrievalid.json",
			batch:             true,
			wantValidationErr: true,
		},
	}
	for _, test := range tests {
		t.Run(path.Base(test.path), func(t *testing.T) {
			f, err := os.Open(test.path)
			t.Cleanup(func() {
				require.NoError(t, f.Close())
			})
			require.NoError(t, err)

			given, err := io.ReadAll(f)
			require.NoError(t, err)

			var subject interface {
				validate() error
			}
			var gotDecodeErr error
			if test.batch {
				var eb eventBatch
				gotDecodeErr = json.Unmarshal(given, &eb)
				subject = eb
			} else {
				var e event
				gotDecodeErr = json.Unmarshal(given, &e)
				subject = e
			}

			if test.wantDecodeErr {
				require.NotNil(t, gotDecodeErr)
			} else {
				require.NoError(t, gotDecodeErr)
				if gotErr := subject.validate(); test.wantValidationErr {
					require.NotNil(t, gotErr)
				} else {
					require.NoError(t, gotErr)
				}
			}
		})
	}

}
