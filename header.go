package envoy

import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type Header interface {
	api.HeaderMap

	AsMap() map[string]string
}

var _ Header = &header{}

type header struct {
	api.HeaderMap
}

func (h *header) AsMap() map[string]string {
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
