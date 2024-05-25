package main

type Config struct {
	EnableRead  bool `json:"enable_read" envoy:"mergeable"`
	EnableWrite bool `json:"enable_write" envoy:"mergeable"`
}
