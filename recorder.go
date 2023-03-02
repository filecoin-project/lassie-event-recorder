package recorder

import (
	"context"
	"database/sql"
	"net"
	"net/http"

	"github.com/ipfs/go-log/v2"
)

var logger = log.Logger("lassie/event_recorder")

type EventRecorder struct {
	*options
	s  *http.Server
	db *sql.DB
}

func NewRecorder(o ...Option) (*EventRecorder, error) {
	opts, err := newOptions(o...)
	if err != nil {
		return nil, err
	}
	var er EventRecorder
	er.options = opts
	er.s = &http.Server{
		Addr:              er.options.httpServerListenAddr,
		Handler:           er.httpServerMux(),
		ReadTimeout:       er.options.httpServerReadTimeout,
		ReadHeaderTimeout: er.options.httpServerReadHeaderTimeout,
		WriteTimeout:      er.options.httpServerWriteTimeout,
		IdleTimeout:       er.options.httpServerWriteTimeout,
		MaxHeaderBytes:    er.options.httpServerMaxHeaderBytes,
	}
	return &er, err
}

func (er *EventRecorder) Start(_ context.Context) error {
	var err error
	if er.db, err = sql.Open("postgres", er.options.dbConnString); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", er.s.Addr)
	if err != nil {
		return err
	}
	go func() { _ = er.s.Serve(ln) }()
	logger.Infow("Server started", "addr", ln.Addr())
	return nil
}

func (er *EventRecorder) httpServerMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/retrieval-events", er.handleRetrievalEvents)
	mux.HandleFunc("/ready", er.handleReady)
	return mux
}

func (er *EventRecorder) handleReady(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// TODO: maybe ping DB here?
	default:
		http.Error(w, "", http.StatusNotFound)
	}
}
func (er *EventRecorder) handleRetrievalEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "", http.StatusNotFound)
	}
	http.Error(w, "to be implemented", http.StatusNotImplemented)
}

func (er *EventRecorder) Shutdown(ctx context.Context) error {
	return er.s.Shutdown(ctx)
}
