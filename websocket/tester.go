package websocket

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Tester struct {
	RequestsAmount          uint
	RequestFactory          func(uint) *Request
	RequestBuffSize         uint
	ListenMessagesMaxAmount int
	ListenTimeout           time.Duration
	Filter                  func(websocketMessage Message) bool
	Logger                  *log.Logger
}

func (t *Tester) RunMessages(ctx context.Context, cl Client) (messages []Message, errs []error) {
	factory := t.RequestFactory
	if factory == nil {
		factory = func(_ uint) *Request {
			return &Request{
				Ctx:        ctx,
				BufferSize: DefaultBuffSize,
			}
		}
	}

	requests := []*Request{}
	for i := uint(0); i < t.RequestsAmount; i++ {
		requests = append(requests, factory(i))
	}

	return t.RequestMessages(cl, requests...)
}

func (t *Tester) RequestMessages(cl Client, requests ...*Request) (messages []Message, errs []error) {
	messages = []Message{}
	errs = []error{}

	for i, req := range requests {
		t.log(fmt.Sprintf("sending request#%03d ...", i))
		requestMessages, err := t.doRequestMessages(cl, req)

		if err != nil {
			errs = append(errs, err)
		} else {
			errs = append(errs, nil)
			messages = append(messages, requestMessages...)
		}
	}

	return
}

func (t *Tester) doRequestMessages(cl Client, req *Request) (messages []Message, err error) {
	resp, err := cl.Request(req)
	if err != nil {
		t.log(fmt.Sprintf("request failed; err = %#v", err.Error()))
		return
	}
	defer resp.Close(req.Context())

	messages, _ = t.ResponseMessages(req.Context(), resp)
	t.log(fmt.Sprintf("messages from request response received; len = %03d", len(messages)))

	return
}

func (t *Tester) ResponseMessages(ctx context.Context, resp RawResponse) (messages []Message, ok bool) {
	var msg Message

	if t.ListenTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.ListenTimeout)
		defer cancel()
	}

	messages = make([]Message, 0, t.ListenMessagesMaxAmount)
	for i := 0; t.ListenMessagesMaxAmount == 0 || i < t.ListenMessagesMaxAmount; i++ {
		select {
		case <-ctx.Done():
			t.log("response listen context done")
			return
		case msg, ok = <-resp.Listen():
			t.log(fmt.Sprintf("response listen message #%03d received; msg = %#v, ok = %v", i, msg, ok))
			if !ok {
				return
			}
			if t.Filter == nil || t.Filter(msg) {
				messages = append(messages, msg)
			}
		}
	}

	return
}

func (t *Tester) log(message string) {
	if t.Logger == nil {
		return
	}

	t.Logger.Println(message)
}
