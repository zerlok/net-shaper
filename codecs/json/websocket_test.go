package json

import (
	"context"
	"errors"
	netWs "golang.org/x/net/websocket"
	net_shaper "netshaper"
	"netshaper/test"
	"netshaper/websocket"
	"reflect"
	"testing"
	"time"
)

type testMessage struct {
	value string
}

func TestRequestWebsocket(t *testing.T) {
	t.Run("invalid json message", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		wantMsg := &WebsocketMessage[testMessage]{
			Raw:   websocket.ByteMessage(`{"value": "unexpected json eof"`),
			Value: nil,
			Error: &ParseError[testMessage]{
				Value: &testMessage{},
				Err:   errors.New("unexpected end of JSON input"),
			},
		}

		url, srv := test.NewWsHandler(func(conn *netWs.Conn) {
			writer, _ := conn.NewFrameWriter(netWs.TextFrame)
			//goland:noinspection GoUnhandledErrorResult
			defer writer.Close()
			//goland:noinspection GoUnhandledErrorResult
			writer.Write(wantMsg.Buff())
		})
		defer srv.Close()

		cl, err := net_shaper.New(websocket.NewNet()).Create(ctx)
		if err != nil {
			t.Errorf("Create() error = %v", err)
			return
		}

		res, err := RequestWebsocket[testMessage](cl, &websocket.Request{
			Ctx: ctx,
			URL: url,
		})
		if err != nil {
			t.Errorf("Request() error got = %v, want nil", err)
		}
		defer res.Close(ctx)

		msg, ok := <-res.Listen()

		if !ok {
			t.Errorf("Request().Listen() should not be closed")
		}
		if !reflect.DeepEqual(msg.Value, wantMsg.Value) {
			t.Errorf("Request().Listen() message value got = %#v, want %#v", msg.Value, wantMsg.Value)
		}
		if !reflect.DeepEqual(msg.Buff(), wantMsg.Buff()) {
			t.Errorf("Request().Listen() message buff got = %#v, want %#v", msg.Buff(), wantMsg.Buff())
		}
		if msg.Err().Error() != wantMsg.Err().Error() {
			t.Errorf("Request().Listen() message error got = %#v, want %#v", msg.Err().Error(), wantMsg.Err().Error())
		}
	})
}
