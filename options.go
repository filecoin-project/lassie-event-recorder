package recorder

import (
	"fmt"
	"time"
)

type (
	Option  func(*options) error
	options struct {
		httpServerListenAddr        string
		httpServerReadTimeout       time.Duration
		httpServerReadHeaderTimeout time.Duration
		httpServerWriteTimeout      time.Duration
		httpServerIdleTimeout       time.Duration
		httpServerMaxHeaderBytes    int

		dbName       string
		dbHost       string
		dbPort       int
		dbUser       string
		dbPassword   string
		dbParameters string
		dbConnString string
	}
)

func newOptions(o ...Option) (*options, error) {
	opts := options{
		httpServerListenAddr:        "0.0.0.0:8080",
		httpServerReadTimeout:       5 * time.Second,
		httpServerReadHeaderTimeout: 5 * time.Second,
		httpServerWriteTimeout:      5 * time.Second,
		httpServerIdleTimeout:       10 * time.Second,
		httpServerMaxHeaderBytes:    2048,
	}
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	opts.dbConnString = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s %s", opts.dbHost, opts.dbPort, opts.dbUser, opts.dbPassword, opts.dbName, opts.dbParameters)
	return &opts, nil
}

func WithHttpServerListenAddr(a string) Option {
	return func(o *options) error {
		o.httpServerListenAddr = a
		return nil
	}
}
