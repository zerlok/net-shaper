package websocket

type Handler[T any] interface {
	Handle(message T) bool
}

type HandlerFunc[T any] func(message T) bool

func (fn HandlerFunc[T]) Handle(message T) bool {
	return fn(message)
}

func Chain[T any](handlers ...Handler[T]) Handler[T] {
	switch len(handlers) {
	case 0:
		return HandlerFunc[T](func(_ T) bool {
			return true
		})
	case 1:
		return handlers[0]
	default:
		return HandlerFunc[T](func(message T) (ok bool) {
			for _, listener := range handlers {
				if ok = listener.Handle(message); !ok {
					return
				}
			}

			return true
		})
	}
}
