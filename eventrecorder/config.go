package eventrecorder

import (
	"errors"
	"fmt"

	"github.com/filecoin-project/lassie-event-recorder/spmap"
	"github.com/jackc/pgx/v5/pgxpool"
)

type (
	config struct {
		dbDSN string
		// pgxPoolConfig is instantiated by parsing config.dbDSN.
		pgxPoolConfig *pgxpool.Config

		mongoEndpoint   string
		mongoDB         string
		mongoCollection string
		mongoPercentile float32

		mapcfg []spmap.Option

		metrics Metrics
	}
	Option func(*config) error
)

func newConfig(opts []Option) (*config, error) {
	cfg := &config{}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	if cfg.dbDSN != "" {
		var err error
		if cfg.pgxPoolConfig, err = pgxpool.ParseConfig(cfg.dbDSN); err != nil {
			return nil, fmt.Errorf("unable to parse db URL: %w", err)
		}
	}
	if cfg.pgxPoolConfig == nil && cfg.metrics == nil && cfg.mongoEndpoint == "" {
		return nil, errors.New("must set up at least one of: postgres, mongo, metrics")
	}
	return cfg, nil
}

func WithDatabaseDSN(url string) Option {
	return func(cfg *config) error {
		cfg.dbDSN = url
		return nil
	}
}

func WithMongoSubmissions(endpoint, db, collection string, percentage float32) Option {
	return func(c *config) error {
		c.mongoEndpoint = endpoint
		c.mongoDB = db
		c.mongoCollection = collection
		c.mongoPercentile = percentage
		return nil
	}
}

func WithMetrics(metrics Metrics) Option {
	return func(cfg *config) error {
		cfg.metrics = metrics
		return nil
	}
}

func WithSPMapOptions(opts ...spmap.Option) Option {
	return func(cfg *config) error {
		cfg.mapcfg = opts
		return nil
	}
}
