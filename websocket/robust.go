package websocket

import (
	"context"
	"fmt"
	"netshaper/conf"
	"netshaper/timer"
	"sync"
	"time"
)

func WithRobust(opts ...conf.Option[RobustConfig]) conf.Option[Config] {
	return conf.OptionFunc[Config](func(config Config) Config {
		cfg := conf.ApplyOptionsInit(opts, RobustConfig{Inner: config})
		return &cfg
	})
}

func WithRobustAutoRefreshPeriod(period time.Duration) conf.Option[RobustConfig] {
	return conf.OptionFunc[RobustConfig](func(config RobustConfig) RobustConfig {
		config.AutoRefresh.Period = period
		return config
	})
}

func WithRobustAutoRefreshJitter(jitter time.Duration) conf.Option[RobustConfig] {
	return conf.OptionFunc[RobustConfig](func(config RobustConfig) RobustConfig {
		config.AutoRefresh.Jitter = jitter
		return config
	})
}

var _ Config = (*RobustConfig)(nil)

type RobustConfig struct {
	Inner       Config
	AutoRefresh timer.Ticker
}

func (c *RobustConfig) Create(ctx context.Context) (Client, error) {
	ctx, cancel := context.WithCancel(ctx)

	inner, err := c.Inner.Create(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	return &robust{
		inner:       inner,
		ctx:         ctx,
		cancel:      cancel,
		autoRefresh: c.AutoRefresh,
	}, nil
}

var _ Client = (*robust)(nil)

type robust struct {
	inner       Client
	ctx         context.Context
	cancel      context.CancelFunc
	autoRefresh timer.Ticker
	responsesWg sync.WaitGroup
}

func (c *robust) Request(req *Request) (RawResponse, error) {
	resCtx, resCancel := context.WithCancel(req.Context())
	responses := make(chan RawResponse, 1)
	incomingMessages := make(chan Message, req.BufferSize)
	// TODO: close channel
	outgoingMessages := make(chan Message, req.BufferSize)

	res := &robustResponse{
		clientCtx:        c.ctx,
		ctx:              resCtx,
		cancel:           resCancel,
		responseWg:       &c.responsesWg,
		inner:            c.inner,
		incomingMessages: incomingMessages,
		outgoingMessages: outgoingMessages,
	}

	defer c.responsesWg.Add(1)
	defer res.workersWg.Add(2)
	go res.produceUnderlying(req, &c.autoRefresh, responses)
	go res.forwardMessages(responses, incomingMessages, outgoingMessages)

	return res, nil
}

func (c *robust) Close(ctx context.Context) {
	defer c.responsesWg.Wait()
	defer c.inner.Close(ctx)
	c.cancel()
}

var _ RawResponse = (*robustResponse)(nil)

type robustResponse struct {
	clientCtx        context.Context
	ctx              context.Context
	cancel           context.CancelFunc
	responseWg       *sync.WaitGroup
	inner            Client
	incomingMessages <-chan Message
	outgoingMessages chan<- Message
	workersWg        sync.WaitGroup
}

func (r *robustResponse) Send(message Message) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("closed")
		}
	}()

	r.outgoingMessages <- message

	return
}

func (r *robustResponse) Listen() <-chan Message {
	return r.incomingMessages
}

func (r *robustResponse) Closed() <-chan struct{} {
	return r.ctx.Done()
}

func (r *robustResponse) Err() error {
	return nil
}

func (r *robustResponse) Close(_ context.Context) {
	defer r.responseWg.Done()
	defer r.workersWg.Wait()
	r.cancel()
}

func (r *robustResponse) produceUnderlying(req *Request, autoRefresh *timer.Ticker, responses chan<- RawResponse) {
	defer r.workersWg.Done()
	defer close(responses)

	for ok := true; ok; {
		res, err := r.createUnderlying(req)
		if err != nil {
			return
		}

		responses <- res

		ok = r.handleUnderlying(res, autoRefresh)
	}
}

func (r *robustResponse) createUnderlying(req *Request) (res RawResponse, err error) {
	for res == nil || err != nil {
		select {
		case <-r.clientCtx.Done():
			err = r.clientCtx.Err()
			return
		case <-r.ctx.Done():
			err = r.ctx.Err()
			return
		default:
			res, err = r.inner.Request(req)
		}
	}

	return
}

func (r *robustResponse) handleUnderlying(res RawResponse, autoRefresh *timer.Ticker) (ok bool) {
	autoRefresh.Do(func(tmr timer.Timer) {
		select {
		case <-r.clientCtx.Done():
			return
		case <-r.ctx.Done():
			return
		case <-res.Closed():
			ok = true
		case <-tmr.Ticks():
			res.Close(r.ctx)
			ok = true
		}
	})

	return
}

func (r *robustResponse) forwardMessages(responses <-chan RawResponse, incomingMessages chan<- Message, outgoingMessages <-chan Message) {
	defer r.workersWg.Done()
	defer close(incomingMessages)

	for res := range responses {
		var in Message

		for ok := true; ok; {
			select {
			case <-r.clientCtx.Done():
				return
			case <-r.ctx.Done():
				return
			case out := <-outgoingMessages:
				// TODO: retry sending until it succeeds
				err := res.Send(out)
				if err != nil {
					res.Close(r.ctx)
				}
			case in, ok = <-res.Listen():
				if ok {
					incomingMessages <- in
				}
			}
		}
	}
}
