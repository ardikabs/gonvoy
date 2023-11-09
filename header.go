package envoy

import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HeaderWriter interface {
	api.HeaderMap

	AsMap() map[string]string
}

var _ HeaderWriter = &headerWriter{}

type headerWriter struct {
	api.HeaderMap
}

func (h *headerWriter) AsMap() map[string]string {
	flatHeader := make(map[string]string)

	h.Range(func(key, value string) bool {
		if v, ok := flatHeader[key]; ok {
			flatHeader[key] = v + ", " + value
			return true
		}

		flatHeader[key] = value
		return true
	})

	return flatHeader
}
