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

		mongoEndpoint   string
		mongoDB         string
		mongoCollection string
		mongoPercentile float32

		metrics *metrics.Metrics
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

	if cfg.dbDSN == "" {
		return nil, errors.New("db URL must be specified")
	}
	var err error
	if cfg.pgxPoolConfig, err = pgxpool.ParseConfig(cfg.dbDSN); err != nil {
		return nil, fmt.Errorf("unable to parse db URL: %w", err)
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

func WithMetrics(metrics *metrics.Metrics) Option {
	return func(cfg *config) error {
		cfg.metrics = metrics
		return nil
	}
}
