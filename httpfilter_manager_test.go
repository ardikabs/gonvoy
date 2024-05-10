package gonvoy

import (
	"net/http"
	"testing"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
			mockContext.EXPECT().Request().Return(&http.Request{}).Maybe()
			mockContext.EXPECT().Committed().Return(false).Maybe()

			mockStrategy := NewMockHttpFilterPhaseStrategy(t)
			mockStrategy.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything).Run(func(c Context, first, last HttpFilterProcessor) {
				_ = first.HandleOnRequestHeader(c)
			}).Return(ActionPause, nil)

			mgr := &httpFilterManager{
				ctx: mockContext,
			}
			mgr.RegisterHTTPFilterHandler(mockHandlerFirst)
			mgr.RegisterHTTPFilterHandler(mockHandlerSecond)
			mgr.RegisterHTTPFilterHandler(mockHandlerThird)

			status := mgr.ServeHTTPFilter(mockStrategy)
			assert.Equal(t, api.StopAndBuffer, status)
		})

		t.Run("within OnRequestBody use FIFO sequences", func(t *testing.T) {
			firstHandlerOnRequestBody := mockHandlerFirst.EXPECT().OnRequestBody(mock.Anything, mock.Anything).Return(nil)
			secondHandlerOnRequestBody := mockHandlerSecond.EXPECT().OnRequestBody(mock.Anything, mock.Anything).Return(nil)
			thirdHandlerOnRequestBody := mockHandlerThird.EXPECT().OnRequestBody(mock.Anything, mock.Anything).Return(nil)

			secondHandlerOnRequestBody.NotBefore(firstHandlerOnRequestBody.Call)
			thirdHandlerOnRequestBody.NotBefore(firstHandlerOnRequestBody.Call, secondHandlerOnRequestBody.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().RequestBody().Return(&bodyWriter{}).Maybe()
			mockContext.EXPECT().Committed().Return(false).Maybe()

			mockStrategy := NewMockHttpFilterPhaseStrategy(t)
			mockStrategy.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything).Run(func(c Context, first, last HttpFilterProcessor) {
				_ = first.HandleOnRequestBody(c)
			}).Return(ActionPause, nil)

			mgr := &httpFilterManager{
				ctx: mockContext,
			}
			mgr.RegisterHTTPFilterHandler(mockHandlerFirst)
			mgr.RegisterHTTPFilterHandler(mockHandlerSecond)
			mgr.RegisterHTTPFilterHandler(mockHandlerThird)

			status := mgr.ServeHTTPFilter(mockStrategy)
			assert.Equal(t, api.StopAndBuffer, status)
		})

		t.Run("within OnResponseHeader use LIFO sequences", func(t *testing.T) {
			firstHandlerOnResponseHeader := mockHandlerFirst.EXPECT().OnResponseHeader(mock.Anything, mock.Anything).Return(nil)
			secondHandlerOnResponseHeader := mockHandlerSecond.EXPECT().OnResponseHeader(mock.Anything, mock.Anything).Return(nil)
			thirdHandlerOnResponseHeader := mockHandlerThird.EXPECT().OnResponseHeader(mock.Anything, mock.Anything).Return(nil)

			firstHandlerOnResponseHeader.NotBefore(secondHandlerOnResponseHeader.Call, thirdHandlerOnResponseHeader.Call)
			secondHandlerOnResponseHeader.NotBefore(thirdHandlerOnResponseHeader.Call)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().Response().Return(&http.Response{}).Maybe()
			mockContext.EXPECT().Committed().Return(false).Maybe()

			mockStrategy := NewMockHttpFilterPhaseStrategy(t)
			mockStrategy.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything).Run(func(c Context, first, last HttpFilterProcessor) {
				_ = last.HandleOnResponseHeader(c)
			}).Return(ActionPause, nil)

			mgr := &httpFilterManager{
				ctx: mockContext,
			}
			mgr.RegisterHTTPFilterHandler(mockHandlerFirst)
			mgr.RegisterHTTPFilterHandler(mockHandlerSecond)
			mgr.RegisterHTTPFilterHandler(mockHandlerThird)

			status := mgr.ServeHTTPFilter(mockStrategy)
			assert.Equal(t, api.StopAndBuffer, status)
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

			mockStrategy := NewMockHttpFilterPhaseStrategy(t)
			mockStrategy.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything).Run(func(c Context, first, last HttpFilterProcessor) {
				_ = last.HandleOnResponseBody(c)
			}).Return(ActionPause, nil)

			mgr := &httpFilterManager{
				ctx: mockContext,
			}
			mgr.RegisterHTTPFilterHandler(mockHandlerFirst)
			mgr.RegisterHTTPFilterHandler(mockHandlerSecond)
			mgr.RegisterHTTPFilterHandler(mockHandlerThird)

			status := mgr.ServeHTTPFilter(mockStrategy)
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
	})

	t.Run("a disabled handler won't be registered", func(t *testing.T) {
		mgr := &httpFilterManager{}

		mockHandler := NewMockHttpFilterHandler(t)
		mockHandler.EXPECT().Disable().Return(true)

		mgr.RegisterHTTPFilterHandler(mockHandler)
		assert.Nil(t, mgr.first)
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
		mockStrategy := NewMockHttpFilterPhaseStrategy(t)
		mockStrategy.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything).Panic("unexpected action")

		mgr := &httpFilterManager{
			ctx:          mockContext,
			errorHandler: DefaultErrorHandler,
		}

		mgr.RegisterHTTPFilterHandler(PassthroughHttpFilterHandler{})

		status := mgr.ServeHTTPFilter(mockStrategy)
		assert.Equal(t, api.LocalReply, status)
	})

	t.Run("ServeHTTPFilter and got pause action", func(t *testing.T) {
		mockContext := NewMockContext(t)
		mockStrategy := NewMockHttpFilterPhaseStrategy(t)
		mockStrategy.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything).Return(ActionPause, nil)

		mgr := &httpFilterManager{
			ctx:          mockContext,
			errorHandler: DefaultErrorHandler,
		}

		mgr.RegisterHTTPFilterHandler(PassthroughHttpFilterHandler{})

		status := mgr.ServeHTTPFilter(mockStrategy)
		assert.Equal(t, api.StopAndBuffer, status)
	})

	t.Run("ServeHTTPFilter with continue action", func(t *testing.T) {
		mockContext := NewMockContext(t)
		mockContext.EXPECT().StatusType().Return(api.Continue)
		mockStrategy := NewMockHttpFilterPhaseStrategy(t)
		mockStrategy.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything).Return(ActionContinue, nil)

		mgr := &httpFilterManager{
			ctx:          mockContext,
			errorHandler: DefaultErrorHandler,
		}
		mgr.RegisterHTTPFilterHandler(PassthroughHttpFilterHandler{})

		status := mgr.ServeHTTPFilter(mockStrategy)
		assert.Equal(t, api.Continue, status)
	})

	t.Run("ServeHTTPFilter with no handler being registered", func(t *testing.T) {
		mockContext := NewMockContext(t)
		mockStrategy := NewMockHttpFilterPhaseStrategy(t)

		mgr := &httpFilterManager{
			ctx:          mockContext,
			errorHandler: DefaultErrorHandler,
		}

		status := mgr.ServeHTTPFilter(mockStrategy)
		assert.Equal(t, api.Continue, status)
	})
}
