package eventrecorder

import (
	"errors"
	"fmt"

	"github.com/filecoin-project/lassie-event-recorder/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

type (
	config struct {
		dbDSN string
		// pgxPoolConfig is instantiated by parsing config.dbDSN.
		pgxPoolConfig *pgxpool.Config

		metrics *metrics.Metrics
	}
	option func(*config) error
)

func newConfig(opts []option) (*config, error) {
	cfg := &config{}
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

func WithDatabaseDSN(url string) option {
	return func(cfg *config) error {
		cfg.dbDSN = url
		return nil
	}
}

func WithMetrics(metrics *metrics.Metrics) option {
	return func(cfg *config) error {
		cfg.metrics = metrics
		return nil
	}
}
