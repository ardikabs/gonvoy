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

func TestInstance_SetErrorHandler(t *testing.T) {
	t.Run("nil error handler will be ignored", func(t *testing.T) {
		ins := &Instance{}

		ins.SetErrorHandler(nil)
		assert.Nil(t, ins.errorHandler)
	})

	t.Run("a custom error handler will be used", func(t *testing.T) {
		mgr := &Instance{}

		mgr.SetErrorHandler(DefaultErrorHandler)
		assert.NotNil(t, mgr.errorHandler)
	})
}

func TestInstance_AddHandlerAndServe(t *testing.T) {

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

			ins := &Instance{}
			ins.AddHandler(mockHandlerFirst)
			ins.AddHandler(mockHandlerSecond)
			ins.AddHandler(mockHandlerThird)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().Request().Return(&http.Request{})
			mockContext.EXPECT().Committed().Return(false)
			mockContext.EXPECT().StatusType().Return(api.Continue)

			server := &httpFilterServer{
				ctx:          mockContext,
				errorHandler: ins.errorHandler,
				decoder:      ins.first,
				encoder:      ins.last,
			}
			res := server.ServeDecodeFilter(fakeDecodeHeadersPhase())
			assert.Equal(t, api.Continue, res.Status)
		})

		t.Run("within OnRequestBody use FIFO sequences", func(t *testing.T) {
			firstHandlerOnRequestBody := mockHandlerFirst.EXPECT().OnRequestBody(mock.Anything, mock.Anything).Return(nil)
			secondHandlerOnRequestBody := mockHandlerSecond.EXPECT().OnRequestBody(mock.Anything, mock.Anything).Return(nil)
			thirdHandlerOnRequestBody := mockHandlerThird.EXPECT().OnRequestBody(mock.Anything, mock.Anything).Return(nil)

			secondHandlerOnRequestBody.NotBefore(firstHandlerOnRequestBody.Call)
			thirdHandlerOnRequestBody.NotBefore(firstHandlerOnRequestBody.Call, secondHandlerOnRequestBody.Call)

			ins := &Instance{}
			ins.AddHandler(mockHandlerFirst)
			ins.AddHandler(mockHandlerSecond)
			ins.AddHandler(mockHandlerThird)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().RequestBody().Return(&bodyWriter{})
			mockContext.EXPECT().Committed().Return(false)

			server := &httpFilterServer{
				ctx:          mockContext,
				errorHandler: ins.errorHandler,
				decoder:      ins.first,
				encoder:      ins.last,
			}
			res := server.ServeDecodeFilter(fakeDecodeDataPhase())
			assert.Equal(t, api.StopAndBuffer, res.Status)
		})

		t.Run("within OnResponseHeader use LIFO sequences", func(t *testing.T) {
			firstHandlerOnResponseHeader := mockHandlerFirst.EXPECT().OnResponseHeader(mock.Anything, mock.Anything).Return(nil)
			secondHandlerOnResponseHeader := mockHandlerSecond.EXPECT().OnResponseHeader(mock.Anything, mock.Anything).Return(nil)
			thirdHandlerOnResponseHeader := mockHandlerThird.EXPECT().OnResponseHeader(mock.Anything, mock.Anything).Return(nil)

			firstHandlerOnResponseHeader.NotBefore(secondHandlerOnResponseHeader.Call, thirdHandlerOnResponseHeader.Call)
			secondHandlerOnResponseHeader.NotBefore(thirdHandlerOnResponseHeader.Call)

			ins := &Instance{}
			ins.AddHandler(mockHandlerFirst)
			ins.AddHandler(mockHandlerSecond)
			ins.AddHandler(mockHandlerThird)

			mockContext := NewMockContext(t)
			mockContext.EXPECT().Response().Return(&http.Response{})
			mockContext.EXPECT().Committed().Return(false)
			mockContext.EXPECT().StatusType().Return(api.Continue)

			server := &httpFilterServer{
				ctx:          mockContext,
				errorHandler: ins.errorHandler,
				decoder:      ins.first,
				encoder:      ins.last,
			}
			res := server.ServeEncodeFilter(fakeEncodeHeadersPhase())
			assert.Equal(t, api.Continue, res.Status)
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

			ins := &Instance{}
			ins.AddHandler(mockHandlerFirst)
			ins.AddHandler(mockHandlerSecond)
			ins.AddHandler(mockHandlerThird)

			server := &httpFilterServer{
				ctx:          mockContext,
				errorHandler: ins.errorHandler,
				decoder:      ins.first,
				encoder:      ins.last,
			}
			res := server.ServeEncodeFilter(fakeEncodeDataPhase())
			assert.Equal(t, api.StopAndBuffer, res.Status)
		})
	})

	t.Run("a nil handler won't be registered", func(t *testing.T) {
		ins := &Instance{}
		createBadHandlerFn := func() *PassthroughHttpFilterHandler {
			return nil
		}

		ins.AddHandler(createBadHandlerFn())
		assert.Nil(t, ins.first)
		assert.Nil(t, ins.last)
	})

	t.Run("a disabled handler won't be registered", func(t *testing.T) {
		ins := &Instance{}

		mockHandler := NewMockHttpFilterHandler(t)
		mockHandler.EXPECT().Disable().Return(true)

		ins.AddHandler(mockHandler)
		assert.Nil(t, ins.first)
		assert.Nil(t, ins.last)
	})

	t.Run("Serve and catch a panic", func(t *testing.T) {
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

		ins := newInstance()
		ins.AddHandler(PassthroughHttpFilterHandler{})

		server := &httpFilterServer{
			ctx:          mockContext,
			errorHandler: ins.errorHandler,
			decoder:      ins.first,
			encoder:      ins.last,
		}

		decodeResult := server.ServeDecodeFilter(func(Context, HttpFilterDecodeProcessor) (HttpFilterAction, error) {
			panic("phase on panic")
		})
		assert.Equal(t, api.LocalReply, decodeResult.Status)

		encodeResult := server.ServeEncodeFilter(func(Context, HttpFilterEncodeProcessor) (HttpFilterAction, error) {
			panic("phase on panic")
		})
		assert.Equal(t, api.LocalReply, encodeResult.Status)
	})

	t.Run("Serve with skip action", func(t *testing.T) {
		mockContext := NewMockContext(t)

		ins := newInstance()
		ins.AddHandler(PassthroughHttpFilterHandler{})

		server := &httpFilterServer{
			ctx:          mockContext,
			errorHandler: ins.errorHandler,
			decoder:      ins.first,
			encoder:      ins.last,
		}
		assert.Equal(t, api.Continue, server.ServeDecodeFilter(fakeSkipDecodePhase()).Status)
		assert.Equal(t, api.Continue, server.ServeEncodeFilter(fakeSkipEncodePhase()).Status)
	})

	t.Run("Serve with no handler being registered", func(t *testing.T) {
		mockContext := NewMockContext(t)

		ins := newInstance()

		server := &httpFilterServer{
			ctx:          mockContext,
			errorHandler: ins.errorHandler,
			decoder:      ins.first,
			encoder:      ins.last,
		}
		assert.Equal(t, api.Continue, server.ServeDecodeFilter(fakeSkipDecodePhase()).Status)
		assert.Equal(t, api.Continue, server.ServeEncodeFilter(fakeSkipEncodePhase()).Status)
	})
}
