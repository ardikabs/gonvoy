package gonvoy

import "errors"

var (
	// List of errors related to HTTP operations.
	// HTTP reason phrases are intentionally capitalized.
	//
	ErrInternalServer      = errors.New("Internal Server Error") //nolint:staticcheck // HTTP reason phrase
	ErrBadRequest          = errors.New("Bad Request")           //nolint:staticcheck // HTTP reason phrase
	ErrUnauthorized        = errors.New("Unauthorized")
	ErrAccessDenied        = errors.New("Access Denied")         //nolint:staticcheck // HTTP reason phrase
	ErrClientClosedRequest = errors.New("Client Closed Request") //nolint:staticcheck // HTTP reason phrase

	// List of errors related to runtime operations.
	//
	ErrRuntime               = errors.New("an unexpected runtime error occurred")
	ErrOperationNotPermitted = errors.New("operation not permitted")
	ErrIncompatibleReceiver  = errors.New("receiver and value has an incompatible type")
	ErrNilReceiver           = errors.New("receiver shouldn't be nil")
)
