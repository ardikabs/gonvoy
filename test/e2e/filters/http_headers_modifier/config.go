package main

type Config struct {
	RequestHeaders  map[string]string `json:"request_headers,omitempty" envoy:"mergeable,preserveRoot"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty" envoy:"mergeable,preserveRoot"`
}
