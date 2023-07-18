package http_ns

import (
	"io"
	"net/http"

	core "github.com/inoxlang/inox/internal/core"

	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

type HttpResponse struct {
	wrapped *http.Response
	cookies []core.Serializable
}

func (resp *HttpResponse) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (resp *HttpResponse) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "body":
		return core.WrapReader(resp.wrapped.Body, nil)
	case "status":
		return core.Str(resp.wrapped.Status)
	case "statusCode":
		//TOOD: use checked "int" ?
		return core.Int(resp.wrapped.StatusCode)
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

func (*HttpResponse) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*HttpResponse) PropertyNames(ctx *core.Context) []string {
	return http_ns_symb.HTTP_RESPONSE_PROPNAMES
}

func (resp *HttpResponse) ContentType(ctx *core.Context) string {
	return resp.wrapped.Header.Get("Content-Type")
}

func (resp *HttpResponse) Body(ctx *core.Context) io.ReadCloser {
	return resp.wrapped.Body
}

func (resp *HttpResponse) StatusCode(ctx *core.Context) int {
	return resp.wrapped.StatusCode
}

func (resp *HttpResponse) Status(ctx *core.Context) string {
	return resp.wrapped.Status
}

//
