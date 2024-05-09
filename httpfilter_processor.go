package gonvoy

// HttpFilterProcessor ---
type HttpFilterProcessor interface {
	HandleOnRequestHeader(Context) error
	HandleOnResponseHeader(Context) error
	HandleOnRequestBody(Context) error
	HandleOnResponseBody(Context) error

	SetNext(HttpFilterProcessor)
}

type defaultHttpFilterProcessor struct {
	handler HttpFilterHandler
	next    HttpFilterProcessor
}

func newHttpFilterProcessor(hf HttpFilterHandler) *defaultHttpFilterProcessor {
	return &defaultHttpFilterProcessor{
		handler: hf,
	}
}

func (b *defaultHttpFilterProcessor) HandleOnRequestHeader(c Context) error {
	if err := b.handler.OnRequestHeader(c, c.Request().Header); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if b.next != nil {
		return b.next.HandleOnRequestHeader(c)
	}

	return nil
}

func (b *defaultHttpFilterProcessor) HandleOnResponseHeader(c Context) error {
	if err := b.handler.OnResponseHeader(c, c.Response().Header); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if b.next != nil {
		return b.next.HandleOnResponseHeader(c)
	}

	return nil
}

func (b *defaultHttpFilterProcessor) HandleOnRequestBody(c Context) error {
	if err := b.handler.OnRequestBody(c, c.RequestBody().Bytes()); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if b.next != nil {
		return b.next.HandleOnRequestBody(c)
	}

	return nil
}

func (b *defaultHttpFilterProcessor) HandleOnResponseBody(c Context) error {
	if err := b.handler.OnResponseBody(c, c.ResponseBody().Bytes()); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if b.next != nil {
		return b.next.HandleOnResponseBody(c)
	}

	return nil
}

func (b *defaultHttpFilterProcessor) SetNext(hfp HttpFilterProcessor) {
	b.next = hfp
}
