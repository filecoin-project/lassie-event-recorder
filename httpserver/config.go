package httpserver

import (
	"time"
)

type (
	config struct {
		httpServerListenAddr        string
		httpServerReadTimeout       time.Duration
		httpServerReadHeaderTimeout time.Duration
		httpServerWriteTimeout      time.Duration
		httpServerIdleTimeout       time.Duration
		httpServerMaxHeaderBytes    int
	}
	option func(*config) error
)

func newConfig(opts []option) (*config, error) {
	cfg := &config{
		httpServerListenAddr:        "0.0.0.0:8080",
		httpServerReadTimeout:       5 * time.Second,
		httpServerReadHeaderTimeout: 5 * time.Second,
		httpServerWriteTimeout:      5 * time.Second,
		httpServerIdleTimeout:       10 * time.Second,
		httpServerMaxHeaderBytes:    2048,
	}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func WithHttpServerListenAddr(addr string) option {
	return func(cfg *config) error {
		cfg.httpServerListenAddr = addr
		return nil
	}
}
