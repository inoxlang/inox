package http_ns

import (
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/inoxlang/inox/internal/commonfmt"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

func readHttpServerArgs(ctx *core.Context, server *HttpServer, host core.Host, args ...core.Value) (
	addr string,
	certificate string,
	certKey *core.Secret,
	userProvidedHandler core.Value,
	handlerValProvided bool,
	middlewares []core.Value,
	argErr error,
) {

	const HANDLING_ARG_NAME = "handler/handling"

	//check host
	{
		parsed, _ := url.Parse(string(host))
		if host.Scheme() != "https" {
			argErr = fmt.Errorf("invalid scheme '%s'", host)
			return
		}
		server.host = host
		addr = parsed.Host

		perm := core.HttpPermission{Kind_: permkind.Provide, Entity: host}
		if err := ctx.CheckHasPermission(perm); err != nil {
			argErr = err
			return
		}
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Host:
			argErr = errors.New("address already provided")
			return
		case *core.InoxFunction:
			if handlerValProvided {
				argErr = commonfmt.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
				return
			}

			if ok, expl := v.IsSharable(server.state); !ok {
				argErr = fmt.Errorf("%w: %s", ErrHandlerNotSharable, expl)
				return
			}
			v.Share(server.state)
			userProvidedHandler = v
			handlerValProvided = true
		case *core.GoFunction:
			if handlerValProvided {
				argErr = commonfmt.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
				return
			}
			if ok, expl := v.IsSharable(server.state); !ok {
				argErr = fmt.Errorf("%w: %s", ErrHandlerNotSharable, expl)
				return
			}
			v.Share(server.state)
			userProvidedHandler = v
			handlerValProvided = true
		case *core.Mapping:
			if handlerValProvided {
				argErr = commonfmt.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
				return
			}
			if ok, expl := v.IsSharable(server.state); !ok {
				argErr = fmt.Errorf("%w: %s", ErrHandlerNotSharable, expl)
			}
			v.Share(server.state)

			userProvidedHandler = v
			handlerValProvided = true
		case *core.Object:
			if handlerValProvided {
				argErr = commonfmt.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
				return
			}
			handlerValProvided = true

			// extract routing handler, middlewares, ... from description
			for propKey, propVal := range v.EntryMap(ctx) {
				switch propKey {
				case HANDLING_DESC_MIDDLEWARES_PROPNAME:
					iterable, ok := propVal.(core.Iterable)
					if !ok {
						argErr = core.FmtPropOfArgXShouldBeOfTypeY(propKey, HANDLING_ARG_NAME, "iterable", propVal)
						return
					}

					it := iterable.Iterator(ctx, core.IteratorConfiguration{})
					for it.Next(ctx) {
						e := it.Value(ctx)
						if !isValidHandlerValue(e) {
							s := fmt.Sprintf("%s is not a middleware", core.Stringify(e, ctx))
							argErr = commonfmt.FmtUnexpectedElementInPropIterableOfArgX(propKey, HANDLING_ARG_NAME, s)
							return
						}

						if psharable, ok := e.(core.PotentiallySharable); ok && utils.Ret0(psharable.IsSharable(server.state)) {
							psharable.Share(server.state)
						} else {
							s := fmt.Sprintf("%s is not sharable", core.Stringify(e, ctx))
							argErr = commonfmt.FmtUnexpectedElementInPropIterableOfArgX(propKey, HANDLING_ARG_NAME, s)
							return
						}
						middlewares = append(middlewares, e)
					}
				case HANDLING_DESC_ROUTING_PROPNAME:
					if !isValidHandlerValue(propVal) {
						argErr = core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, HANDLING_ARG_NAME)
					}

					if path, ok := propVal.(core.Path); ok {
						if !path.IsDirPath() {
							argErr = commonfmt.FmtPropOfArgXShouldBeY(propKey, HANDLING_ARG_NAME, "absolute if it's a path")
							return
						}
						var err error
						propVal, err = path.ToAbs(ctx.GetFileSystem())
						if err != nil {
							argErr = err
							return
						}
					} else if obj, ok := propVal.(*core.Object); ok {
						properties := obj.PropertyNames(ctx)
						if slices.Contains(properties, "static") {
							static, ok := obj.Prop(ctx, "static").(core.Path)
							if !ok || !static.IsDirPath() {
								argErr = commonfmt.FmtPropOfArgXShouldBeY(propKey, HANDLING_ARG_NAME, symbolic.Stringify(HTTP_ROUTING_SYMB_OBJ))
							}
						}
						if slices.Contains(properties, "dynamic") {
							static, ok := obj.Prop(ctx, "dynamic").(core.Path)
							if !ok || !static.IsDirPath() {
								argErr = commonfmt.FmtPropOfArgXShouldBeY(propKey, HANDLING_ARG_NAME, symbolic.Stringify(HTTP_ROUTING_SYMB_OBJ))
							}
						}
					} else if psharable, ok := propVal.(core.PotentiallySharable); ok && utils.Ret0(psharable.IsSharable(server.state)) {
						psharable.Share(server.state)
					} else {
						argErr = commonfmt.FmtPropOfArgXShouldBeY(propKey, HANDLING_ARG_NAME, "sharable")
						return
					}

					userProvidedHandler = propVal
				case HANDLING_DESC_DEFAULT_CSP_PROPNAME:
					csp, ok := propVal.(*ContentSecurityPolicy)
					if !ok {
						argErr = core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, HANDLING_ARG_NAME)
						return
					}
					server.defaultCSP = csp
				case HANDLING_DESC_CERTIFICATE_PROPNAME:
					certVal, ok := propVal.(core.StringLike)
					if !ok {
						argErr = core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, HANDLING_ARG_NAME)
						return
					}
					certificate = certVal.GetOrBuildString()
				case HANDLING_DESC_KEY_PROPNAME:
					secret, ok := propVal.(*core.Secret)
					if !ok {
						argErr = core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, HANDLING_ARG_NAME)
						return
					}
					certKey = secret
				default:
					argErr = commonfmt.FmtUnexpectedPropInArgX(propKey, HANDLING_ARG_NAME)
				}
			}

			if userProvidedHandler == nil {
				argErr = commonfmt.FmtMissingPropInArgX(HANDLING_DESC_ROUTING_PROPNAME, HANDLING_ARG_NAME)
			}
		default:
			argErr = fmt.Errorf("http.server: invalid argument of type %T", v)
		}
	}

	if addr == "" {
		argErr = errors.New("no address provided")
		return
	}

	return
}