package options

import (
	"context"
	"net/http"
	"netshaper"
	"netshaper/conf"
	"sync"
)

func WithPool[T1 netshaper.Request, T2 any](opts ...conf.Option[PoolConfig[T1, T2]]) conf.Option[netshaper.Config[T1, T2]] {
	return conf.OptionFunc[netshaper.Config[T1, T2]](func(config netshaper.Config[T1, T2]) netshaper.Config[T1, T2] {
		cfg := conf.ApplyOptionsInit(opts, PoolConfig[T1, T2]{Inner: config})
		return &cfg
	})
}

func WithPoolSize[T1 netshaper.Request, T2 any](size uint) conf.Option[PoolConfig[T1, T2]] {
	return conf.OptionFunc[PoolConfig[T1, T2]](func(config PoolConfig[T1, T2]) PoolConfig[T1, T2] {
		config.Size = size
		return config
	})
}

func WithPoolPendingSize[T1 netshaper.Request, T2 any](size uint) conf.Option[PoolConfig[T1, T2]] {
	return conf.OptionFunc[PoolConfig[T1, T2]](func(config PoolConfig[T1, T2]) PoolConfig[T1, T2] {
		config.PendingSize = size
		return config
	})
}

var _ netshaper.Config[*http.Request, string] = (*PoolConfig[*http.Request, string])(nil)

type PoolConfig[T1 netshaper.Request, T2 any] struct {
	Inner       netshaper.Config[T1, T2]
	Size        uint
	PendingSize uint
}

func (c *PoolConfig[T1, T2]) Create(ctx context.Context) (netshaper.Client[T1, T2], error) {
	if c.Size <= 1 {
		return c.Inner.Create(ctx)
	}

	pendingSize := c.PendingSize
	if pendingSize == 0 {
		pendingSize = c.Size * 2
	} else if pendingSize < c.Size {
		pendingSize = c.Size
	}

	poolCtx, poolCancel := context.WithCancel(ctx)
	pending := make(chan *requestJob[T1, T2], pendingSize)

	p := &pool[T1, T2]{
		ctx:     poolCtx,
		cancel:  poolCancel,
		pending: pending,
	}

	for i := uint(0); i < c.Size; i++ {
		inner, err := c.Inner.Create(ctx)
		if err != nil {
			p.Close(ctx)
			return nil, err
		}

		p.wg.Add(1)
		go p.runWorker(inner, pending)
	}

	return p, nil
}

var _ netshaper.Client[*http.Request, string] = (*pool[*http.Request, string])(nil)

type pool[T1 netshaper.Request, T2 any] struct {
	ctx     context.Context
	cancel  context.CancelFunc
	pending chan<- *requestJob[T1, T2]
	wg      sync.WaitGroup
}

func (c *pool[T1, T2]) Request(req T1) (res T2, err error) {
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
	case result := <-results:
		res = result.response
		err = result.err
	}

	return
}

func (c *pool[T1, T2]) Close(_ context.Context) {
	c.close()
}

func (c *pool[T1, T2]) close() {
	defer c.wg.Wait()
	defer close(c.pending)
	c.cancel()
}

func (c *pool[T1, T2]) runWorker(cl netshaper.Client[T1, T2], pending <-chan *requestJob[T1, T2]) {
	defer c.wg.Done()
	defer c.close()

	for {
		select {
		case <-c.ctx.Done():
			return
		case r := <-pending:
			c.handleJob(cl, r)
		}
	}
}

func (c *pool[T1, T2]) handleJob(cl netshaper.Client[T1, T2], job *requestJob[T1, T2]) {
	req := job.request
	res, err := cl.Request(req)

	select {
	case <-c.ctx.Done():
		return
	case <-req.Context().Done():
		return
	default:
		job.results <- &responseResult[T2]{res, err}
	}

	return
}
