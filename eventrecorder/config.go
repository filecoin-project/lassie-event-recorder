package eventrecorder

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type (
	config struct {
		httpServerListenAddr        string
		httpServerReadTimeout       time.Duration
		httpServerReadHeaderTimeout time.Duration
		httpServerWriteTimeout      time.Duration
		httpServerIdleTimeout       time.Duration
		httpServerMaxHeaderBytes    int

		dbDSN string
		// pgxPoolConfig is instantiated by parsing config.dbDSN.
		pgxPoolConfig *pgxpool.Config
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

	if cfg.dbDSN == "" {
		return nil, errors.New("db URL must be specified")
	}
	var err error
	if cfg.pgxPoolConfig, err = pgxpool.ParseConfig(cfg.dbDSN); err != nil {
		return nil, fmt.Errorf("unable to parse db URL: %w", err)
	}
	return cfg, nil
}

func WithHttpServerListenAddr(addr string) option {
	return func(cfg *config) error {
		cfg.httpServerListenAddr = addr
		return nil
	}
}

func WithDatabaseDSN(url string) option {
	return func(cfg *config) error {
		cfg.dbDSN = url
		return nil
	}
}
