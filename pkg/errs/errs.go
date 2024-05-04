package errs

import "errors"

var (
	ErrInternalServer = errors.New("Internal Server Error")
	ErrBadRequest     = errors.New("Bad Request")
	ErrUnauthorized   = errors.New("Unauthorized")
	ErrAccessDenied   = errors.New("RBAC: Access Denied")

	ErrPanic = errors.New("panic occurred")
	ErrNil   = errors.New("value is nil")
)

func Unwrap(err error) error {
	if unwrapErr := errors.Unwrap(err); unwrapErr != nil {
		return unwrapErr
	}

	return err
}
