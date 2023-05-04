package websocket

import (
	"context"
	"errors"
	"fmt"
	"netshaper/conf"
	"reflect"
	"testing"
	"time"
)

func TestMockRequest(t *testing.T) {
	tests := []struct {
		name         string
		mock         *MockConfig
		tester       *Tester
		wantMessages []Message
		wantErrors   []error
	}{
		{
			name: "1 request without messages",
			mock: NewMockConfig(),
			tester: &Tester{
				RequestsAmount:          1,
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 1,
			},
			wantMessages: []Message{},
			wantErrors:   []error{nil},
		},
		{
			name: "1 request with 1 message",
			mock: NewMockConfig(WithSideEffects(WithResponseMessages(TextMessage("hello, world!")))),
			tester: &Tester{
				RequestsAmount:          1,
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 10,
			},
			wantMessages: []Message{
				TextMessage("hello, world!"),
			},
			wantErrors: []error{nil},
		},
		{
			name: "1 request with 10 messages",
			mock: NewMockConfig(WithSideEffects(WithResponseMessages(
				TextMessage("0"),
				TextMessage("1"),
				TextMessage("2"),
				TextMessage("3"),
				TextMessage("4"),
				TextMessage("5"),
				TextMessage("6"),
				TextMessage("7"),
				TextMessage("8"),
				TextMessage("9"),
			))),
			tester: &Tester{
				RequestsAmount:          1,
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 20,
			},
			wantMessages: []Message{
				TextMessage("0"),
				TextMessage("1"),
				TextMessage("2"),
				TextMessage("3"),
				TextMessage("4"),
				TextMessage("5"),
				TextMessage("6"),
				TextMessage("7"),
				TextMessage("8"),
				TextMessage("9"),
			},
			wantErrors: []error{nil},
		},
		{
			name: "3 requests with 3 messages each",
			mock: NewMockConfig(WithSideEffectsForNRequestsFunc(func(i int) []conf.Option[*MockResponse] {
				return WithSideEffectForSingleRequest(WithResponseMessages(
					TextMessage(fmt.Sprintf("req#%03d/msg#000", i)),
					TextMessage(fmt.Sprintf("req#%03d/msg#001", i)),
					TextMessage(fmt.Sprintf("req#%03d/msg#002", i)),
				))
			}, 100)),
			tester: &Tester{
				RequestsAmount:          3,
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 10,
			},
			wantMessages: []Message{
				TextMessage("req#000/msg#000"),
				TextMessage("req#000/msg#001"),
				TextMessage("req#000/msg#002"),
				TextMessage("req#001/msg#000"),
				TextMessage("req#001/msg#001"),
				TextMessage("req#001/msg#002"),
				TextMessage("req#002/msg#000"),
				TextMessage("req#002/msg#001"),
				TextMessage("req#002/msg#002"),
			},
			wantErrors: []error{nil, nil, nil},
		},
		{
			name: "1 failed request",
			mock: NewMockConfig(WithSideEffects(WithRequestErrorReason("test invalid request"))),
			tester: &Tester{
				RequestsAmount:          1,
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 1,
			},
			wantMessages: []Message{},
			wantErrors:   []error{fmt.Errorf("test invalid request")},
		},
		{
			name: "1 request with 3 text and 2 error messages",
			mock: NewMockConfig(WithSideEffects(WithResponseMessages(
				TextMessage("0"),
				&ErrorMessage{errors.New("err1")},
				TextMessage("2"),
				&ErrorMessage{errors.New("err3")},
				TextMessage("4"),
			))),
			tester: &Tester{
				RequestsAmount:          1,
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 6,
			},
			wantMessages: []Message{
				TextMessage("0"),
				&ErrorMessage{errors.New("err1")},
				TextMessage("2"),
				&ErrorMessage{errors.New("err3")},
				TextMessage("4"),
			},
			wantErrors: []error{nil},
		},
		{
			name: "1 request with 5 texts and close after 3rd",
			mock: NewMockConfig(WithSideEffects(
				WithResponseMessages(
					TextMessage("0"),
					TextMessage("1"),
					TextMessage("2"),
				),
				WithResponseClose(),
				WithResponseMessages(
					TextMessage("3"),
					TextMessage("4"),
				),
			)),
			tester: &Tester{
				RequestsAmount:          1,
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 4,
			},
			wantMessages: []Message{
				TextMessage("0"),
				TextMessage("1"),
				TextMessage("2"),
			},
			wantErrors: []error{nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if timeout := tt.tester.ListenTimeout; timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.tester.ListenTimeout*100)
				defer cancel()
			}

			mock := tt.mock.create(ctx)
			gotMessages, gotErrors := tt.tester.RunMessages(ctx, mock)
			mock.Close(ctx)

			if gotAmount := uint(len(mock.Requests())); gotAmount != tt.tester.RequestsAmount {
				t.Errorf("len(mock.Requests()) got = %v, want %v", gotAmount, tt.tester.RequestsAmount)
			}
			if !reflect.DeepEqual(gotErrors, tt.wantErrors) {
				t.Errorf("Request().Listen() errors got = %v, want %v", gotErrors, tt.wantErrors)
			}
			if !reflect.DeepEqual(gotMessages, tt.wantMessages) {
				t.Errorf("Request().Listen() messages got = %v, want %v", gotMessages, tt.wantMessages)
			}
		})
	}
}
