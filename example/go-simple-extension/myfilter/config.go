package httpfilter

type Config struct {
	ParentOnly     string            `json:"parent_only,omitempty"`
	RequestHeaders map[string]string `json:"request_headers,omitempty" envoy:"mergeable,preserve_root"`
}
