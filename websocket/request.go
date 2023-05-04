package websocket

import (
	"context"
	"netshaper"
	"time"
)

type Request struct {
	Ctx            context.Context
	URL            netshaper.URL
	Headers        netshaper.Headers
	Protocol       string
	Origin         string
	ReceiveTimeout time.Duration
	BufferSize     uint
}

func (r *Request) Context() context.Context {
	if r.Ctx == nil {
		return context.Background()
	}

	return r.Ctx
}
