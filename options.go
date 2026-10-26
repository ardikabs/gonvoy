package gonvoy

// BodyAccess controls when HTTP body access is allowed.
type BodyAccess int

const (
	// BodyAccessHeaderGated allows body access only with the presence
	// of the `X-Content-Operation` header. This is the default.
	BodyAccessHeaderGated BodyAccess = iota

	// BodyAccessOpen allows body access without the `X-Content-Operation` header.
	BodyAccessOpen
)

// BodyRule controls read and write access to an HTTP body.
// ReadWrite implies read.
type BodyRule int

const (
	// BodyRuleNone disables body access. This is the default.
	BodyRuleNone BodyRule = iota

	// BodyRuleRead allows reading the body; writes panic.
	BodyRuleRead

	// BodyRuleReadWrite allows reading and modifying the body.
	BodyRuleReadWrite
)

// KeepLength controls Content-Length preservation on body writes.
type KeepLength int

const (
	// KeepLengthNone removes Content-Length on body writes. This is the default.
	KeepLengthNone KeepLength = iota

	// KeepLengthRequest preserves Content-Length on request body writes.
	KeepLengthRequest

	// KeepLengthResponse preserves Content-Length on response body writes.
	KeepLengthResponse

	// KeepLengthBoth preserves Content-Length on request and response body writes.
	KeepLengthBoth
)

// MergeOnError controls merge failure handling for struct filter configs.
type MergeOnError int

const (
	// MergeOnErrorPanic panics on merge failure. This is the default.
	MergeOnErrorPanic MergeOnError = iota

	// MergeOnErrorFallbackToParent falls back to the parent config on merge failure.
	MergeOnErrorFallbackToParent
)

// BodyPolicy describes HTTP body access for a filter.
type BodyPolicy struct {
	// Access gates body access on the `X-Content-Operation` header.
	Access BodyAccess

	// Request controls request body access.
	Request BodyRule

	// Response controls response body access.
	Response BodyRule

	// KeepContentLength preserves Content-Length on body writes.
	KeepContentLength KeepLength
}

// RoutePolicy describes routing behavior for a filter.
type RoutePolicy struct {
	// ReloadOnHeaderChange re-evaluates routing decisions when request headers change.
	ReloadOnHeaderChange bool
}

// MergePolicy describes parent/child config merge behavior for a filter.
type MergePolicy struct {
	// ChildWins skips the field merge; the child config is the config.
	ChildWins bool

	// OnError handles merge failure for struct filter configs.
	OnError MergeOnError
}

// AttrPolicy describes Envoy attribute prefetching for a filter.
type AttrPolicy struct {
	// Fetch lists the Envoy attribute names prefetched per request for Attr
	// lookups (for example "xds.cluster_name"). Nil (the default) prefetches
	// nothing: only the always-free request fields and the synthesized
	// response code resolve.
	Fetch []string
}

// Options represents the configuration options for a filter.
// Field semantics match ConfigOptions; names say what they do.
type Options struct {
	// Config is the user filter configuration struct.
	Config any

	// MetricsPrefix specifies the prefix used for metrics.
	MetricsPrefix string

	// Route describes routing behavior.
	Route RoutePolicy

	// Merge describes parent/child config merge behavior.
	Merge MergePolicy

	// Body describes HTTP body access.
	Body BodyPolicy

	// Attrs describes Envoy attribute prefetching.
	Attrs AttrPolicy

	// DownstreamTLS snapshots the downstream TLS connection per request.
	DownstreamTLS bool
}

// Option configures a handler registration. Options compose; each sets one
// plain-English behavior on the underlying filter options.
type Option func(*Options)

// WithConfig sets the typed plugin config. Read it back with Config.
func WithConfig(v any) Option {
	return func(o *Options) { o.Config = v }
}

// MetricPrefix sets the prefix used for metrics.
func MetricPrefix(prefix string) Option {
	return func(o *Options) { o.MetricsPrefix = prefix }
}

// ReloadRoute enables re-evaluation of the routing decision when the handler
// changes the host, method, or URL. Envoy name: clear the route cache. Pair
// it with Reroute(r), the per-request action that flags the reload.
func ReloadRoute() Option {
	return func(o *Options) { o.Route.ReloadOnHeaderChange = true }
}

// RouteConfigWins makes the route config replace the listener config
// instead of merging field by field.
func RouteConfigWins() Option {
	return func(o *Options) { o.Merge.ChildWins = true }
}

// EnableEnvoyAttributes declares the Envoy attributes the filter reads
// through Attr (for example "xds.cluster_name"), prefetched per request.
// Without it only request host/method/path and the response code resolve;
// anything else reports absent with a one-time migration warning naming the
// key. The request fields stay free and need no declaration.
func EnableEnvoyAttributes(names ...string) Option {
	return func(o *Options) { o.Attrs.Fetch = names }
}

// EnableDownstreamTLS snapshots the downstream TLS connection per request
// for TLS lookups. Without it TLS reports absent even on TLS listeners
// (with a one-time migration warning when a connection is present).
// Plaintext listeners pay nothing either way.
func EnableDownstreamTLS() Option {
	return func(o *Options) { o.DownstreamTLS = true }
}

// ConfigErrorAction selects merge-failure handling for struct configs.
type ConfigErrorAction int

const (
	// UseListener falls back to the listener config on merge failure.
	UseListener ConfigErrorAction = iota + 1
	// Stop stops the plugin on merge failure.
	Stop
)

