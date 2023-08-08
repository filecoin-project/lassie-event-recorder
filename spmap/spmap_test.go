package spmap_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/filecoin-project/lassie-event-recorder/spmap"
	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	TestPeerID = "12D3KooWDGBkHBZye7rN6Pz9ihEZrHnggoVRQh6eEtKP4z1K4KeE"
	TestSPID   = "f01228000"
)

var MockHeyfilHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/sp" {
		http.Error(w, "nope, bad path", http.StatusBadRequest)
		return
	}
	if r.URL.RawQuery != "peerid="+TestPeerID {
		http.Error(w, "nope, bad query", http.StatusBadRequest)
		return
	}
	fmt.Fprintf(w, `["%s"]\n`, TestSPID)
})

func TestSPMap(t *testing.T) {
	ts := httptest.NewServer(MockHeyfilHandler)
	defer ts.Close()
	spm := spmap.NewSPMap(spmap.WithHeyFil(ts.URL))
	var pid peer.ID
	pid.UnmarshalText([]byte(TestPeerID))
	ch := spm.Get(context.Background(), pid)
	sid, ok := <-ch
	if sid != TestSPID {
		t.Fatalf("unexpected response; got: %v / %t", sid, ok)
	}
}
