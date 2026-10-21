package gonvoy

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSON_WritesJSONReply(t *testing.T) {
	t.Parallel()

	w := newRecordingWriter()
	JSON(w, http.StatusTeapot, NewMinimalJSONResponse("TEAPOT", "Teapot"))

	assert.Equal(t, http.StatusTeapot, w.Status())
	assert.Equal(t, MIMEApplicationJSONCharsetUTF8, w.Header().Get(HeaderContentType))

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(w.body, &payload))
	assert.Equal(t, "TEAPOT", payload["code"])
}

func TestString_WritesTextReply(t *testing.T) {
	t.Parallel()

	w := newRecordingWriter()
	String(w, http.StatusUnauthorized, "unauthorized")

	assert.Equal(t, http.StatusUnauthorized, w.Status())
	assert.Equal(t, MIMETextPlainCharsetUTF8, w.Header().Get(HeaderContentType))
	assert.Equal(t, "unauthorized", string(w.body))
}

func TestJSON_ExplicitStatusWinsOverImplied(t *testing.T) {
	t.Parallel()

	w := newRecordingWriter()
	_, _ = w.Write([]byte("early"))
	JSON(w, http.StatusBadRequest, NewMinimalJSONResponse("BAD_REQUEST", "Bad Request"))

	assert.Equal(t, http.StatusOK, w.Status())
}
