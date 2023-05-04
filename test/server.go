package test

import (
	netWs "golang.org/x/net/websocket"
	netHttp "net/http"
	"net/http/httptest"
	"netshaper"
)

func NewHttpHandlerFunc(path string, handler netHttp.HandlerFunc) (netshaper.URL, *httptest.Server) {
	mux := netHttp.NewServeMux()
	mux.HandleFunc(path, handler)

	url, srv := NewHttpServer(mux)
	url.Path = path

	return url, srv
}

func NewHttpServer(mux *netHttp.ServeMux) (netshaper.URL, *httptest.Server) {
	srv := httptest.NewServer(mux)
	url, err := netshaper.ParseURL(srv.URL)
	if err != nil {
		panic(err)
	}

	return *url, srv
}

func NewWsHandler(handler func(conn *netWs.Conn)) (netshaper.URL, *httptest.Server) {
	mux := &netWs.Server{
		Handler: handler,
	}
	url, srv := NewWsServer(mux)

	return url, srv
}

func NewWsServer(mux *netWs.Server) (netshaper.URL, *httptest.Server) {
	srv := httptest.NewServer(mux)

	url, _ := netshaper.ParseURL(srv.URL)
	url.Scheme = "ws"

	return *url, srv
}
