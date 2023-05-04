package timer

import (
	"context"
	"math/rand"
	"time"
)

type Ticker struct {
	Period time.Duration
	Jitter time.Duration
}

func (t *Ticker) New() Timer {
	p := t.Period
	if t.Jitter > 0 {
		p += time.Duration(rand.Int63n((int64)(t.Jitter)))
	} else if t.Jitter < 0 {
		p -= time.Duration(rand.Int63n((int64)(-t.Jitter)))
	}

	if p == 0 {
		return newNullTimer()
	}

	return (*timer)(time.NewTimer(p))
}

func (t *Ticker) Do(fn func(Timer)) {
	tmr := t.New()
	defer tmr.Stop()

	fn(tmr)
}

func (t *Ticker) DoOnTick(ctx context.Context, fn func(time.Time)) {
	tmr := t.New()
	defer tmr.Stop()

	select {
	case <-ctx.Done():
		return
	case ts, ok := <-tmr.Ticks():
		if !ok {
			return
		}

		fn(ts)
	}
}

func (t *Ticker) DoOnEveryTick(ctx context.Context, fn func(time.Time)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			t.DoOnTick(ctx, fn)
		}
	}
}
