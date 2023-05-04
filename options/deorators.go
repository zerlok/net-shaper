package options

import (
	"fmt"
	netHttp "net/http"
	"netshaper"
)

func MakeResponseStatusCodesAsErrorsDecorator[T netshaper.Request](codes ...int) Decorator[T, *netHttp.Response] {
	if len(codes) == 0 {
		return nil
	}

	codesMap := map[int]struct{}{}
	for _, code := range codes {
		codesMap[code] = struct{}{}
	}

	return MakeResponseErrorCheckDecorator[T](func(res *netHttp.Response) error {
		if res != nil {
			if _, ok := codesMap[res.StatusCode]; ok {
				return fmt.Errorf("invalid status code %v", res.StatusCode)
			}
		}

		return nil
	})
}

func MakeResponseErrorCheckDecorator[T1 netshaper.Request, T2 any](checker func(res T2) error) Decorator[T1, T2] {
	if checker == nil {
		return nil
	}

	return PostProcessingFunc[T1, T2](func(req T1, res T2, err error) (T2, error) {
		if err == nil {
			if err = checker(res); err != nil {
				return res, err
			}
		}

		return res, err
	})
}

type PreProcessingFunc[T1 netshaper.Request, T2 any] func(req T1) T1

func (fn PreProcessingFunc[T1, T2]) Enter(req T1) T1 {
	return fn(req)
}

func (fn PreProcessingFunc[T1, T2]) Exit(_ T1, res T2, err error) (T2, error) {
	return res, err
}

type PostProcessingFunc[T1 netshaper.Request, T2 any] func(req T1, res T2, err error) (T2, error)

func (fn PostProcessingFunc[T1, T2]) Enter(req T1) T1 {
	return req
}

func (fn PostProcessingFunc[T1, T2]) Exit(req T1, res T2, err error) (T2, error) {
	return fn(req, res, err)
}
