package websocket

type MessageSender[T any] interface {
	Send(message T) error
}

type MessageListener[T any] interface {
	Listen() <-chan T
}

type Message interface {
	Buff() []byte
	Err() error
}

var _ Message = (*ByteMessage)(nil)

type ByteMessage []byte

func (m ByteMessage) Buff() []byte {
	return m
}

func (m ByteMessage) Err() error {
	return nil
}

var _ Message = (*TextMessage)(nil)

type TextMessage string

func (m TextMessage) Buff() []byte {
	return []byte(m)
}

func (m TextMessage) Err() error {
	return nil
}

var _ Message = (*ErrorMessage)(nil)

type ErrorMessage struct {
	Error error
}

func (m *ErrorMessage) Buff() []byte {
	return nil
}

func (m *ErrorMessage) Err() error {
	return m.Error
}
