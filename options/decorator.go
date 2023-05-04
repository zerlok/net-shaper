package options

import (
	"context"
	"net/http"
	"netshaper"
	"netshaper/conf"
)

func WithDecorators[T1 netshaper.Request, T2 any](decorators ...Decorator[T1, T2]) conf.Option[netshaper.Config[T1, T2]] {
	cleanDecorators := make([]Decorator[T1, T2], 0, len(decorators))
	for _, dec := range decorators {
		if dec != nil {
			cleanDecorators = append(cleanDecorators, dec)
		}
	}

	if len(cleanDecorators) == 0 {
		return nil
	}

	return conf.OptionFunc[netshaper.Config[T1, T2]](func(config netshaper.Config[T1, T2]) netshaper.Config[T1, T2] {
		return &DecoratorConfig[T1, T2]{Inner: config, Decorators: cleanDecorators}
	})
}

type Decorator[T1 netshaper.Request, T2 any] interface {
	Enter(req T1) T1
	Exit(req T1, res T2, err error) (T2, error)
}

var _ netshaper.Config[*http.Request, string] = (*DecoratorConfig[*http.Request, string])(nil)

type DecoratorConfig[T1 netshaper.Request, T2 any] struct {
	Inner      netshaper.Config[T1, T2]
	Decorators []Decorator[T1, T2]
}

func (c *DecoratorConfig[T1, T2]) Create(ctx context.Context) (netshaper.Client[T1, T2], error) {
	inner, err := c.Inner.Create(ctx)
	if err != nil {
		return nil, err
	}

	if len(c.Decorators) == 0 {
		return inner, nil
	}

	return &decoratorClient[T1, T2]{inner, c.Decorators}, nil
}

var _ netshaper.Client[*http.Request, string] = (*decoratorClient[*http.Request, string])(nil)

type decoratorClient[T1 netshaper.Request, T2 any] struct {
	inner      netshaper.Client[T1, T2]
	decorators []Decorator[T1, T2]
}

func (c *decoratorClient[T1, T2]) Request(req T1) (res T2, err error) {
	for _, d := range c.decorators {
		req = d.Enter(req)
	}

	res, err = c.inner.Request(req)

	for i := len(c.decorators) - 1; i >= 0; i-- {
		res, err = c.decorators[i].Exit(req, res, err)
	}

	return
}

func (c *decoratorClient[T1, T2]) Close(ctx context.Context) {
	c.inner.Close(ctx)
}
