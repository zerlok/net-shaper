package websocket

import (
	"context"
	"netshaper/timer"
	"reflect"
	"testing"
	"time"
)

func TestRobustRequest(t *testing.T) {
	tests := []struct {
		name                    string
		mock                    *MockConfig
		tester                  *Tester
		autoRefresh             timer.Ticker
		wantInnerRequestsAmount int
		wantMessages            []Message
		wantErrors              []error
	}{
		{
			name: "no messages",
			mock: NewMockConfig(),
			tester: &Tester{
				RequestsAmount:          1,
				ListenMessagesMaxAmount: 2,
				ListenTimeout:           100 * time.Millisecond,
			},
			wantInnerRequestsAmount: 1,
			wantMessages:            []Message{},
			wantErrors:              []error{nil},
		},
		{
			name: "10 messages without restart",
			mock: NewMockConfig(WithSideEffectsForNRequests(
				WithSideEffectForSingleRequest(WithResponseMessages(
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
				)),
			)),
			tester: &Tester{
				RequestsAmount:          1,
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 11,
			},
			wantInnerRequestsAmount: 1,
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
			name: "7 messages with 3 closes",
			mock: NewMockConfig(WithSideEffectsForNRequests(
				WithSideEffectForSingleRequest(WithResponseMessages(TextMessage("0"), TextMessage("1")), WithResponseClose()),
				WithSideEffectForSingleRequest(WithResponseMessages(TextMessage("2")), WithResponseClose()),
				WithSideEffectForSingleRequest(WithResponseMessages(TextMessage("3"), TextMessage("4")), WithResponseClose()),
				WithSideEffectForSingleRequest(WithResponseMessages(TextMessage("5"), TextMessage("6"))),
			)),
			tester: &Tester{
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 8,
			},
			wantInnerRequestsAmount: 4,
			wantMessages: []Message{
				TextMessage("0"),
				TextMessage("1"),
				TextMessage("2"),
				TextMessage("3"),
				TextMessage("4"),
				TextMessage("5"),
				TextMessage("6"),
			},
			wantErrors: []error{nil},
		},
		{
			name: "10 messages with 8 restarts",
			mock: NewMockConfig(WithSideEffectsForNRequests(
				WithSideEffectForSingleRequest(
					WithRequestErrorReason("test failed to connect"),
				),
				WithSideEffectForSingleRequest(
					WithResponseMessages(
						TextMessage("0"),
						TextMessage("1"),
					),
					WithResponseClose(),
				),
				WithSideEffectForSingleRequest(
					WithResponseMessages(
						TextMessage("2"),
						TextMessage("3"),
					),
					WithResponseClose(),
				),
				WithSideEffectForSingleRequest(
					WithRequestErrorReason("test failed to connect"),
				),
				WithSideEffectForSingleRequest(
					WithRequestErrorReason("test failed to connect"),
				),
				WithSideEffectForSingleRequest(
					WithResponseMessages(
						TextMessage("4"),
						TextMessage("5"),
					),
					WithResponseClose(),
				),
				WithSideEffectForSingleRequest(
					WithResponseClose(),
				),
				WithSideEffectForSingleRequest(
					WithResponseMessages(
						TextMessage("6"),
						TextMessage("7"),
						TextMessage("8"),
						TextMessage("9"),
					),
				),
			)),
			tester: &Tester{
				ListenTimeout:           100 * time.Millisecond,
				ListenMessagesMaxAmount: 11,
			},
			wantInnerRequestsAmount: 8,
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
			name: "8 messages with 2 auto restarts",
			mock: NewMockConfig(WithSideEffectsForNRequests(
				WithSideEffectForSingleRequest(
					WithResponseMessages(
						TextMessage("0"),
						TextMessage("1"),
						TextMessage("2"),
						TextMessage("3"),
					),
				),
				WithSideEffectForSingleRequest(
					WithResponseMessages(
						TextMessage("4"),
						TextMessage("5"),
						TextMessage("6"),
						TextMessage("7"),
					),
				),
				WithSideEffectForSingleRequest(),
			)),
			tester: &Tester{
				ListenTimeout:           50 * time.Millisecond,
				ListenMessagesMaxAmount: 11,
			},
			autoRefresh: timer.Ticker{
				Period: 20 * time.Millisecond,
			},
			wantInnerRequestsAmount: 3,
			wantMessages: []Message{
				TextMessage("0"),
				TextMessage("1"),
				TextMessage("2"),
				TextMessage("3"),
				TextMessage("4"),
				TextMessage("5"),
				TextMessage("6"),
				TextMessage("7"),
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
			cl, err := (&RobustConfig{Inner: mock, AutoRefresh: tt.autoRefresh}).Create(ctx)
			if err != nil {
				t.Errorf("Create() error = %v", err)
				return
			}

			gotMessages, gotErrors := tt.tester.RequestMessages(cl, &Request{
				Ctx:        ctx,
				BufferSize: DefaultWsBuffSize,
			})

			if gotLen := len(mock.Requests()); gotLen != tt.wantInnerRequestsAmount {
				t.Errorf("len(mock.Requests()) got = %v, want %v", gotLen, tt.wantInnerRequestsAmount)
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
