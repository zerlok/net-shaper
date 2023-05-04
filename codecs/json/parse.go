package json

import (
	"encoding/json"
	"fmt"
)

func Parse[T any](buff []byte) (value *T, err error) {
	value = new(T)
	if err = json.Unmarshal(buff, value); err != nil {
		return nil, &ParseError[T]{value, err}
	}

	return
}

type ParseError[T any] struct {
	Value *T
	Err   error
}

func (e *ParseError[T]) Unwrap() error {
	return e.Err
}

func (e *ParseError[T]) Error() string {
	return fmt.Sprintf("failed to parse %T response: %s", e.Value, e.Err.Error())
}
