package http_ns

import (
	"io"
	"net/http"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core"

	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

type Response struct {
	wrapped      *http.Response
	cookies      []core.Serializable
	markedClosed atomic.Bool
}

func (resp *Response) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (resp *Response) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "body":
		return core.WrapReader(resp.wrapped.Body, nil)
	case "status":
		return core.String(resp.wrapped.Status)
	case "status-code":
		//TOOD: use checked "int" ?
		return StatusCode(resp.wrapped.StatusCode)
	case "cookies":
		// TODO: make cookies immutable ?

		if resp.cookies != nil {
			return core.NewWrappedValueList(resp.cookies...)
		}
		cookies := resp.wrapped.Cookies()
		resp.cookies = make([]core.Serializable, len(cookies))

		for i, c := range cookies {
			resp.cookies[i] = createObjectFromCookie(ctx, *c)
		}

		return core.NewWrappedValueList(resp.cookies...)
	default:
		method, ok := resp.GetGoMethod(name)
		if !ok {
			panic(core.FormatErrPropertyDoesNotExist(name, resp))
		}
		return method
	}
}

func (*Response) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Response) PropertyNames(ctx *core.Context) []string {
	return http_ns_symb.HTTP_RESPONSE_PROPNAMES
}

func (resp *Response) ContentType(ctx *core.Context) (core.Mimetype, bool, error) {
	contentType := resp.wrapped.Header.Get("Content-Type")
	if contentType == "" {
		return "", false, nil
	}
	mtype, err := core.MimeTypeFrom(contentType)
	if err != nil {
		return "", false, err
	}
	return mtype, true, nil
}

func (resp *Response) Body(ctx *core.Context) io.ReadCloser {
	return resp.wrapped.Body
}

func (resp *Response) StatusCode(ctx *core.Context) int {
	return resp.wrapped.StatusCode
}

func (resp *Response) Status(ctx *core.Context) string {
	return resp.wrapped.Status
}

func (resp *Response) StdlibResponse() *http.Response {
	return resp.wrapped
}

func (resp *Response) CloseBody() error {
	defer resp.markedClosed.Store(true)
	return resp.wrapped.Body.Close()
}