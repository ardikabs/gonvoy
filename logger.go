package gaetway

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
	"github.com/rs/zerolog"
)

// logWriter is a custom implementation of io.Writer that writes log messages to a buffer.
type logWriter struct {
	mu  sync.Mutex
	buf *bytes.Buffer
}

// Write writes the log message to the buffer.
func (w *logWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err = w.buf.Write(p)
	if err != nil {
		return n, fmt.Errorf("failed to write log, %w", err)
	}

	return n, nil
}

// String returns the contents of the buffer as a string and resets the buffer.
func (w *logWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := w.buf.String()
	w.buf.Reset()
	return strings.TrimSuffix(out, "\n")
}

var _ logr.LogSink = &logSink{}

// logSink is a logr.LogSink implementation that sends log messages to the Envoy context via FilterCallbacks.
type logSink struct {
	callback api.FilterCallbacks

	logger    *zerolog.Logger
	logWriter *logWriter

	name  string
	depth int
}

// newLogger creates a new logr.Logger implementation for Gaetway.
func newLogger(callback api.FilterCallbacks) logr.Logger {
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

	logger := zerolog.New(writer).Level(zerolog.InfoLevel).With().Caller().Stack().Logger()
	logSink := &logSink{
		callback:  callback,
		logWriter: out,
		logger:    &logger,
	}

	return logr.New(logSink)
}

// Init receives runtime info about the logr library.
func (ls *logSink) Init(ri logr.RuntimeInfo) {
	ls.depth = ri.CallDepth + 2
}

// Enabled tests whether this LogSink is enabled at the specified V-level.
func (ls *logSink) Enabled(i int) bool {
	return true
}

// Error logs an error, with the given message and key/value pairs as context.
func (ls *logSink) Error(err error, msg string, keysAndValues ...interface{}) {
	e := ls.logger.Error().Err(err)
	ls.msg(api.Error, e, msg, keysAndValues)
}

// Info logs a non-error message at specified V-level with the given key/value pairs as context.
func (ls *logSink) Info(level int, msg string, keysAndValues ...interface{}) {
	e := ls.logger.Info().Int("v", level)
	ls.msg(levelToLogType(level), e, msg, keysAndValues)
}

// msg is a helper function that adds log fields, caller information, and sends the log message to the callback.
func (ls *logSink) msg(level api.LogType, e *zerolog.Event, msg string, keysAndValues []interface{}) {
	if e == nil {
		return
	}
	if ls.name != "" {
		e.Str("logger", ls.name)
	}

	e.Fields(DefaultRender(keysAndValues))
	e.CallerSkipFrame(ls.depth)
	e.Msg(msg)

	ls.callback.Log(level, ls.logWriter.String())
}

// WithName returns a new LogSink with the specified name appended, it splits with "/".
func (ls logSink) WithName(name string) logr.LogSink {
	if ls.name != "" {
		ls.name += "/" + name
	} else {
		ls.name = name
	}
	return &ls
}

// WithValues returns a new LogSink with additional key/value pairs.
func (ls logSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	zl := ls.logger.With().Fields(DefaultRender(keysAndValues)).Logger()
	ls.logger = &zl
	return &ls
}

// WithCallDepth returns a new LogSink that offsets the call stack by adding specified depths.
func (ls logSink) WithCallDepth(depth int) logr.LogSink {
	ls.depth += depth
	return &ls
}

// DefaultRender is a default renderer for key-value zerolog fields that supports logr.Marshaler and fmt.Stringer.
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

// levelToLogType converts the log level to the corresponding LogType.
func levelToLogType(lvl int) api.LogType {
	switch lvl {
	case 0:
		return api.Info
	default:
		return api.Debug
	}
}
