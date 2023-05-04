package json

import (
	"context"
	"encoding/json"
	"io"
	"netshaper"
	"netshaper/http"
)

type HttpResponse[T any] http.Response

func (r *HttpResponse[T]) Value() (*T, error) {
	return DecodeBody[T](r.Body)
}

func RequestHttp[T1 any, T2 any](ctx context.Context, client http.Client, method string, url netshaper.URL, headers netshaper.Headers, body *T1) (response *HttpResponse[T2], err error) {
	req, err := NewHttpRequest(ctx, method, url, headers, body)
	if err != nil {
		return
	}

	res, err := client.Request(req)
	if err != nil {
		return
	}

	response = (*HttpResponse[T2])(res)

	return
}

func NewHttpRequest[T1 any](ctx context.Context, method string, url netshaper.URL, headers netshaper.Headers, body *T1) (*http.Request, error) {
	byteBody, err := EncodeBody(body)
	if err != nil {
		return nil, err
	}

	return http.NewRequest(ctx, method, url, headers, byteBody)
}

func EncodeBody[T any](body *T) (buff []byte, err error) {
	if body != nil {
		buff, err = json.Marshal(body)
	} else {
		buff = nil
	}

	return
}

func DecodeBody[T any](body io.Reader) (value *T, err error) {
	buff, err := io.ReadAll(body)
	if err != nil {
		return
	}

	return Parse[T](buff)
}
