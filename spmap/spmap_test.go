package spmap_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/filecoin-project/lassie-event-recorder/spmap"
	"github.com/filecoin-project/lassie-event-recorder/spmap/testutil"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestSPMap(t *testing.T) {
	ts := httptest.NewServer(testutil.MockHeyfilHandler)
	defer ts.Close()
	spm := spmap.NewSPMap(spmap.WithHeyFil(ts.URL))
	var pid peer.ID
	pid.UnmarshalText([]byte(testutil.TestPeerID))
	ch := spm.Get(context.Background(), pid)
	sid, ok := <-ch
	if sid != testutil.TestSPID {
		t.Fatalf("unexpected response; got: %v / %t", sid, ok)
	}
}
