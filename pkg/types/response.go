package types

import (
	"net/http"
)

type ResponseOption func(res *http.Response) error

// NewResponse return valid http.Response struct from given arguments
func NewResponse(status int, opts ...ResponseOption) (*http.Response, error) {
	res := new(http.Response)
	res.StatusCode = status

	for _, opt := range opts {
		if err := opt(res); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func WithResponseHeaderRangeSetter(hr HeaderRange) ResponseOption {
	return func(res *http.Response) error {
		if res.Header == nil {
			res.Header = make(http.Header)
		}

		hr.Range(func(k string, v string) bool {
			res.Header.Add(k, v)
			return true
		})

		return nil
	}
}
