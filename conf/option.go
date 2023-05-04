package conf

type Option[T any] interface {
	Apply(T) T
}

type OptionFunc[T any] func(T) T

func (fn OptionFunc[T]) Apply(config T) T {
	return fn(config)
}

func ApplyOptions[T any](opts []Option[T]) (result T) {
	return ApplyOptionsInit(opts, result)
}

func ApplyOptionsInit[T any](opts []Option[T], initial T) (result T) {
	result = initial

	for _, opt := range opts {
		if opt != nil {
			result = opt.Apply(result)
		}
	}

	return
}
