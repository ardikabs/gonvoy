package gonvoy

// HttpFilterProcessor defines an interface for processing HTTP filter phases,
// and enabling chaining between user's HTTP filter handlers.
type HttpFilterProcessor interface {
	// HandleOnRequestHeader manages operations during the OnRequestHeader phase.
	//
	HandleOnRequestHeader(Context) error

	// HandleOnRequestBody manages operations during the OnRequestBody phase.
	//
	HandleOnRequestBody(Context) error

	// HandleOnResponseHeader manages operations during the OnResponseHeader phase.
	//
	HandleOnResponseHeader(Context) error

	// HandleOnResponseBody manages operations during the OnResponseBody phase.
	//
	HandleOnResponseBody(Context) error

	// SetNext sets the next HttpFilterProcessor in the sequence.
	// It is specifically used for managing the flow of HTTP requests.
	//
	SetNext(HttpFilterProcessor)

	// SetPrevious sets the previous HttpFilterProcessor in the sequence.
	// It is specifically used for managing the flow of HTTP responses.
	//
	SetPrevious(HttpFilterProcessor)
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
	if err := p.OnRequestHeader(c, c.Request().Header); err != nil {
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
	if err := p.OnRequestBody(c, c.RequestBody().Bytes()); err != nil {
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
	if err := p.OnResponseHeader(c, c.Response().Header); err != nil {
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
	if err := p.OnResponseBody(c, c.ResponseBody().Bytes()); err != nil {
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

func (p *httpFilterProcessor) SetNext(hfp HttpFilterProcessor) {
	p.next = hfp
}

func (p *httpFilterProcessor) SetPrevious(hfp HttpFilterProcessor) {
	p.prev = hfp
}
