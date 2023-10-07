package types

import (
	"net/http"
	"net/url"
)

// HeaderRange is an interface that implement Range function for loop over a header values, and replicate it to the target
type HeaderRange interface {
	// Range calls f sequentially for each key and value present in the map.
	// If f returns false, range stops the iteration.
	// When there are multiple values of a key, f will be invoked multiple times with the same key and each value.

	Range(f func(key string, value string) bool)
}

type RequestOption func(req *http.Request) error

// NewRequest return valid http.Request struct from given arguments
func NewRequest(method, host string, opts ...RequestOption) (*http.Request, error) {
	req := new(http.Request)

	req.Method = method
	req.Host = host

	for _, opt := range opts {
		if err := opt(req); err != nil {
			return nil, err
		}
	}

	return req, nil
}

func WithRequestHeaderRangeSetter(hr HeaderRange) RequestOption {
	return func(req *http.Request) error {
		if req.Header == nil {
			req.Header = make(http.Header)
		}

		hr.Range(func(k string, v string) bool {
			req.Header.Add(k, v)
			return true
		})

		return nil
	}
}

func WithRequestHeader(headers map[string][]string) RequestOption {
	return func(req *http.Request) error {
		if req.Header == nil {
			req.Header = make(http.Header)
		}

		for k, v := range headers {
			req.Header[http.CanonicalHeaderKey(k)] = v
		}
		return nil
	}
}

func WithRequestURI(rawURI string) RequestOption {
	return func(req *http.Request) error {
		parsedURL, err := url.ParseRequestURI(rawURI)
		if err != nil {
			return err
		}

		req.URL = parsedURL
		return nil
	}
}
