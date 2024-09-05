package gaetway

import "errors"

var (
	// List of errors related to HTTP operations.
	//
	ErrInternalServer      = errors.New("Internal Server Error")
	ErrBadRequest          = errors.New("Bad Request")
	ErrUnauthorized        = errors.New("Unauthorized")
	ErrAccessDenied        = errors.New("Access Denied")
	ErrClientClosedRequest = errors.New("Client Closed Request")

	// List of errors related to runtime operations.
	//
	ErrRuntime               = errors.New("an unexpected runtime error occurred")
	ErrOperationNotPermitted = errors.New("operation not permitted")
	ErrIncompatibleReceiver  = errors.New("receiver and value has an incompatible type")
	ErrNilReceiver           = errors.New("receiver shouldn't be nil")
)
