package options

import (
	"context"
	"fmt"
	"netshaper"
	"netshaper/conf"
	"time"
)

func WithCircuitBreaker[T1 any, T2 any](opts ...conf.Option[CircuitBreakerConfig[T1, T2]]) conf.Option[netshaper.Config[T1, T2]] {
	return conf.OptionFunc[netshaper.Config[T1, T2]](func(config netshaper.Config[T1, T2]) netshaper.Config[T1, T2] {
		cfg := conf.ApplyOptionsInit(opts, CircuitBreakerConfig[T1, T2]{Inner: config})
		return &cfg
	})
}

func WithMaxRetriesLimit[T1 any, T2 any](limit uint) conf.Option[CircuitBreakerConfig[T1, T2]] {
	if limit == 0 {
		return nil
	}

	return conf.OptionFunc[CircuitBreakerConfig[T1, T2]](func(config CircuitBreakerConfig[T1, T2]) CircuitBreakerConfig[T1, T2] {
		config.wrapFactory(func(inner CircuitBreaker[T1, T2]) CircuitBreaker[T1, T2] {
			return &attemptLimitCircuitBreaker[T1, T2]{inner: inner, maxRetries: limit}
		})

		return config
	})
}

func WithExponentialDelayRetries[T1 netshaper.Request, T2 any](initial time.Duration, mul time.Duration, maxDelay time.Duration) conf.Option[CircuitBreakerConfig[T1, T2]] {
	if mul == 0 {
		return nil
	}

	return conf.OptionFunc[CircuitBreakerConfig[T1, T2]](func(config CircuitBreakerConfig[T1, T2]) CircuitBreakerConfig[T1, T2] {
		config.wrapFactory(func(inner CircuitBreaker[T1, T2]) CircuitBreaker[T1, T2] {
			return &exponentialDelayCircuitBreaker[T1, T2]{inner: inner, maxDelay: maxDelay, delay: initial, mul: mul}
		})

		return config
	})
}

type CircuitBreaker[T1 any, T2 any] interface {
	Next(req T1, res T2, err error) (bool, T1, error)
}

type noopCircuitBreaker[T1 any, T2 any] struct{}

func (b *noopCircuitBreaker[T1, T2]) Next(req T1, _ T2, err error) (bool, T1, error) {
	if err == nil {
		// no error - stop circuit breaker iteration
		return err != nil, req, nil
	} else {
		// error occurred - continue circuit breaker iteration
		return true, req, err
	}
}

type attemptLimitCircuitBreaker[I any, O any] struct {
	inner      CircuitBreaker[I, O]
	maxRetries uint
	retries    uint
}

func (b *attemptLimitCircuitBreaker[I, O]) Next(req I, res O, err error) (bool, I, error) {
	if err != nil {
		retries := b.retries + 1
		if retries >= b.maxRetries {
			// attempt limit reached - stop circuit breaker iteration
			return false, req, fmt.Errorf("max retries limit reached %v, last error: %w", b.maxRetries, err)
		}

		b.retries = retries
	}

	return b.inner.Next(req, res, err)
}

type exponentialDelayCircuitBreaker[T1 netshaper.Request, T2 any] struct {
	inner    CircuitBreaker[T1, T2]
	maxDelay time.Duration
	delay    time.Duration
	mul      time.Duration
}

func (b *exponentialDelayCircuitBreaker[T1, T2]) Next(req T1, res T2, err error) (bool, T1, error) {
	if err != nil {
		t := time.NewTimer(b.delay)
		select {
		case <-req.Context().Done():
			if t.Stop() {
				<-t.C
			}
			return false, req, req.Context().Err()
		case <-t.C:
			break
		}

		b.delay *= b.mul
		if b.maxDelay > 0 && b.delay > b.maxDelay {
			b.delay = b.maxDelay
		}
	}

	return b.inner.Next(req, res, err)
}

var _ netshaper.Config[int, string] = (*CircuitBreakerConfig[int, string])(nil)

type CircuitBreakerConfig[T1 any, T2 any] struct {
	Inner          netshaper.Config[T1, T2]
	BreakerFactory func() CircuitBreaker[T1, T2]
}

func (c *CircuitBreakerConfig[T1, T2]) wrapFactory(wrapper func(inner CircuitBreaker[T1, T2]) CircuitBreaker[T1, T2]) {
	if inner := c.BreakerFactory; inner != nil {
		c.BreakerFactory = func() CircuitBreaker[T1, T2] {
			return wrapper(inner())
		}
	} else {
		c.BreakerFactory = func() CircuitBreaker[T1, T2] {
			return wrapper(&noopCircuitBreaker[T1, T2]{})
		}
	}

}

func (c *CircuitBreakerConfig[T1, T2]) Create(ctx context.Context) (netshaper.Client[T1, T2], error) {
	inner, err := c.Inner.Create(ctx)
	if err != nil {
		return nil, err
	}

	if c.BreakerFactory == nil {
		return inner, nil
	}

	return &circuitBreakerClient[T1, T2]{inner, c.BreakerFactory}, nil
}

var _ netshaper.Client[int, string] = (*circuitBreakerClient[int, string])(nil)

type circuitBreakerClient[T1 any, T2 any] struct {
	inner          netshaper.Client[T1, T2]
	breakerFactory func() CircuitBreaker[T1, T2]
}

func (c *circuitBreakerClient[T1, T2]) Request(req T1) (res T2, err error) {
	breaker := c.breakerFactory()

	for ok := true; ok; ok, req, err = breaker.Next(req, res, err) {
		res, err = c.inner.Request(req)
	}

	return
}

func (c *circuitBreakerClient[T1, T2]) Close(ctx context.Context) {
	c.inner.Close(ctx)
}
