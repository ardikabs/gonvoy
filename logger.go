package envoy

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
	"github.com/rs/zerolog"
)

type logWriter struct {
	mu  sync.Mutex
	buf *bytes.Buffer
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.Write(p)
}

func (w *logWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := w.buf.String()
	w.buf.Reset()
	return strings.TrimSuffix(out, "\n")
}

type logSink struct {
	callback api.FilterCallbacks

	logger    *zerolog.Logger
	logWriter *logWriter

	name  string
	depth int
}

func NewLogger(callback api.FilterCallbacks) logr.Logger {
	out := &logWriter{buf: &bytes.Buffer{}}

	writer := zerolog.ConsoleWriter{
		Out:          out,
		NoColor:      true,
		PartsExclude: []string{"time"},
		FormatLevel: func(i interface{}) string {
			return ""
		},
		FormatMessage: func(i interface{}) string {
			return strings.TrimSuffix(i.(string), "\n")
		},
	}

	logger := zerolog.New(writer).Level(zerolog.NoLevel).With().Caller().Stack().Logger()
	return logr.New(&logSink{
		callback:  callback,
		logWriter: out,
		logger:    &logger,
	})
}

var _ logr.LogSink = &logSink{}

func (ls *logSink) Init(ri logr.RuntimeInfo) {
	ls.depth = ri.CallDepth + 2
}

func (l *logSink) Enabled(i int) bool {
	return true
}

func (l *logSink) Error(err error, msg string, keysAndValues ...interface{}) {
	e := l.logger.Log()

	if err != nil {
		e = e.Str("error", err.Error())
	}

	l.msg(api.Error, e, msg, keysAndValues)
}

func (l *logSink) Info(level int, msg string, keysAndValues ...interface{}) {
	e := l.logger.Log()
	l.msg(lvlToLogType(level), e, msg, keysAndValues)
}

func (l *logSink) msg(level api.LogType, e *zerolog.Event, msg string, keysAndValues []interface{}) {
	if e == nil {
		return
	}
	if l.name != "" {
		e.Str("logger", l.name)
	}

	e.Fields(DefaultRender(keysAndValues)).
		CallerSkipFrame(l.depth).
		Msg(msg)

	l.callback.Log(level, l.logWriter.String())
}

func (l logSink) WithName(name string) logr.LogSink {
	if l.name != "" {
		l.name += "/" + name
	} else {
		l.name = name
	}
	return &l
}

func (l logSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	zl := l.logger.With().Fields(DefaultRender(keysAndValues)).Logger()
	l.logger = &zl
	return &l
}

func (l logSink) WithCallDepth(depth int) logr.LogSink {
	l.depth += depth
	return &l
}

func DefaultRender(keysAndValues []interface{}) []interface{} {
	for i, n := 1, len(keysAndValues); i < n; i += 2 {
		value := keysAndValues[i]
		switch v := value.(type) {
		case logr.Marshaler:
			keysAndValues[i] = v.MarshalLog()
		case fmt.Stringer:
			keysAndValues[i] = v.String()
		}
	}
	return keysAndValues
}

func lvlToLogType(lvl int) api.LogType {
	if lvl <= -2 {
		return api.Critical
	}

	switch lvl {
	case -1:
		return api.Warn
	case 0:
		return api.Info
	default:
		return api.Debug
	}
}
