package recorder

import "time"

type (
	Option  func(*options) error
	options struct {
		httpServerListenAddr        string
		httpServerReadTimeout       time.Duration
		httpServerReadHeaderTimeout time.Duration
		httpServerWriteTimeout      time.Duration
		httpServerIdleTimeout       time.Duration
		httpServerMaxHeaderBytes    int
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
	return &opts, nil
}

func WithHttpServerListenAddr(a string) Option {
	return func(o *options) error {
		o.httpServerListenAddr = a
		return nil
	}
}
