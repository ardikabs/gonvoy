package gonvoy

// HttpFilterHandler represents an interface for an HTTP filter handler.
// In a typical HTTP flow, the sequence of events can be as follows:
// OnRequestHeader -> OnRequestBody -> <Any number of intermediate Envoy processes> -> OnResponseHeader -> OnResponseBody
// HttpFilterHandler is an interface that defines the methods to handle HTTP filter operations.
type HttpFilterHandler interface {
	// Disable disables the HTTP filter handler.
	//
	// It returns a boolean value indicating whether the HTTP filter handler is disabled.
	Disable() bool

	// OnRequestHeader is called when processing the HTTP request header during the OnRequestBody phase.
	//
	OnRequestHeader(c Context) error

	// OnRequestBody is called when processing the HTTP request body during the OnRequestBody phase.
	//
	OnRequestBody(c Context) error

	// OnResponseHeader is called when processing the HTTP response header during the OnResponseHeader phase.
	//
	OnResponseHeader(c Context) error

	// OnResponseBody is called when processing the HTTP response body during the OnResponseBody phase.
	//
	OnResponseBody(c Context) error
}

var _ HttpFilterHandler = PassthroughHttpFilterHandler{}

type PassthroughHttpFilterHandler struct{}

func (PassthroughHttpFilterHandler) Disable() bool                    { return false }
func (PassthroughHttpFilterHandler) OnRequestHeader(c Context) error  { return nil }
func (PassthroughHttpFilterHandler) OnRequestBody(c Context) error    { return nil }
func (PassthroughHttpFilterHandler) OnResponseHeader(c Context) error { return nil }
func (PassthroughHttpFilterHandler) OnResponseBody(c Context) error   { return nil }
