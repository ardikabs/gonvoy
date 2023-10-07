package errs_test

import (
	"fmt"
	"testing"

	"github.com/ardikabs/go-envoy/pkg/errs"
	"github.com/stretchr/testify/assert"
)

func TestUnwrap(t *testing.T) {
	t.Run("Unwrap should return the given error if there is no inner error", func(t *testing.T) {
		noLvlErr := fmt.Errorf("first layer error")
		err := errs.Unwrap(noLvlErr)
		assert.ErrorIs(t, err, noLvlErr)
	})

	t.Run("Unwrap should return the inner error", func(t *testing.T) {
		wrappedErr := fmt.Errorf("err: this the outer layer of error, %w", errs.ErrInternalServer)
		err := errs.Unwrap(wrappedErr)
		assert.ErrorIs(t, err, errs.ErrInternalServer)
	})

	t.Run("Unwrap should return the inner error, even with multi-level error in it", func(t *testing.T) {
		secondLvlErr := fmt.Errorf("err: this is the second layer of error, %w", errs.ErrInternalServer)
		thirdLvlErr := fmt.Errorf("err: this is the third layer of error, %w", secondLvlErr)
		err := errs.Unwrap(thirdLvlErr)
		assert.ErrorIs(t, err, secondLvlErr)
		assert.ErrorIs(t, err, errs.ErrInternalServer)
	})
}
