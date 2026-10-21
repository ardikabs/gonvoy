package gonvoy

import (
	"errors"
)

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

	if committed(c) {
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

	if committed(c) {
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

	if committed(c) {
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

	if committed(c) {
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

// committed reports whether the current context has already completed its
// processing within the filter and forwarded the result to Envoy.
func committed(c Context) bool {
	if hc, ok := c.(hostContext); ok {
		return hc.Committed()
	}

	return false
}

// invokePhase runs a filter phase, converting operation-not-permitted misuse
// into a returned error. Any other panic keeps propagating so real bugs
// still reach Finalize and never go unnoticed.
func invokePhase(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok && errors.Is(e, ErrOperationNotPermitted) {
				err = e
				return
			}

			panic(r)
		}
	}()

	return fn()
}

// filterProcessor chains a user Filter, dispatching only implemented phases.
type filterProcessor struct {
	filter Filter

	prev HttpFilterProcessor
	next HttpFilterProcessor
}

func newFilterProcessor(f Filter) *filterProcessor {
	return &filterProcessor{
		filter: f,
	}
}

func (p *filterProcessor) HandleOnRequestHeader(c Context) error {
	if f, ok := p.filter.(RequestHeaderFilter); ok {
		if err := invokePhase(func() error { return f.OnRequestHeader(c) }); err != nil {
			return err
		}
	}

	if committed(c) {
		return nil
	}

	if p.next != nil {
		return p.next.HandleOnRequestHeader(c)
	}

	return nil
}

func (p *filterProcessor) HandleOnRequestBody(c Context) error {
	if f, ok := p.filter.(RequestBodyFilter); ok {
		if err := invokePhase(func() error { return f.OnRequestBody(c) }); err != nil {
			return err
		}
	}

	if committed(c) {
		return nil
	}

	if p.next != nil {
		return p.next.HandleOnRequestBody(c)
	}

	return nil
}

func (p *filterProcessor) HandleOnResponseHeader(c Context) error {
	if f, ok := p.filter.(ResponseHeaderFilter); ok {
		if err := invokePhase(func() error { return f.OnResponseHeader(c) }); err != nil {
			return err
		}
	}

	if committed(c) {
		return nil
	}

	if p.prev != nil {
		return p.prev.HandleOnResponseHeader(c)
	}

	return nil
}

func (p *filterProcessor) HandleOnResponseBody(c Context) error {
	if f, ok := p.filter.(ResponseBodyFilter); ok {
		if err := invokePhase(func() error { return f.OnResponseBody(c) }); err != nil {
			return err
		}
	}

	if committed(c) {
		return nil
	}

	if p.prev != nil {
		return p.prev.HandleOnResponseBody(c)
	}

	return nil
}

func (p *filterProcessor) SetNext(next HttpFilterProcessor) {
	p.next = next
}

func (p *filterProcessor) SetPrevious(previous HttpFilterProcessor) {
	p.prev = previous
}
