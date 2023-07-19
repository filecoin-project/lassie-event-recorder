package spmap_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/lassie-event-recorder/spmap"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestSPMap(t *testing.T) {
	spm := spmap.NewSPMap()
	var pid peer.ID
	pid.UnmarshalText([]byte("12D3KooWDGBkHBZye7rN6Pz9ihEZrHnggoVRQh6eEtKP4z1K4KeE"))
	ch := spm.Get(context.Background(), pid)
	sid, ok := <-ch
	if sid != "f01228000" {
		t.Fatalf("unexpected response; got: %v / %t", sid, ok)
	}
}
