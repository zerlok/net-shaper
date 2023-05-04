package netshaper_test

import (
	"context"
	netWs "golang.org/x/net/websocket"
	"io"
	netHttp "net/http"
	"netshaper"
	"netshaper/factory"
	"netshaper/http"
	"netshaper/test"
	"netshaper/websocket"
	"reflect"
	"testing"
	"time"
)

func TestClientRequest(t *testing.T) {
	t.Run("simple http", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		want := []byte("hey")
		url, srv := test.NewHttpHandlerFunc("/", func(writer netHttp.ResponseWriter, request *netHttp.Request) {
			//goland:noinspection GoUnhandledErrorResult
			writer.Write(want)
		})
		defer srv.Close()

		client, err := netshaper.NewClient(ctx, http.NewNet(http.WithNetClient(srv.Client())))
		if err != nil {
			t.Errorf("NewClient() error got = %#v, want nil", err)
		}

		req, err := http.NewGetRequest(ctx, url, nil)
		if err != nil {
			t.Errorf("http.NewRequestWithContext() error got = %#v, want nil", err)
		}

		res, err := client.Request(req)
		if err != nil {
			t.Errorf("client.Request() error got = %#v, want nil", err)
		}

		got, err := io.ReadAll(res.Body)
		if err != nil {
			t.Errorf("client.Request() response body error got = %#v, want nil", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("client.Request() response body got = %#v, want %#v", got, want)
		}
	})

	t.Run("http with pool, rate limiter and exponential retries on bad status", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		want := []byte("hey")
		url, srv := test.NewHttpHandlerFunc("/", func(writer netHttp.ResponseWriter, request *netHttp.Request) {
			//goland:noinspection GoUnhandledErrorResult
			writer.Write(want)
		})
		defer srv.Close()

		client, err := factory.DefaultHttp(ctx)
		if err != nil {
			t.Errorf("NewClient() error got = %#v, want nil", err)
		}

		req, err := http.NewGetRequest(ctx, url, nil)
		if err != nil {
			t.Errorf("http.NewRequestWithContext() error got = %#v, want nil", err)
		}

		res, err := client.Request(req)
		if err != nil {
			t.Errorf("client.Request() error got = %#v, want nil", err)
		}

		got, err := io.ReadAll(res.Body)
		if err != nil {
			t.Errorf("client.Request() response body error got = %#v, want nil", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("client.Request() response body got = %#v, want %#v", got, want)
		}
	})

	t.Run("simple websocket", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		want := []websocket.Message{websocket.ByteMessage("hey")}
		url, srv := test.NewWsHandler(func(conn *netWs.Conn) {
			writer, _ := conn.NewFrameWriter(netWs.TextFrame)
			//goland:noinspection GoUnhandledErrorResult
			defer writer.Close()
			for _, msg := range want {
				//goland:noinspection GoUnhandledErrorResult
				writer.Write(msg.Buff())
			}
		})
		defer srv.Close()

		client, err := netshaper.NewClient(ctx, websocket.NewNet())
		if err != nil {
			t.Errorf("NewClient() error got = %#v, want nil", err)
		}

		listenCtx, listenCancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer listenCancel()
		got, err := websocket.ListenSliceRawMessages(listenCtx, client, &websocket.Request{
			Ctx: ctx,
			URL: url,
		}, 2)
		if err != nil {
			t.Errorf("websocket.ListenSliceRawMessages() error got = %#v, want nil", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("websocket.ListenSliceRawMessages() messages got = %#v, want %#v", got, want)
		}
	})
}
