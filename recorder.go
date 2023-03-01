package recorder

import (
	"context"
	"net"
	"net/http"

	"github.com/ipfs/go-log/v2"
)

var logger = log.Logger("lassie/event_recorder")

type EventRecorder struct {
	*options
	s *http.Server
}

func NewRecorder(o ...Option) (*EventRecorder, error) {
	opts, err := newOptions(o...)
	if err != nil {
		return nil, err
	}
	var r EventRecorder
	r.options = opts
	r.s = &http.Server{
		Addr:              r.options.httpServerListenAddr,
		Handler:           r.httpServerMux(),
		ReadTimeout:       r.options.httpServerReadTimeout,
		ReadHeaderTimeout: r.options.httpServerReadHeaderTimeout,
		WriteTimeout:      r.options.httpServerWriteTimeout,
		IdleTimeout:       r.options.httpServerWriteTimeout,
		MaxHeaderBytes:    r.options.httpServerMaxHeaderBytes,
	}
	return &r, err
}

func (r *EventRecorder) Start(_ context.Context) error {
	ln, err := net.Listen("tcp", r.s.Addr)
	if err != nil {
		return err
	}
	go func() { _ = r.s.Serve(ln) }()
	logger.Infow("Server started", "addr", ln.Addr())
	return nil
}

func (r *EventRecorder) httpServerMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/retrieval-events", r.handleRetrievalEvents)
	return mux
}

func (er *EventRecorder) handleRetrievalEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "", http.StatusNotFound)
	}
	http.Error(w, "to be implemented", http.StatusNotImplemented)
}

func (r *EventRecorder) Shutdown(ctx context.Context) error {
	return r.s.Shutdown(ctx)
}
