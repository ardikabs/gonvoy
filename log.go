package envoy

import "github.com/envoyproxy/envoy/contrib/golang/common/go/api"

type LogLevel api.LogType

const (
	DebugLevel LogLevel = iota + 1
	InfoLevel
	WarnLevel
	ErrorLevel
	CriticalLevel
)
