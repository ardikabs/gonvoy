package errs

import "errors"

var (
	ErrInternalServer    = errors.New("Internal Server Error")
	ErrInvalidDataFormat = errors.New("Invalid Data Format")
	ErrBadRequest        = errors.New("Bad Request")
	ErrUnauthorized      = errors.New("Unauthorized")
	ErrAccessDenied      = errors.New("RBAC: Access Denied")

	ErrPanic = errors.New("panic occurred")
	ErrNil   = errors.New("value is nil")

	ErrOperationNotPermitted = errors.New("operation not permitted")
	ErrIncompatibleReceiver  = errors.New("receiver and value has an incompatible type")
	ErrNilReceiver           = errors.New("receiver shouldn't be nil")
)

func Unwrap(err error) error {
	if unwrapErr := errors.Unwrap(err); unwrapErr != nil {
		return unwrapErr
	}

	return err
}
