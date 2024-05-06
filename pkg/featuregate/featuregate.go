package featuregate

import (
	"os"
	"strings"
)

var prefix = "GONVOY_"

var (
	AllowRequestHeaderPhase  = setFeature("ALLOW_REQUEST_HEADER_PHASE", true)
	AllowRequestBodyPhase    = setFeature("ALLOW_REQUEST_BODY_PHASE", true)
	AllowResponseHeaderPhase = setFeature("ALLOW_RESPONSE_HEADER_PHASE", true)
	AllowResponseBodyPhase   = setFeature("ALLOW_RESPONSE_BODY_PHASE", true)
)

func setFeature(param string, defVal bool) func() bool {
	return func() bool {
		value, ok := os.LookupEnv(prefix + param)
		if value == "" || !ok {
			return defVal
		}

		value = strings.ToLower(value)
		return value == "true" || value == "1" || value == "yes" || value == "enabled"
	}
}
