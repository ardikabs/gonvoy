package gonvoy

import (
	"net/http"
	"testing"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func fakeSkipPhase() HttpFilterPhaseFunc {
	return func(ctx Context, hfpd HttpFilterPhaseDirector) (HttpFilterAction, error) {
		return ActionSkip, nil
	}
}

func fakeDecodeHeadersPhase() HttpFilterPhaseFunc {
	return func(ctx Context, hfpd HttpFilterPhaseDirector) (HttpFilterAction, error) {
		return ActionContinue, hfpd.decode.HandleOnRequestHeader(ctx)
	}
}

func fakeDecodeDataPhase() HttpFilterPhaseFunc {
	return func(ctx Context, hfpd HttpFilterPhaseDirector) (HttpFilterAction, error) {
		return ActionPause, hfpd.decode.HandleOnRequestBody(ctx)
	}
}

func fakeEncodeHeadersPhase() HttpFilterPhaseFunc {
	return func(ctx Context, hfpd HttpFilterPhaseDirector) (HttpFilterAction, error) {
		return ActionContinue, hfpd.encode.HandleOnResponseHeader(ctx)
	}
}

func fakeEncodeDataPhase() HttpFilterPhaseFunc {
	return func(ctx Context, hfpd HttpFilterPhaseDirector) (HttpFilterAction, error) {
		return ActionPause, hfpd.encode.HandleOnResponseBody(ctx)
	}
}

func TestHttpFilterManager_SetErrorHandler(t *testing.T) {
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
}

func TestHttpFilterManager_RegisterHandler(t *testing.T) {

	t.Run("execution order of all registered handlers should follow based on each phase", func(t *testing.T) {
		mockHandlerFirst := NewMockHttpFilterHandler(t)
		mockHandlerSecond := NewMockHttpFilterHandler(t)
		mockHandlerThird := NewMockHttpFilterHandler(t)

		mockHandlerFirst.EXPECT().Disable().Return(false)
		mockHandlerSecond.EXPECT().Disable().Return(false)
		mockHandlerThird.EXPECT().Disable().Return(false)

		t.Run("within OnRequestHeader use FIFO sequences", func(t *testing.T) {
			firstHandlerOnRequestHeader := mockHandlerFirst.EXPECT().OnRequestHeader(mock.Anything, mock.Anything).Return(nil)
			secondHandlerOnRequestHeader := mockHandlerSecond.EXPECT().OnRequestHeader(mock.Anything, mock.Anything).Return(nil)
			thirdHandlerOnRequestHeader := mockHandlerThird.EXPECT().OnRequestHeader(mock.Anything, mock.Anything).Return(nil)

			secondHandlerOnRequestHeader.NotBefore(firstHandlerOnRequestHeader.Call)
			thirdHandlerOnRequestHeader.NotBefore(firstHandlerOnRequestHeader.Call, secondHandlerOnRequestHeader.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().Request().Return(&http.Request{})
			mockContext.EXPECT().Committed().Return(false)
			mockContext.EXPECT().StatusType().Return(api.Continue)

			mgr := &httpFilterManager{ctx: mockContext}
			mgr.RegisterHTTPFilterHandler(mockHandlerFirst)
			mgr.RegisterHTTPFilterHandler(mockHandlerSecond)
			mgr.RegisterHTTPFilterHandler(mockHandlerThird)

			status := mgr.ServeHTTPFilter(fakeDecodeHeadersPhase())
			assert.Equal(t, api.Continue, status)
		})

		t.Run("within OnRequestBody use FIFO sequences", func(t *testing.T) {
			firstHandlerOnRequestBody := mockHandlerFirst.EXPECT().OnRequestBody(mock.Anything, mock.Anything).Return(nil)
			secondHandlerOnRequestBody := mockHandlerSecond.EXPECT().OnRequestBody(mock.Anything, mock.Anything).Return(nil)
			thirdHandlerOnRequestBody := mockHandlerThird.EXPECT().OnRequestBody(mock.Anything, mock.Anything).Return(nil)

			secondHandlerOnRequestBody.NotBefore(firstHandlerOnRequestBody.Call)
			thirdHandlerOnRequestBody.NotBefore(firstHandlerOnRequestBody.Call, secondHandlerOnRequestBody.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().RequestBody().Return(&bodyWriter{})
			mockContext.EXPECT().Committed().Return(false)

			mgr := &httpFilterManager{ctx: mockContext}
			mgr.RegisterHTTPFilterHandler(mockHandlerFirst)
			mgr.RegisterHTTPFilterHandler(mockHandlerSecond)
			mgr.RegisterHTTPFilterHandler(mockHandlerThird)

			status := mgr.ServeHTTPFilter(fakeDecodeDataPhase())
			assert.Equal(t, api.StopAndBuffer, status)
		})

		t.Run("within OnResponseHeader use LIFO sequences", func(t *testing.T) {
			firstHandlerOnResponseHeader := mockHandlerFirst.EXPECT().OnResponseHeader(mock.Anything, mock.Anything).Return(nil)
			secondHandlerOnResponseHeader := mockHandlerSecond.EXPECT().OnResponseHeader(mock.Anything, mock.Anything).Return(nil)
			thirdHandlerOnResponseHeader := mockHandlerThird.EXPECT().OnResponseHeader(mock.Anything, mock.Anything).Return(nil)

			firstHandlerOnResponseHeader.NotBefore(secondHandlerOnResponseHeader.Call, thirdHandlerOnResponseHeader.Call)
			secondHandlerOnResponseHeader.NotBefore(thirdHandlerOnResponseHeader.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().Response().Return(&http.Response{})
			mockContext.EXPECT().Committed().Return(false)
			mockContext.EXPECT().StatusType().Return(api.Continue)

			mgr := &httpFilterManager{ctx: mockContext}
			mgr.RegisterHTTPFilterHandler(mockHandlerFirst)
			mgr.RegisterHTTPFilterHandler(mockHandlerSecond)
			mgr.RegisterHTTPFilterHandler(mockHandlerThird)

			status := mgr.ServeHTTPFilter(fakeEncodeHeadersPhase())
			assert.Equal(t, api.Continue, status)
		})

		t.Run("within OnResponseBody use LIFO sequences", func(t *testing.T) {
			firstHandlerOnResponseBody := mockHandlerFirst.EXPECT().OnResponseBody(mock.Anything, mock.Anything).Return(nil)
			secondHandlerOnResponseBody := mockHandlerSecond.EXPECT().OnResponseBody(mock.Anything, mock.Anything).Return(nil)
			thirdHandlerOnResponseBody := mockHandlerThird.EXPECT().OnResponseBody(mock.Anything, mock.Anything).Return(nil)

			firstHandlerOnResponseBody.NotBefore(secondHandlerOnResponseBody.Call, thirdHandlerOnResponseBody.Call)
			secondHandlerOnResponseBody.NotBefore(thirdHandlerOnResponseBody.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().ResponseBody().Return(&bodyWriter{}).Maybe()
			mockContext.EXPECT().Committed().Return(false).Maybe()

			mgr := &httpFilterManager{ctx: mockContext}
			mgr.RegisterHTTPFilterHandler(mockHandlerFirst)
			mgr.RegisterHTTPFilterHandler(mockHandlerSecond)
			mgr.RegisterHTTPFilterHandler(mockHandlerThird)

			status := mgr.ServeHTTPFilter(fakeEncodeDataPhase())
			assert.Equal(t, api.StopAndBuffer, status)
		})
	})

	t.Run("a nil handler won't be registered", func(t *testing.T) {
		mgr := &httpFilterManager{}
		createBadHandlerFn := func() *PassthroughHttpFilterHandler {
			return nil
		}

		mgr.RegisterHTTPFilterHandler(createBadHandlerFn())
		assert.Nil(t, mgr.first)
		assert.Nil(t, mgr.last)
	})

	t.Run("a disabled handler won't be registered", func(t *testing.T) {
		mgr := &httpFilterManager{}

		mockHandler := NewMockHttpFilterHandler(t)
		mockHandler.EXPECT().Disable().Return(true)

		mgr.RegisterHTTPFilterHandler(mockHandler)
		assert.Nil(t, mgr.first)
		assert.Nil(t, mgr.last)
	})
}

func TestHttpFilterManager_ServeHTTPFilter(t *testing.T) {

	t.Run("ServeHTTPFilter and catch a panic", func(t *testing.T) {
		mockContext := NewMockContext(t)
		mockContext.EXPECT().Log().Return(logr.Logger{})
		mockContext.EXPECT().GetProperty(mock.Anything, mock.Anything).Return("", nil)
		mockContext.EXPECT().JSON(
			mock.MatchedBy(func(code int) bool {
				return assert.Equal(t, http.StatusInternalServerError, code)
			}),
			mock.MatchedBy(func(body []byte) bool {
				return assert.Equal(t, ResponseInternalServerError, body)
			}),
			mock.Anything,
			mock.Anything,
		).Return(nil)

		mgr := &httpFilterManager{
			ctx:          mockContext,
			errorHandler: DefaultErrorHandler,
		}

		mgr.RegisterHTTPFilterHandler(PassthroughHttpFilterHandler{})

		status := mgr.ServeHTTPFilter(func(ctx Context, hfpd HttpFilterPhaseDirector) (HttpFilterAction, error) {
			panic("phase on panic")
		})
		assert.Equal(t, api.LocalReply, status)
	})

	t.Run("ServeHTTPFilter with skip action", func(t *testing.T) {
		mockContext := NewMockContext(t)
		mgr := &httpFilterManager{
			ctx:          mockContext,
			errorHandler: DefaultErrorHandler,
		}
		mgr.RegisterHTTPFilterHandler(PassthroughHttpFilterHandler{})

		status := mgr.ServeHTTPFilter(fakeSkipPhase())
		assert.Equal(t, api.Continue, status)
	})

	t.Run("ServeHTTPFilter with no handler being registered", func(t *testing.T) {
		mockContext := NewMockContext(t)

		mgr := &httpFilterManager{
			ctx:          mockContext,
			errorHandler: DefaultErrorHandler,
		}

		status := mgr.ServeHTTPFilter(fakeSkipPhase())
		assert.Equal(t, api.Continue, status)
	})
}
