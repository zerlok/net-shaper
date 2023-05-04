package factory

import (
	"context"
	netHttp "net/http"
	"netshaper"
	"netshaper/http"
	"netshaper/options"
	"time"
)

func DefaultHttp(ctx context.Context) (http.Client, error) {
	return NewNetHttp(ctx, NetHttpConfig{
		Timeout:              1 * time.Minute,
		PoolSize:             10,
		MaxRps:               100.0,
		PreProcessors:        nil,
		PostProcessors:       nil,
		RetryOnStatusCodes:   []int{429},
		InitialRetryDelay:    1 * time.Second,
		RetryDelayMultiplier: 2 * time.Second,
		MaxRetryDelay:        1 * time.Minute,
		MaxRetriesLimit:      10,
	})
}

func NewNetHttp(ctx context.Context, cfg NetHttpConfig) (http.Client, error) {
	var decorators []options.Decorator[*http.Request, *http.Response]
	for _, p := range cfg.PreProcessors {
		decorators = append(decorators, p)
	}
	decorators = append(decorators, options.MakeResponseStatusCodesAsErrorsDecorator[*http.Request](cfg.RetryOnStatusCodes...))
	for _, p := range cfg.PostProcessors {
		decorators = append(decorators, p)
	}

	return netshaper.NewClient(ctx,
		http.NewNet(
			http.WithNetTransport(cfg.Transport),
			http.WithNetCheckRedirect(cfg.CheckRedirect),
			http.WithNetJar(cfg.Jar),
			http.WithNetTimeout(cfg.Timeout),
		),
		options.WithPool(
			options.WithPoolSize[*http.Request, *http.Response](cfg.PoolSize),
		),
		options.WithRequestPerSecondLimiter[*http.Request, *http.Response](cfg.MaxRps),
		options.WithDecorators[*http.Request, *http.Response](decorators...),
		options.WithCircuitBreaker(
			options.WithExponentialDelayRetries[*http.Request, *http.Response](cfg.InitialRetryDelay, cfg.RetryDelayMultiplier, cfg.MaxRetryDelay),
			options.WithMaxRetriesLimit[*http.Request, *http.Response](cfg.MaxRetriesLimit),
		),
	)
}

type NetHttpConfig struct {
	Transport            netHttp.RoundTripper
	CheckRedirect        func(req *http.Request, via []*http.Request) error
	Jar                  netHttp.CookieJar
	Timeout              time.Duration
	PoolSize             uint
	MaxRps               float64
	PreProcessors        []options.PreProcessingFunc[*http.Request, *http.Response]
	PostProcessors       []options.PreProcessingFunc[*http.Request, *http.Response]
	RetryOnStatusCodes   []int
	InitialRetryDelay    time.Duration
	RetryDelayMultiplier time.Duration
	MaxRetryDelay        time.Duration
	MaxRetriesLimit      uint
}
