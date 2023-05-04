package query

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func EncodeQueryMultiParams(params map[string][]any) (string, error) {
	query := &url.Values{}

	for key, values := range params {
		for _, value := range values {
			s, err := encodeQueryParamValue(value)
			if err != nil {
				return query.Encode(), err
			}

			query.Add(key, s)
		}
	}

	return query.Encode(), nil
}

func EncodeQueryMultiParamsNoEscape(params map[string][]any) (string, error) {
	ss := strings.Builder{}

	i := 0
	for key, values := range params {
		for _, value := range values {
			s, err := encodeQueryParamValue(value)
			if err != nil {
				return ss.String(), err
			}

			if i > 0 {
				ss.WriteRune('&')
			}

			ss.WriteString(key)
			ss.WriteRune('=')
			ss.WriteString(s)

			i++
		}
	}

	return ss.String(), nil
}

func EncodeQueryParams(params map[string]any) (string, error) {
	return EncodeQueryMultiParams(toMultiParams(params))
}

func EncodeQueryParamsNoEscape(params map[string]any) (string, error) {
	return EncodeQueryMultiParamsNoEscape(toMultiParams(params))
}

func encodeQueryParamValue(value any) (s string, err error) {
	switch value.(type) {
	case int:
		s = strconv.Itoa(value.(int))
	case string:
		s = value.(string)
	default:
		err = fmt.Errorf("unsupported query params type %T: %#v", value, value)
	}

	return
}

func toMultiParams(params map[string]any) (multiParams map[string][]any) {
	multiParams = make(map[string][]any)

	for key, value := range params {
		multiParams[key] = append([]any{}, value)
	}

	return
}
