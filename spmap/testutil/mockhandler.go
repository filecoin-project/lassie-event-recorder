package testutil

import (
	"fmt"
	"net/http"
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
