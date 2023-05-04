package json

import (
	"context"
	"netshaper"
	"netshaper/websocket"
	"sync"
)

func RequestWebsocket[T any](client websocket.Client, req *websocket.Request) (res *WebsocketResponse[T], err error) {
	rawRes, err := client.Request(req)
	if err != nil {
		return
	}

	messages := make(chan *WebsocketMessage[T], req.BufferSize)
	res = &WebsocketResponse[T]{
		raw:      rawRes,
		messages: messages,
	}

	defer res.wg.Add(1)
	go res.run(messages)

	return res, err
}

var _ websocket.MessageSender[int] = (*WebsocketResponse[int])(nil)
var _ websocket.MessageListener[*WebsocketMessage[int]] = (*WebsocketResponse[int])(nil)
var _ netshaper.Closeable = (*WebsocketResponse[int])(nil)

type WebsocketResponse[T any] struct {
	raw      websocket.RawResponse
	cancel   context.CancelFunc
	messages <-chan *WebsocketMessage[T]
	wg       sync.WaitGroup
}

func (r *WebsocketResponse[T]) Send(message T) error {
	bytes, err := EncodeBody(&message)
	if err != nil {
		return err
	}

	return r.raw.Send(websocket.ByteMessage(bytes))
}

func (r *WebsocketResponse[T]) Listen() <-chan *WebsocketMessage[T] {
	return r.messages
}

func (r *WebsocketResponse[T]) Closed() <-chan struct{} {
	return r.raw.Closed()
}

func (r *WebsocketResponse[T]) Err() error {
	return r.raw.Err()
}

func (r *WebsocketResponse[T]) Close(ctx context.Context) {
	defer r.wg.Wait()
	r.raw.Close(ctx)
}

func (r *WebsocketResponse[T]) run(messages chan<- *WebsocketMessage[T]) {
	defer r.wg.Done()
	defer close(messages)

	for rawMsg := range r.raw.Listen() {
		msg := &WebsocketMessage[T]{Raw: rawMsg}

		if rawMsg.Err() == nil {
			msg.Value, msg.Error = Parse[T](rawMsg.Buff())
		}

		messages <- msg
	}
}

var _ websocket.Message = (*WebsocketMessage[int])(nil)

type WebsocketMessage[T any] struct {
	Raw   websocket.Message
	Value *T
	Error error
}

func (m *WebsocketMessage[T]) Buff() []byte {
	return m.Raw.Buff()
}

func (m *WebsocketMessage[T]) Err() error {
	if m.Error == nil {
		return m.Raw.Err()
	}

	return m.Error
}
