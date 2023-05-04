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
	responsesForIncoming := make(chan RawResponse, 1)
	responsesForOutgoing := make(chan RawResponse, 1)
	incomingMessages := make(chan Message, req.BufferSize)
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
	defer res.workersWg.Add(3)
	go res.produceUnderlying(req, &c.autoRefresh, responsesForOutgoing, responsesForIncoming)
	go res.forwardOutgoingMessages(responsesForOutgoing, outgoingMessages)
	go res.forwardIncomingMessages(responsesForIncoming, incomingMessages)

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
	defer close(r.outgoingMessages)
	r.cancel()
}

func (r *robustResponse) produceUnderlying(req *Request, autoRefresh *timer.Ticker, responsesForOutgoing, responsesForIncoming chan<- RawResponse) {
	defer r.workersWg.Done()
	defer close(responsesForOutgoing)
	defer close(responsesForIncoming)

	for ok := true; ok; {
		res, err := r.createUnderlying(req)
		if err != nil {
			return
		}

		responsesForOutgoing <- res
		responsesForIncoming <- res

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

func (r *robustResponse) forwardOutgoingMessages(responses <-chan RawResponse, outgoingMessages <-chan Message) {
	defer r.workersWg.Done()

	pool := make(chan RawResponse, 1)

	go func() {
		defer close(pool)
		for {
			select {
			case <-r.clientCtx.Done():
				return
			case <-r.ctx.Done():
				return
			case res := <-responses:
				select {
				case _, ok := <-pool:
					if !ok {
						return
					}
				default:
					pool <- res
				}
			}
		}
	}()

	for msg := range outgoingMessages {
		var err error
		for ok := false; !ok; ok = err == nil {
			select {
			case <-r.clientCtx.Done():
				return
			case <-r.ctx.Done():
				return
			case res := <-pool:
				pool <- res
				err = res.Send(msg)
				if err != nil {
					select {
					case <-res.Closed():
						continue
					default:
						res.Close(r.ctx)
					}
				}
			}
		}
	}
}

func (r *robustResponse) forwardIncomingMessages(responses <-chan RawResponse, incomingMessages chan<- Message) {
	defer r.workersWg.Done()
	defer close(incomingMessages)

	for res := range responses {
		var msg Message

		for ok := true; ok; {
			select {
			case <-r.clientCtx.Done():
				return
			case <-r.ctx.Done():
				return
			case msg, ok = <-res.Listen():
				if ok {
					incomingMessages <- msg
				}
			}
		}
	}
}
