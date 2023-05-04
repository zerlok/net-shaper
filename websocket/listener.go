package websocket

import (
	"context"
	"netshaper"
)

func ListenSliceRawMessages(ctx context.Context, client Client, request *Request, maxSize uint) ([]Message, error) {
	return ListenSlice[*Request, Message](ctx, client, request, maxSize)
}

func ListenSlice[T1 any, T2 any](ctx context.Context, client netshaper.Client[T1, Response[T2]], request T1, maxSize uint) ([]T2, error) {
	res, err := client.Request(request)
	if err != nil {
		return nil, err
	}
	defer res.Close(ctx)

	listener := NewSliceListener[T2](ctx, maxSize)
	for ok := true; ok; {
		select {
		case <-ctx.Done():
			ok = false
			break
		case msg, closed := <-res.Listen():
			ok = !closed || listener.Handle(msg)
		}
	}

	return listener.Messages, nil
}

type SliceListener[T any] struct {
	Messages []T
	handler  Handler[T]
}

func (l *SliceListener[T]) Handle(message T) bool {
	return l.handler.Handle(message)
}

func NewSliceListener[T any](ctx context.Context, maxSize uint) *SliceListener[T] {
	listener := &SliceListener[T]{}

	handlers := append([]Handler[T]{}, HandlerFunc[T](func(message T) bool {
		select {
		case <-ctx.Done():
			return false
		default:
			listener.Messages = append(listener.Messages, message)
			return true
		}
	}))
	if maxSize > 0 {
		handlers = append(handlers, HandlerFunc[T](func(message T) bool {
			return uint(len(listener.Messages)) < maxSize
		}))
	}

	listener.handler = Chain[T](handlers...)

	return listener
}
