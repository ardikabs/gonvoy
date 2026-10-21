package gonvoy

// JSON answers with a JSON body. It sets the JSON content type, records
// the status, and writes the body. Prefer it over raw Write calls when
// the answer is JSON; fall back to WriteHeader plus Write for anything else.
func JSON(w ResponseWriter, code int, body []byte) {
	w.Header().Set(HeaderContentType, MIMEApplicationJSONCharsetUTF8)
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

// String answers with a plain-text body. It sets the plain-text content
// type, records the status, and writes the body. Prefer it over raw Write
// calls when the answer is plain text; fall back to WriteHeader plus Write
// for anything else.
func String(w ResponseWriter, code int, s string) {
	w.Header().Set(HeaderContentType, MIMETextPlainCharsetUTF8)
	w.WriteHeader(code)
	_, _ = w.Write([]byte(s))
}
