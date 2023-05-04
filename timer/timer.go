package timer

import (
	"sync"
	"time"
)

type Timer interface {
	Ticks() <-chan time.Time
	Stop()
}

type timer time.Timer

func (t *timer) Ticks() <-chan time.Time {
	return t.C
}

func (t *timer) Stop() {
	(*time.Timer)(t).Stop()
}

func newNullTimer() Timer {
	return &nullTimer{
		ticks: make(chan time.Time),
	}
}

type nullTimer struct {
	ticks     chan time.Time
	closeOnce sync.Once
}

func (t *nullTimer) Ticks() <-chan time.Time {
	return t.ticks
}

func (t *nullTimer) Stop() {
	t.closeOnce.Do(func() {
		close(t.ticks)
	})
}
