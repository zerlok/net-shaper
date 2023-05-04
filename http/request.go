package http

import (
	"bytes"
	"context"
	"net/http"
	net_shaper "netshaper"
)

func NewRequest(ctx context.Context, method string, url net_shaper.URL, headers net_shaper.Headers, body net_shaper.Body) (req *Request, err error) {
	if body == nil {
		req, err = http.NewRequestWithContext(ctx, method, url.String(), nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url.String(), bytes.NewBuffer(body))
	}
	if err != nil {
		return
	}

	if headers != nil {
		req.Header = headers
	}

	return
}

func NewGetRequest(ctx context.Context, url net_shaper.URL, headers net_shaper.Headers) (*Request, error) {
	return NewRequest(ctx, http.MethodGet, url, headers, nil)
}

func NewHeadRequest(ctx context.Context, url net_shaper.URL, headers net_shaper.Headers) (*Request, error) {
	return NewRequest(ctx, http.MethodHead, url, headers, nil)
}

func NewPostRequest(ctx context.Context, url net_shaper.URL, headers net_shaper.Headers, body net_shaper.Body) (*Request, error) {
	return NewRequest(ctx, http.MethodPost, url, headers, body)
}

func NewPutRequest(ctx context.Context, url net_shaper.URL, headers net_shaper.Headers, body net_shaper.Body) (*Request, error) {
	return NewRequest(ctx, http.MethodPut, url, headers, body)
}

func NewPatchRequest(ctx context.Context, url net_shaper.URL, headers net_shaper.Headers, body net_shaper.Body) (*Request, error) {
	return NewRequest(ctx, http.MethodPatch, url, headers, body)
}

func NewDeleteRequest(ctx context.Context, url net_shaper.URL, headers net_shaper.Headers, body net_shaper.Body) (*Request, error) {
	return NewRequest(ctx, http.MethodDelete, url, headers, body)
}

func NewConnectRequest(ctx context.Context, url net_shaper.URL, headers net_shaper.Headers) (*Request, error) {
	return NewRequest(ctx, http.MethodConnect, url, headers, nil)
}

func NewOptionsRequest(ctx context.Context, url net_shaper.URL, headers net_shaper.Headers) (*Request, error) {
	return NewRequest(ctx, http.MethodOptions, url, headers, nil)
}

func NewTraceRequest(ctx context.Context, url net_shaper.URL, headers net_shaper.Headers) (*Request, error) {
	return NewRequest(ctx, http.MethodTrace, url, headers, nil)
}

type Request = http.Request
