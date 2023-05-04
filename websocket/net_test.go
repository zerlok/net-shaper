package websocket

import (
	"context"
	"fmt"
	"golang.org/x/net/websocket"
	"net/http/httptest"
	"net/url"
	"netshaper"
	"reflect"
	"testing"
	"time"
)

func TestNetRequestMessages(t *testing.T) {
	tests := []struct {
		name         string
		tester       *Tester
		server       *websocket.Server
		wantMessages []Message
	}{
		{
			name: "no messages",
			tester: &Tester{
				RequestsAmount:          1,
				ListenMessagesMaxAmount: 1,
				ListenTimeout:           100 * time.Millisecond,
			},
			server: &websocket.Server{
				Handler: func(conn *websocket.Conn) {
					_ = conn.WriteClose(websocket.CloseFrame)
				},
			},
			wantMessages: []Message{},
		},
		{
			name: "10 messages",
			tester: &Tester{
				RequestsAmount:          1,
				ListenMessagesMaxAmount: 11,
				ListenTimeout:           100 * time.Millisecond,
			},
			server: &websocket.Server{
				Handler: func(conn *websocket.Conn) {
					writer, _ := conn.NewFrameWriter(websocket.TextFrame)
					//goland:noinspection GoUnhandledErrorResult
					defer writer.Close()
					for i := 0; i < 10; i++ {
						//goland:noinspection GoUnhandledErrorResult
						writer.Write([]byte(fmt.Sprintf("hey#%03d", i)))
					}
				},
			},
			wantMessages: []Message{
				ByteMessage("hey#000"),
				ByteMessage("hey#001"),
				ByteMessage("hey#002"),
				ByteMessage("hey#003"),
				ByteMessage("hey#004"),
				ByteMessage("hey#005"),
				ByteMessage("hey#006"),
				ByteMessage("hey#007"),
				ByteMessage("hey#008"),
				ByteMessage("hey#009"),
			},
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

			srv := httptest.NewServer(tt.server)
			defer srv.Close()

			endpoint, _ := url.Parse(srv.URL)
			endpoint.Scheme = "ws"

			cl, err := netshaper.New(NewNet()).Create(ctx)
			if err != nil {
				t.Errorf("Create() error = %v", err)
				return
			}

			gotMessages, _ := tt.tester.RequestMessages(cl, &Request{
				Ctx: ctx,
				URL: *endpoint,
			})

			if !reflect.DeepEqual(gotMessages, tt.wantMessages) {
				t.Errorf("Request().Listen() messages got = %v, want %v", gotMessages, tt.wantMessages)
			}
		})
	}
}

func TestNetRequestErrors(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		want := &websocket.DialError{Config: &websocket.Config{}}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		cl, err := netshaper.New(NewNet()).Create(ctx)
		if err != nil {
			t.Errorf("Create() error = %v", err)
			return
		}

		_, err = cl.Request(&Request{
			Ctx: ctx,
			URL: url.URL{
				Scheme: "ws",
				Host:   "localhost",
			},
		})

		switch e := err.(type) {
		case *websocket.DialError:
			break
		default:
			t.Errorf("Request() error got = %v, want %v", e, want)
		}
	})

	t.Run("message receive timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		timeout := 50 * time.Millisecond

		srv := httptest.NewServer(&websocket.Server{
			Handler: func(conn *websocket.Conn) {
				writer, _ := conn.NewFrameWriter(websocket.TextFrame)
				//goland:noinspection GoUnhandledErrorResult
				defer writer.Close()
				time.Sleep(timeout * 5)
			},
		})
		defer srv.Close()

		endpoint, _ := url.Parse(srv.URL)
		endpoint.Scheme = "ws"

		cl, err := NewNet(WithNetReceiveTimeout(timeout)).Apply(nil).Create(ctx)
		if err != nil {
			t.Errorf("Create() error = %v", err)
			return
		}

		res, err := cl.Request(&Request{
			Ctx: ctx,
			URL: *endpoint,
		})
		if err != nil {
			t.Errorf("Request() error got = %v, want nil", err)
		}

		defer res.Close(ctx)

		msg, ok := <-res.Listen()
		if ok {
			t.Errorf("Request().Listen() should be closed")
		}
		if msg != nil {
			t.Errorf("Request().Listen() message got = %v, want nil", msg)
		}

		select {
		case <-res.Closed():
			break
		default:
			t.Errorf("Request().Closed() must be done")
		}
	})
}
