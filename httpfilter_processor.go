package gaetway

// HttpFilterProcessor is an interface that defines the methods for processing HTTP filter phases and enabling chaining between user's HTTP filter handlers.
type HttpFilterProcessor interface {
	HttpFilterDecodeProcessor
	HttpFilterEncodeProcessor

	// SetNext sets the next HttpFilterProcessor in the sequence.
	SetNext(HttpFilterProcessor)

	// SetPrevious sets the previous HttpFilterProcessor in the sequence.
	SetPrevious(HttpFilterProcessor)
}

// HttpFilterDecodeProcessor is an interface that defines the methods for processing HTTP filter decode phases.
type HttpFilterDecodeProcessor interface {
	// HandleOnRequestHeader manages operations during the OnRequestHeader phase.
	HandleOnRequestHeader(Context) error

	// HandleOnRequestBody manages operations during the OnRequestBody phase.
	HandleOnRequestBody(Context) error
}

// HttpFilterEncodeProcessor is an interface that defines the methods for processing HTTP filter encode phases.
type HttpFilterEncodeProcessor interface {
	// HandleOnResponseHeader manages operations during the OnResponseHeader phase.
	HandleOnResponseHeader(Context) error

	// HandleOnResponseBody manages operations during the OnResponseBody phase.
	HandleOnResponseBody(Context) error
}

type httpFilterProcessor struct {
	HttpFilterHandler

	prev HttpFilterProcessor
	next HttpFilterProcessor
}

func newHttpFilterProcessor(hf HttpFilterHandler) *httpFilterProcessor {
	return &httpFilterProcessor{
		HttpFilterHandler: hf,
	}
}

func (p *httpFilterProcessor) HandleOnRequestHeader(c Context) error {
	if err := p.OnRequestHeader(c); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if p.next != nil {
		return p.next.HandleOnRequestHeader(c)
	}

	return nil
}

func (p *httpFilterProcessor) HandleOnRequestBody(c Context) error {
	if err := p.OnRequestBody(c); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if p.next != nil {
		return p.next.HandleOnRequestBody(c)
	}

	return nil
}

func (p *httpFilterProcessor) HandleOnResponseHeader(c Context) error {
	if err := p.OnResponseHeader(c); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if p.prev != nil {
		return p.prev.HandleOnResponseHeader(c)
	}

	return nil
}

func (p *httpFilterProcessor) HandleOnResponseBody(c Context) error {
	if err := p.OnResponseBody(c); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if p.prev != nil {
		return p.prev.HandleOnResponseBody(c)
	}

	return nil
}

func (p *httpFilterProcessor) SetNext(next HttpFilterProcessor) {
	p.next = next
}

func (p *httpFilterProcessor) SetPrevious(previous HttpFilterProcessor) {
	p.prev = previous
}
