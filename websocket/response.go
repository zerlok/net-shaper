package websocket

import (
	"netshaper"
)

type RawResponse = Response[Message]

type Response[T any] interface {
	MessageSender[T]
	MessageListener[T]
	Closed() <-chan struct{}
	Err() error
	netshaper.Closeable
}
