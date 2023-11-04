package types

import (
	"fmt"
	"net/http"
	"net/url"
)

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
			return fmt.Errorf("failed to parse URL, %w", err)
		}

		req.URL = parsedURL
		return nil
	}
}
