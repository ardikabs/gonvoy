package gonvoy

import (
	"fmt"
)

// Filter is a user filter registered through envoy.MustRegister.
// A filter implements the phase interfaces for the phases it handles;
// phases without an implementation are skipped.
type Filter interface{}

// RequestHeaderFilter handles the HTTP request header phase.
type RequestHeaderFilter interface {
	Filter

	// OnRequestHeader is called when processing the HTTP request header.
	OnRequestHeader(c Context) error
}

// RequestBodyFilter handles the HTTP request body phase.
type RequestBodyFilter interface {
	Filter

	// OnRequestBody is called when processing the HTTP request body.
	OnRequestBody(c Context) error
}

// ResponseHeaderFilter handles the HTTP response header phase.
type ResponseHeaderFilter interface {
	Filter

	// OnResponseHeader is called when processing the HTTP response header.
	OnResponseHeader(c Context) error
}

// ResponseBodyFilter handles the HTTP response body phase.
type ResponseBodyFilter interface {
	Filter

	// OnResponseBody is called when processing the HTTP response body.
	OnResponseBody(c Context) error
}

// CompleteFilter handles filter completion.
type CompleteFilter interface {
	Filter

	// OnComplete is called when the filter is about to be destroyed.
	OnComplete(c Context) error
}

// Composer builds the per-request handler chain.
// It runs once per request, after the filter configuration is loaded.
//
// Composition is single-level: a Composer returned inside another
// composer's Use slice runs as a phase-only filter, its own Use
// is never called.
type Composer interface {
	Filter

	// Use returns the filters handling the current request.
	// A non-nil error ignores the filter and logs, matching OnBegin failure.
	Use(c Context) ([]Filter, error)
}

// filterConfigSource is implemented by contexts carrying a filter config.
type filterConfigSource interface {
	// GetFilterConfig returns the filter configuration associated with the route.
	GetFilterConfig() any
}

// cacheSource is implemented by contexts carrying an internal cache.
type cacheSource interface {
	// GetCache returns a Cache.
	GetCache() Cache
}

// FilterConfig returns the typed filter configuration for the current request.
// Use the same type passed as Options.Config during registration.
func FilterConfig[T any](c Context) (T, error) {
	var zero T

	src, ok := c.(filterConfigSource)
	if !ok {
		return zero, fmt.Errorf("filter config is not available, %w", errInternalConfigNotFound)
	}

	v := src.GetFilterConfig()
	if v == nil {
		return zero, fmt.Errorf("filter config is not available, %w", errInternalConfigNotFound)
	}

	typed, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("unexpected filter config type '%T', expecting '%T'", v, zero)
	}

	return typed, nil
}

// FilterCache returns the internal cache for the current request.
// Use it when variable initialization is expensive or requires statefulness.
func FilterCache(c Context) Cache {
	src, ok := c.(cacheSource)
	if !ok {
		return nil
	}

	return src.GetCache()
}
