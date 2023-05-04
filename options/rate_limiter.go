package options

import (
	"context"
	"net/http"
	"netshaper"
	"netshaper/conf"
	"sync"
	"time"
)

func WithRateLimiter[T1 netshaper.Request, T2 any](limiter RateLimiter[T1, T2]) conf.Option[netshaper.Config[T1, T2]] {
	if limiter == nil {
		return nil
	}

	return conf.OptionFunc[netshaper.Config[T1, T2]](func(config netshaper.Config[T1, T2]) netshaper.Config[T1, T2] {
		return &RateLimitConfig[T1, T2]{config, limiter}
	})
}

func WithRequestPerSecondLimiter[T1 netshaper.Request, T2 any](rps float64) conf.Option[netshaper.Config[T1, T2]] {
	if rps == 0 {
		return nil
	}

	return WithRateLimiter[T1, T2](&timeBucketRateLimiter[T1, T2]{
		maxRps: rps,
		now:    time.Now,
	})
}

func WithRequestPerDurationLimiter[T1 netshaper.Request, T2 any](amount uint, interval time.Duration) conf.Option[netshaper.Config[T1, T2]] {
	return WithRequestPerSecondLimiter[T1, T2](float64(amount) / float64(interval/time.Second))
}

type RateLimiter[T1 any, T2 any] interface {
	Enter(req T1)
	Exit(req T1, res T2, err error)
}

type timeBucketRateLimiter[T1 netshaper.Request, T2 any] struct {
	maxRps     float64
	now        func() time.Time
	lastUpdate time.Time
	lastCount  float64
}

func (l *timeBucketRateLimiter[T1, T2]) Enter(req T1) {
	count := l.lastCount + 1.0
	seconds := float64(l.now().Sub(l.lastUpdate) / time.Second)

	if count/seconds > l.maxRps {
		t := time.NewTimer(time.Duration(int(count/l.maxRps)) * time.Second)
		select {
		case <-req.Context().Done():
			if t.Stop() {
				<-t.C
			}
			return
		case <-t.C:
			break
		}

		count = 1.0
	}

	l.lastUpdate = l.now()
	l.lastCount = count
}

func (l *timeBucketRateLimiter[T1, T2]) Exit(_ T1, _ T2, _ error) {
}

var _ netshaper.Config[*http.Request, string] = (*RateLimitConfig[*http.Request, string])(nil)

type RateLimitConfig[T1 netshaper.Request, T2 any] struct {
	Inner   netshaper.Config[T1, T2]
	Limiter RateLimiter[T1, T2]
}

func (c *RateLimitConfig[T1, T2]) Create(ctx context.Context) (netshaper.Client[T1, T2], error) {
	ctx, cancel := context.WithCancel(ctx)

	inner, err := c.Inner.Create(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	pending := make(chan *requestJob[T1, T2], 1000)

	cl := &rateLimitClient[T1, T2]{
		ctx:     ctx,
		cancel:  cancel,
		limiter: c.Limiter,
		inner:   inner,
		pending: pending,
	}

	defer cl.wg.Add(1)
	go cl.runWorker(pending)

	return cl, nil
}

var _ netshaper.Client[*http.Request, string] = (*rateLimitClient[*http.Request, string])(nil)

type rateLimitClient[T1 netshaper.Request, T2 any] struct {
	ctx     context.Context
	cancel  context.CancelFunc
	limiter RateLimiter[T1, T2]
	inner   netshaper.Client[T1, T2]
	wg      sync.WaitGroup
	pending chan<- *requestJob[T1, T2]
}

func (c *rateLimitClient[T1, T2]) Request(req T1) (res T2, err error) {
	results := make(chan *responseResult[T2], 1)
	defer close(results)

	c.pending <- &requestJob[T1, T2]{
		request: req,
		results: results,
	}

	select {
	case <-c.ctx.Done():
		err = c.ctx.Err()
	case <-req.Context().Done():
		err = req.Context().Err()
	case resCtx := <-results:
		res = resCtx.response
		err = resCtx.err
	}

	return
}

func (c *rateLimitClient[T1, T2]) Close(ctx context.Context) {
	defer c.wg.Wait()
	defer close(c.pending)
	defer c.inner.Close(ctx)
	c.cancel()
}

func (c *rateLimitClient[T1, T2]) runWorker(pending <-chan *requestJob[T1, T2]) {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case req := <-pending:
			c.handleJob(req)
		}
	}
}

func (c *rateLimitClient[T1, T2]) handleJob(job *requestJob[T1, T2]) {
	var res T2
	var err error

	req := job.request

	select {
	case <-c.ctx.Done():
		err = c.ctx.Err()
	case <-req.Context().Done():
		err = req.Context().Err()
	default:
		c.limiter.Enter(req)
		defer func() { c.limiter.Exit(req, res, err) }()

		res, err = c.inner.Request(req)
	}

	select {
	case <-req.Context().Done():
		break
	default:
		job.results <- &responseResult[T2]{res, err}
	}

	return
}
