package http_ns

import (
	"fmt"
	"net/http"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	html_ns_symb "github.com/inoxlang/inox/internal/globals/html_ns/symbolic"
	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

const (
	RESULT_INIT_STATUS_PROPNAME  = "status"
	RESULT_INIT_BODY_PROPNAME    = "body"
	RESULT_INIT_HEADERS_PROPNAME = "headers"
	RESULT_INIT_SESSION_PROPNAME = "session"
)

var (
	SYMBOLIC_RESULT_INIT_ARG = symbolic.NewInexactObject(
		map[string]symbolic.Serializable{
			RESULT_INIT_STATUS_PROPNAME:  http_ns_symb.ANY_STATUS_CODE,
			RESULT_INIT_BODY_PROPNAME:    symbolic.AsSerializableChecked(symbolic.NewMultivalue(html_ns_symb.ANY_HTML_NODE, symbolic.ANY_STR_LIKE)),
			RESULT_INIT_HEADERS_PROPNAME: symbolic.NewInexactObject2(map[string]symbolic.Serializable{}),
			RESULT_INIT_SESSION_PROPNAME: symbolic.NewInexactObject2(map[string]symbolic.Serializable{SESSION_ID_PROPNAME: symbolic.ANY_STR_LIKE}),
		},
		map[string]struct{}{
			RESULT_INIT_STATUS_PROPNAME:  {},
			RESULT_INIT_BODY_PROPNAME:    {},
			RESULT_INIT_HEADERS_PROPNAME: {},
			RESULT_INIT_SESSION_PROPNAME: {},
		},
		nil)
	NEW_RESULT_PARAMS      = &[]symbolic.Value{SYMBOLIC_RESULT_INIT_ARG}
	NEW_RESULT_PARAM_NAMES = []string{"init"}

	_ = core.Value((*Result)(nil))
)

type Result struct {
	value   core.Serializable //can be nil
	status  StatusCode
	headers http.Header
	session *core.Object //can be nil
	//cookies []core.Serializable
}

func NewResult(ctx *core.Context, init *core.Object) *Result {
	status := StatusCode(http.StatusOK)
	var value core.Serializable
	var headers http.Header
	var session *core.Object

	init.ForEachEntry(func(k string, v core.Serializable) error {
		switch k {
		case RESULT_INIT_STATUS_PROPNAME:
			status = v.(StatusCode)
		case RESULT_INIT_BODY_PROPNAME:
			value = v
		case RESULT_INIT_HEADERS_PROPNAME:
			headers = http.Header{}
			v.(*core.Object).ForEachEntry(func(headerName string, propertyValue core.Serializable) error {

				strLike, ok := propertyValue.(core.StringLike)
				if ok {
					headerValue := strLike.GetOrBuildString()
					headers.Add(headerName, headerValue)

				} else {
					//Handle iterable after string like because string-like values are iterable.
					iterable := propertyValue.(core.Iterable)
					core.ForEachValueInIterable(ctx, iterable, func(v core.Value) error {
						headerValue := v.(core.StringLike).GetOrBuildString()
						headers.Add(headerName, headerValue)
						return nil
					})
				}

				return nil
			})
		case RESULT_INIT_SESSION_PROPNAME:
			session = v.(*core.Object)

			//If the id is not a secure hexadecimal session ID we change its value.
			id := session.Prop(ctx, SESSION_ID_PROPNAME).(core.StringLike).GetOrBuildString()
			if !isValidHexSessionID(id) {
				err := session.SetProp(ctx, SESSION_ID_PROPNAME, core.String(randomSessionID()))
				if err != nil {
					panic(err)
				}
			}

			session.Share(ctx.MustGetClosestState())

			//TODO: panic if the object is watched or has cycles.
		}
		return nil
	})

	return &Result{
		value:   value,
		status:  status,
		headers: headers,
		session: session,
	}
}

func symbolicNewResult(ctx *symbolic.Context, init *symbolic.Object) *http_ns_symb.Result {
	ctx.SetSymbolicGoFunctionParameters(NEW_RESULT_PARAMS, NEW_RESULT_PARAM_NAMES)

	if symbolic.HasRequiredOrOptionalProperty(init, RESULT_INIT_HEADERS_PROPNAME) {
		headers, ok := init.Prop(RESULT_INIT_HEADERS_PROPNAME).(*symbolic.Object)
		if ok {
			headers.ForEachEntry(func(headerName string, propertyValue symbolic.Value) error {
				isValidValue := false
				switch propertyValue := propertyValue.(type) {
				case symbolic.StringLike:
					isValidValue = true
				case symbolic.Iterable:
					_, isStrLike := symbolic.AsStringLike(propertyValue.IteratorElementValue()).(symbolic.StringLike)
					isValidValue = isStrLike
				}
				if !isValidValue {
					errMsg := fmt.Sprintf("invalid value for header %q, only string-like values and lists (iterables) containing string-likes are allowed", headerName)
					ctx.AddSymbolicGoFunctionError(errMsg)
				}
				return nil
			})
		}
	}

	return http_ns_symb.ANY_RESULT
}
