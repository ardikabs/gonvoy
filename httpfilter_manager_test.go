package gonvoy

import (
	"net/http"
	"testing"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHttpFilterHandlerManager_SetErrorHandler(t *testing.T) {
	t.Run("nil error handler will be ignored", func(t *testing.T) {
		mgr := &httpFilterHandlerManager{}

		mgr.SetErrorHandler(nil)
		assert.Nil(t, mgr.errorHandler)
	})

	t.Run("non-nil error handler will be used", func(t *testing.T) {
		mgr := &httpFilterHandlerManager{}

		mgr.SetErrorHandler(DefaultHttpFilterErrorHandler)
		assert.NotNil(t, mgr.errorHandler)
	})
}

func TestHttpFilterHandlerManager_RegisterHandler(t *testing.T) {

	t.Run("all registered handler must be followed based on order, FIFO", func(t *testing.T) {
		mgr := &httpFilterHandlerManager{}

		mockHandlerFirst := NewMockHttpFilterHandler(t)
		mockHandlerFirst.EXPECT().Disable().Return(false)
		mockHandlerSecond := NewMockHttpFilterHandler(t)
		mockHandlerSecond.EXPECT().Disable().Return(false)
		mockHandlerThird := NewMockHttpFilterHandler(t)
		mockHandlerThird.EXPECT().Disable().Return(false)

		mgr.RegisterHandler(mockHandlerFirst)
		mgr.RegisterHandler(mockHandlerSecond)
		mgr.RegisterHandler(mockHandlerThird)

		assert.Equal(t, mockHandlerFirst, (mgr.entrypoint).(*defaultHttpFilterProcessor).handler)
		assert.Equal(t, mockHandlerThird, (mgr.last).(*defaultHttpFilterProcessor).handler)
	})

	t.Run("a nil handler won't be registered", func(t *testing.T) {
		mgr := &httpFilterHandlerManager{}
		createBadHandlerFn := func() *PassthroughHttpFilterHandler {
			return nil
		}

		mgr.RegisterHandler(createBadHandlerFn())
		assert.Nil(t, mgr.entrypoint)
	})

	t.Run("a disabled handler won't be registered", func(t *testing.T) {
		mgr := &httpFilterHandlerManager{}

		mockHandler := NewMockHttpFilterHandler(t)
		mockHandler.EXPECT().Disable().Return(true)

		mgr.RegisterHandler(mockHandler)
		assert.Nil(t, mgr.entrypoint)
	})
}

func TestHttpFilterHandlerManager_Serve(t *testing.T) {

	t.Run("serve and catch a panic", func(t *testing.T) {
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
		mockCtrl := NewMockHttpFilterPhaseController(t)
		mockCtrl.EXPECT().Handle(mock.Anything, mock.Anything).Panic("unexpected action")

		mgr := &httpFilterHandlerManager{
			errorHandler: DefaultHttpFilterErrorHandler,
		}

		status := mgr.Serve(mockContext, mockCtrl)
		assert.Equal(t, api.LocalReply, status)
	})

	t.Run("serve and got pause action", func(t *testing.T) {
		mockContext := NewMockContext(t)
		mockCtrl := NewMockHttpFilterPhaseController(t)
		mockCtrl.EXPECT().Handle(mock.Anything, mock.Anything).Return(ActionPause, nil)

		mgr := &httpFilterHandlerManager{
			errorHandler: DefaultHttpFilterErrorHandler,
		}

		status := mgr.Serve(mockContext, mockCtrl)
		assert.Equal(t, api.StopAndBuffer, status)
	})

	t.Run("serve with continue action", func(t *testing.T) {
		mockContext := NewMockContext(t)
		mockContext.EXPECT().StatusType().Return(api.Continue)
		mockCtrl := NewMockHttpFilterPhaseController(t)
		mockCtrl.EXPECT().Handle(mock.Anything, mock.Anything).Return(ActionContinue, nil)

		mgr := &httpFilterHandlerManager{
			errorHandler: DefaultHttpFilterErrorHandler,
		}

		status := mgr.Serve(mockContext, mockCtrl)
		assert.Equal(t, api.Continue, status)
	})
}
