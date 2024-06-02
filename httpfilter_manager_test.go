package gonvoy

import (
	"net/http"
	"testing"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func fakeSkipDecodePhase() HttpFilterDecoderFunc {
	return func(ctx Context, p HttpFilterDecodeProcessor) (HttpFilterAction, error) {
		return ActionSkip, nil
	}
}

func fakeSkipEncodePhase() HttpFilterEncoderFunc {
	return func(ctx Context, p HttpFilterEncodeProcessor) (HttpFilterAction, error) {
		return ActionSkip, nil
	}
}

func fakeDecodeHeadersPhase() HttpFilterDecoderFunc {
	return func(ctx Context, p HttpFilterDecodeProcessor) (HttpFilterAction, error) {
		return ActionContinue, p.HandleOnRequestHeader(ctx)
	}
}

func fakeDecodeDataPhase() HttpFilterDecoderFunc {
	return func(ctx Context, p HttpFilterDecodeProcessor) (HttpFilterAction, error) {
		return ActionPause, p.HandleOnRequestBody(ctx)
	}
}

func fakeEncodeHeadersPhase() HttpFilterEncoderFunc {
	return func(ctx Context, p HttpFilterEncodeProcessor) (HttpFilterAction, error) {
		return ActionContinue, p.HandleOnResponseHeader(ctx)
	}
}

func fakeEncodeDataPhase() HttpFilterEncoderFunc {
	return func(ctx Context, p HttpFilterEncodeProcessor) (HttpFilterAction, error) {
		return ActionPause, p.HandleOnResponseBody(ctx)
	}
}

func TestHttpFilterManager(t *testing.T) {

	t.Run("set custom error handler", func(t *testing.T) {
		t.Run("nil error handler will be ignored", func(t *testing.T) {
			mgr := &httpFilterManager{}

			mgr.SetErrorHandler(nil)
			assert.Nil(t, mgr.errorHandler)
		})

		t.Run("a custom error handler will be used", func(t *testing.T) {
			mgr := &httpFilterManager{}

			mgr.SetErrorHandler(DefaultErrorHandler)
			assert.NotNil(t, mgr.errorHandler)
		})
	})

	t.Run("execution order of all registered handlers should follow based on each phase", func(t *testing.T) {
		mockHandlerFirst := NewMockHttpFilterHandler(t)
		mockHandlerSecond := NewMockHttpFilterHandler(t)
		mockHandlerThird := NewMockHttpFilterHandler(t)

		mockHandlerFirst.EXPECT().Disable().Return(false)
		mockHandlerSecond.EXPECT().Disable().Return(false)
		mockHandlerThird.EXPECT().Disable().Return(false)

		t.Run("within OnRequestHeader use FIFO sequences", func(t *testing.T) {
			firstHandlerOnRequestHeader := mockHandlerFirst.EXPECT().OnRequestHeader(mock.Anything).Return(nil)
			secondHandlerOnRequestHeader := mockHandlerSecond.EXPECT().OnRequestHeader(mock.Anything).Return(nil)
			thirdHandlerOnRequestHeader := mockHandlerThird.EXPECT().OnRequestHeader(mock.Anything).Return(nil)

			secondHandlerOnRequestHeader.NotBefore(firstHandlerOnRequestHeader.Call)
			thirdHandlerOnRequestHeader.NotBefore(firstHandlerOnRequestHeader.Call, secondHandlerOnRequestHeader.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().Committed().Return(false)
			mockContext.EXPECT().StatusType().Return(api.Continue)

			mgr := newHttpFilterManager(mockContext)
			mgr.AddHandler(mockHandlerFirst)
			mgr.AddHandler(mockHandlerSecond)
			mgr.AddHandler(mockHandlerThird)

			res := mgr.ServeDecodeFilter(fakeDecodeHeadersPhase())
			assert.Equal(t, api.Continue, res.Status)
		})

		t.Run("within OnRequestBody use FIFO sequences", func(t *testing.T) {
			firstHandlerOnRequestBody := mockHandlerFirst.EXPECT().OnRequestBody(mock.Anything).Return(nil)
			secondHandlerOnRequestBody := mockHandlerSecond.EXPECT().OnRequestBody(mock.Anything).Return(nil)
			thirdHandlerOnRequestBody := mockHandlerThird.EXPECT().OnRequestBody(mock.Anything).Return(nil)

			secondHandlerOnRequestBody.NotBefore(firstHandlerOnRequestBody.Call)
			thirdHandlerOnRequestBody.NotBefore(firstHandlerOnRequestBody.Call, secondHandlerOnRequestBody.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().Committed().Return(false)

			mgr := newHttpFilterManager(mockContext)
			mgr.AddHandler(mockHandlerFirst)
			mgr.AddHandler(mockHandlerSecond)
			mgr.AddHandler(mockHandlerThird)

			res := mgr.ServeDecodeFilter(fakeDecodeDataPhase())
			assert.Equal(t, api.StopAndBuffer, res.Status)
		})

		t.Run("within OnResponseHeader use LIFO sequences", func(t *testing.T) {
			firstHandlerOnResponseHeader := mockHandlerFirst.EXPECT().OnResponseHeader(mock.Anything).Return(nil)
			secondHandlerOnResponseHeader := mockHandlerSecond.EXPECT().OnResponseHeader(mock.Anything).Return(nil)
			thirdHandlerOnResponseHeader := mockHandlerThird.EXPECT().OnResponseHeader(mock.Anything).Return(nil)

			firstHandlerOnResponseHeader.NotBefore(secondHandlerOnResponseHeader.Call, thirdHandlerOnResponseHeader.Call)
			secondHandlerOnResponseHeader.NotBefore(thirdHandlerOnResponseHeader.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().Committed().Return(false)
			mockContext.EXPECT().StatusType().Return(api.Continue)

			mgr := newHttpFilterManager(mockContext)
			mgr.AddHandler(mockHandlerFirst)
			mgr.AddHandler(mockHandlerSecond)
			mgr.AddHandler(mockHandlerThird)

			res := mgr.ServeEncodeFilter(fakeEncodeHeadersPhase())
			assert.Equal(t, api.Continue, res.Status)
		})

		t.Run("within OnResponseBody use LIFO sequences", func(t *testing.T) {
			firstHandlerOnResponseBody := mockHandlerFirst.EXPECT().OnResponseBody(mock.Anything).Return(nil)
			secondHandlerOnResponseBody := mockHandlerSecond.EXPECT().OnResponseBody(mock.Anything).Return(nil)
			thirdHandlerOnResponseBody := mockHandlerThird.EXPECT().OnResponseBody(mock.Anything).Return(nil)

			firstHandlerOnResponseBody.NotBefore(secondHandlerOnResponseBody.Call, thirdHandlerOnResponseBody.Call)
			secondHandlerOnResponseBody.NotBefore(thirdHandlerOnResponseBody.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().Committed().Return(false).Maybe()

			mgr := newHttpFilterManager(mockContext)
			mgr.AddHandler(mockHandlerFirst)
			mgr.AddHandler(mockHandlerSecond)
			mgr.AddHandler(mockHandlerThird)

			res := mgr.ServeEncodeFilter(fakeEncodeDataPhase())
			assert.Equal(t, api.StopAndBuffer, res.Status)
		})
	})

	t.Run("a nil handler won't be registered", func(t *testing.T) {
		createBadHandlerFn := func() *PassthroughHttpFilterHandler {
			return nil
		}

		mgr := &httpFilterManager{}
		mgr.AddHandler(createBadHandlerFn())

		assert.Nil(t, mgr.first)
		assert.Nil(t, mgr.last)
	})

	t.Run("a disabled handler won't be registered", func(t *testing.T) {

		mockHandler := NewMockHttpFilterHandler(t)
		mockHandler.EXPECT().Disable().Return(true)
		mgr := &httpFilterManager{}
		mgr.AddHandler(mockHandler)

		assert.Nil(t, mgr.first)
		assert.Nil(t, mgr.last)
	})

	t.Run("Serve and catch a panic", func(t *testing.T) {
		mockContext := NewMockContext(t)
		mockContext.EXPECT().Log().Return(logr.Logger{})
		mockContext.EXPECT().GetProperty(mock.Anything, mock.Anything).Return("", nil)
		mockContext.EXPECT().JSON(
			mock.MatchedBy(func(code int) bool {
				return assert.Equal(t, http.StatusInternalServerError, code)
			}),
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil)

		mgr := newHttpFilterManager(mockContext)
		mgr.AddHandler(PassthroughHttpFilterHandler{})

		decodeResult := mgr.ServeDecodeFilter(func(Context, HttpFilterDecodeProcessor) (HttpFilterAction, error) {
			panic("phase on panic")
		})
		assert.Equal(t, api.LocalReply, decodeResult.Status)

		encodeResult := mgr.ServeEncodeFilter(func(Context, HttpFilterEncodeProcessor) (HttpFilterAction, error) {
			panic("phase on panic")
		})
		assert.Equal(t, api.LocalReply, encodeResult.Status)
	})

	t.Run("Serve with skip action", func(t *testing.T) {
		mockContext := NewMockContext(t)

		mgr := newHttpFilterManager(mockContext)
		mgr.AddHandler(PassthroughHttpFilterHandler{})

		assert.Equal(t, api.Continue, mgr.ServeDecodeFilter(fakeSkipDecodePhase()).Status)
		assert.Equal(t, api.Continue, mgr.ServeEncodeFilter(fakeSkipEncodePhase()).Status)
	})

	t.Run("Serve with no handler being registered", func(t *testing.T) {
		mockContext := NewMockContext(t)

		mgr := newHttpFilterManager(mockContext)

		assert.Equal(t, api.Continue, mgr.ServeDecodeFilter(fakeSkipDecodePhase()).Status)
		assert.Equal(t, api.Continue, mgr.ServeEncodeFilter(fakeSkipEncodePhase()).Status)
	})

	t.Run("trigger finalize", func(t *testing.T) {
		mockContext := NewMockContext(t)
		mockContext.EXPECT().Request().Return(&http.Request{})
		mgr := newHttpFilterManager(mockContext)
		mgr.completer = func() {
			func(c Context) {
				assert.Empty(t, c.Request().Header)
			}(mockContext)
		}

		mgr.Finalize()
	})
}
