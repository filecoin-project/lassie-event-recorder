// spmap is a map of [peerid->spid]
// when the mapping is not known locally, hey-fil is used to find it.
package spmap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var logger = log.Logger("lassie/spmap")

type Option func(spConfig)

func NewSPMap(opts ...Option) *SPMap {
	cf := spConfig{
		heyFilEndpoint: "https://heyfil.prod.cid.contact",
		client:         http.DefaultClient,
	}
	for _, o := range opts {
		o(cf)
	}
	arc, err := lru.NewARC(10_000)
	if err != nil {
		logger.Errorf("failed to allocate cache: %w", err)
		return nil
	}
	sm := SPMap{
		cfg:   cf,
		cache: arc,
		c:     make(chan work, 10),
	}
	go sm.run()
	return &sm
}

type spConfig struct {
	heyFilEndpoint string
	client         *http.Client
}

func WithHeyFil(endpoint string) Option {
	return func(sc spConfig) {
		sc.heyFilEndpoint = endpoint
	}
}

func WithClient(c *http.Client) Option {
	return func(sc spConfig) {
		sc.client = c
	}
}

type SPMap struct {
	cfg spConfig

	cache *lru.ARCCache
	lk    sync.RWMutex

	c chan work
}

type work struct {
	ctx      context.Context
	query    peer.ID
	response chan string
}

func (s *SPMap) get(id string) ([]string, bool) {
	s.lk.RLock()
	defer s.lk.RUnlock()
	v, ok := s.cache.Get(id)
	if ok {
		vs := v.([]string)
		return vs, true
	}
	return nil, ok
}

func (s *SPMap) set(id string, val []string) {
	s.lk.Lock()
	defer s.lk.Unlock()
	// overwrite if there's a previous version.
	s.cache.Add(id, val)
}

func (s *SPMap) query(ctx context.Context, id peer.ID) []string {
	url := fmt.Sprintf("%s/sp?peerid=%s", s.cfg.heyFilEndpoint, id.String())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := s.cfg.client.Do(req)
	if err != nil {
		logger.Warnf("failed to contact heyfil: %w", err)
		return nil
	}
	defer resp.Body.Close()
	sps := []string{}

	if err = json.NewDecoder(resp.Body).Decode(&sps); err != nil {
		logger.Warnf("failed to decode response from heyfil: %w", err)
		return nil
	}
	return sps
}

func (s *SPMap) run() {
	for t := range s.c {
		resp := s.query(t.ctx, t.query)

		s.set(t.query.String(), resp)
		if len(resp) > 0 {
			t.response <- resp[0]
		}
		close(t.response)
		continue
	}
}

func (s *SPMap) Close() {
	close(s.c)
}

func (s *SPMap) Get(ctx context.Context, id peer.ID) chan string {
	resp := make(chan string, 1)
	c, ok := s.get(id.String())
	if ok {
		if len(c) > 0 {
			resp <- c[0]
		}

		close(resp)
		return resp
	}
	wk := work{
		ctx:      ctx,
		query:    id,
		response: resp,
	}
	select {
	case s.c <- wk:
		return resp
	default:
		close(resp)
		return resp
	}
}
