package netshaper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"netshaper/conf"
)

func New[T1 any, T2 any](opts ...conf.Option[Config[T1, T2]]) Config[T1, T2] {
	return conf.ApplyOptions(opts)
}

func NewClient[T1 any, T2 any](ctx context.Context, opts ...conf.Option[Config[T1, T2]]) (Client[T1, T2], error) {
	config := New(opts...)
	if config == nil {
		return nil, fmt.Errorf("nil config")
	}

	return config.Create(ctx)
}

type URL = url.URL

func ParseURL(u string) (*URL, error) {
	return url.Parse(u)
}

type Headers = http.Header

type Body = []byte

type Request interface {
	Context() context.Context
}

type Config[T1 any, T2 any] interface {
	Create(ctx context.Context) (Client[T1, T2], error)
}

type Client[T1 any, T2 any] interface {
	Request(request T1) (response T2, err error)
	Closeable
}

type Closeable interface {
	Close(ctx context.Context)
}
