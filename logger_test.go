package gonvoy

import (
	"errors"
	"strings"
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/stretchr/testify/mock"
)

func TestLogger(t *testing.T) {
	logMsg0 := "debug-log"
	logMsg1 := "info-log"
	logMsg2 := "error-log"

	mockFilterCallback := mock_envoy.NewFilterCallbackHandler(t)
	mockFilterCallback.EXPECT().Log(mock.MatchedBy(func(l api.LogType) bool {
		return l == api.Debug
	}), mock.MatchedBy(func(msg string) bool {
		return strings.Contains(msg, logMsg0)
	}))
	mockFilterCallback.EXPECT().Log(mock.MatchedBy(func(l api.LogType) bool {
		return l == api.Info
	}), mock.MatchedBy(func(msg string) bool {
		return strings.Contains(msg, logMsg1)
	}))
	mockFilterCallback.EXPECT().Log(mock.MatchedBy(func(l api.LogType) bool {
		return l == api.Error
	}), mock.MatchedBy(func(msg string) bool {
		return strings.Contains(msg, logMsg2)
	}))

	logger := newLogger(mockFilterCallback)
	logger.V(1).Info(logMsg0)
	logger.Info(logMsg1)
	logger.Error(errors.New("error1"), logMsg2)
}

func TestLogger_WithName(t *testing.T) {
	mockFilterCallback := mock_envoy.NewFilterCallbackHandler(t)
	logger := newLogger(mockFilterCallback)

	mockFilterCallback.EXPECT().Log(mock.MatchedBy(func(l api.LogType) bool {
		return l == api.Info
	}), mock.MatchedBy(func(msg string) bool {
		if !strings.Contains(msg, "logger=app1") {
			return false
		}

		if !strings.Contains(msg, "foo=bar") {
			return false
		}

		if !strings.Contains(msg, "fii=123") {
			return false
		}

		return true
	}))
	app1Logger := logger.WithName("app1").WithValues("foo", "bar", "fii", 123).WithCallDepth(5)
	app1Logger.Info("foo-msg")

	mockFilterCallback.EXPECT().Log(mock.MatchedBy(func(l api.LogType) bool {
		return l == api.Info
	}), mock.MatchedBy(func(msg string) bool {
		if !strings.Contains(msg, "logger=app1/app2") {
			return false
		}

		if !strings.Contains(msg, "foo=bar") {
			return false
		}

		if !strings.Contains(msg, "fii=123") {
			return false
		}

		return true
	}))
	app2Logger := app1Logger.WithName("app2")
	app2Logger.Info("foo2-msg", "foo", "bar", "fii", 123)
}