// OnConfigError selects merge-failure handling for struct configs.
// The default is Stop, matching today's panic.
func OnConfigError(a ConfigErrorAction) Option {
	return func(o *Options) {
		if a == UseListener {
			o.Merge.OnError = MergeOnErrorFallbackToParent
		} else {
			o.Merge.OnError = MergeOnErrorPanic
		}
	}
}

// ReadRequestBody allows the handler to read r.Body.
func ReadRequestBody() Option {
	return func(o *Options) {
		o.Body.Request = BodyRuleRead
	}
}

// WriteRequestBody allows the handler to replace r.Body. Implies read.
func WriteRequestBody() Option {
	return func(o *Options) {
		o.Body.Request = BodyRuleReadWrite
	}
}

// ReadResponseBody allows after-next code to see the upstream body.
func ReadResponseBody() Option {
	return func(o *Options) {
		o.Body.Response = BodyRuleRead
	}
}

// WriteResponseBody allows replacing the upstream body.
func WriteResponseBody() Option {
	return func(o *Options) {
		o.Body.Response = BodyRuleReadWrite
	}
}

// RequireBodyPermissionHeader keeps the default strict gate: body access
// applies only with the X-Content-Operation header. It is the default;
// pass it to be explicit.
func RequireBodyPermissionHeader() Option {
	return func(o *Options) { o.Body.Access = BodyAccessHeaderGated }
}

// OpenBodyAccess drops the X-Content-Operation gate: body access applies
// whenever the handler reads. Prefer the default gate; use this only when
// intermediaries strip the header.
func OpenBodyAccess() Option {
	return func(o *Options) { o.Body.Access = BodyAccessOpen }
}

// KeepContentLength preserves Content-Length on body writes for the given
// directions. The zero value removes it (chunked encoding).
func KeepContentLength(k KeepLength) Option {
	return func(o *Options) { o.Body.KeepContentLength = k }
}

// ConfigOptionsFromOptions converts Options to legacy ConfigOptions,
// preserving semantics. It is the exact inverse of OptionsFromConfigOptions.
func ConfigOptionsFromOptions(opts Options) ConfigOptions {
	o := ConfigOptions{
		FilterConfig:         opts.Config,
		MetricsPrefix:        opts.MetricsPrefix,
		AutoReloadRoute:      opts.Route.ReloadOnHeaderChange,
		AlwaysUseChildConfig: opts.Merge.ChildWins,
	}

	if opts.Body.Access == BodyAccessOpen {
		o.DisableStrictBodyAccess = true
	}

	if opts.Body.Request == BodyRuleRead || opts.Body.Request == BodyRuleReadWrite {
		o.EnableRequestBodyRead = true
	}

	if opts.Body.Request == BodyRuleReadWrite {
		o.EnableRequestBodyWrite = true
	}

	if opts.Body.Response == BodyRuleRead || opts.Body.Response == BodyRuleReadWrite {
		o.EnableResponseBodyRead = true
	}

	if opts.Body.Response == BodyRuleReadWrite {
		o.EnableResponseBodyWrite = true
	}

	if opts.Merge.OnError == MergeOnErrorFallbackToParent {
		o.IgnoreMergeError = true
	}

	if opts.Attrs.Fetch != nil {
		o.AttrFetch = append([]string(nil), opts.Attrs.Fetch...)
	}

	if opts.DownstreamTLS {
		o.EnableDownstreamTLS = true
	}

	if opts.Body.KeepContentLength == KeepLengthRequest || opts.Body.KeepContentLength == KeepLengthBoth {
		o.DisableChunkedEncodingRequest = true
	}

	if opts.Body.KeepContentLength == KeepLengthResponse || opts.Body.KeepContentLength == KeepLengthBoth {
		o.DisableChunkedEncodingResponse = true
	}

	return o
}

// OptionsFromConfigOptions converts legacy ConfigOptions to Options,
// preserving semantics. Deprecated names map to behavior-identical policies.
func OptionsFromConfigOptions(o ConfigOptions) Options {
	opts := Options{
		Config:        o.FilterConfig,
		MetricsPrefix: o.MetricsPrefix,
		Route: RoutePolicy{
			ReloadOnHeaderChange: o.AutoReloadRoute,
		},
		Merge: MergePolicy{
			ChildWins: o.AlwaysUseChildConfig,
			OnError:   MergeOnErrorPanic,
		},
		Body: BodyPolicy{
			Access:            BodyAccessHeaderGated,
			KeepContentLength: KeepLengthNone,
		},
	}

	if o.DisableStrictBodyAccess {
		opts.Body.Access = BodyAccessOpen
	}

	if o.EnableRequestBodyWrite {
		opts.Body.Request = BodyRuleReadWrite
	} else if o.EnableRequestBodyRead {
		opts.Body.Request = BodyRuleRead
	}

	if o.EnableResponseBodyWrite {
		opts.Body.Response = BodyRuleReadWrite
	} else if o.EnableResponseBodyRead {
		opts.Body.Response = BodyRuleRead
	}

	if o.IgnoreMergeError {
		opts.Merge.OnError = MergeOnErrorFallbackToParent
	}

	if o.AttrFetch != nil {
		opts.Attrs.Fetch = append([]string(nil), o.AttrFetch...)
	}

	if o.EnableDownstreamTLS {
		opts.DownstreamTLS = true
	}

	switch {
	case o.DisableChunkedEncodingRequest && o.DisableChunkedEncodingResponse:
		opts.Body.KeepContentLength = KeepLengthBoth
	case o.DisableChunkedEncodingRequest:
		opts.Body.KeepContentLength = KeepLengthRequest
	case o.DisableChunkedEncodingResponse:
		opts.Body.KeepContentLength = KeepLengthResponse
	}

	return opts
}
