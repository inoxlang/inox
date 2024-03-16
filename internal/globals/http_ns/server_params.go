package http_ns

import (
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/setcoll"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_HTTPS_PORT = "443"

	SERVER_HANDLING_ARG_NAME = "handler/handling"
)

type serverParams struct {
	effectiveAddr              string
	effectiveListeningAddrHost core.Host
	port                       string
	exposingAllowed            bool
	certificate                string
	certKey                    *core.Secret

	handlerValProvided  bool
	userProvidedHandler core.Value

	defaultLimits map[string]core.Limit
	maxLimits     map[string]core.Limit

	sessions *setcoll.Set
}

func determineHttpServerParams(ctx *core.Context, server *HttpsServer, providedHost core.Host, args ...core.Value) (params serverParams, argErr error) {
	params.defaultLimits = map[string]core.Limit{}
	for _, limit := range core.GetDefaultRequestHandlingLimits() {
		params.defaultLimits[limit.Name] = limit
	}
	params.maxLimits = map[string]core.Limit{}
	for _, limit := range core.GetDefaultMaxRequestHandlerLimits() {
		params.maxLimits[limit.Name] = limit
	}

	//check host
	{
		parsed, _ := url.Parse(string(providedHost))
		if providedHost.Scheme() != "https" {
			argErr = fmt.Errorf("invalid scheme '%s', only https is supported", providedHost)
			return
		}

		perm := core.HttpPermission{Kind_: permkind.Provide, Entity: providedHost}
		if err := ctx.CheckHasPermission(perm); err != nil {
			argErr = err
			return
		}

		params.effectiveAddr = parsed.Host
		originalAddress := params.effectiveAddr
		params.port = parsed.Port()
		if params.port == "" {
			params.port = DEFAULT_HTTPS_PORT
		}

		if isBindAllAddress(params.effectiveAddr) {
			if server.state.Project == nil || !server.state.Project.Configuration().AreExposedWebServersAllowed() {
				//if exposing web servers is not allowed we only bind to localhost.
				params.effectiveAddr = "localhost"
				params.effectiveAddr += ":" + params.port
				params.effectiveListeningAddrHost = core.Host("https://" + params.effectiveAddr)
				server.state.Ctx.Logger().
					Warn().Msgf("exposing web servers is not allowed, change listening address from %s to %s", originalAddress, params.effectiveAddr)
			} else {
				params.effectiveListeningAddrHost = providedHost
				params.exposingAllowed = true
			}
		} else {
			params.effectiveListeningAddrHost = providedHost
		}
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Host:
			argErr = errors.New("address already provided")
			return
		case *core.InoxFunction:
			if params.handlerValProvided {
				argErr = commonfmt.FmtErrArgumentProvidedAtLeastTwice(SERVER_HANDLING_ARG_NAME)
				return
			}

			if ok, expl := v.IsSharable(server.state); !ok {
				argErr = fmt.Errorf("%w: %s", ErrHandlerNotSharable, expl)
				return
			}
			v.Share(server.state)
			params.userProvidedHandler = v
			params.handlerValProvided = true
		case *core.GoFunction:
			if params.handlerValProvided {
				argErr = commonfmt.FmtErrArgumentProvidedAtLeastTwice(SERVER_HANDLING_ARG_NAME)
				return
			}
			if ok, expl := v.IsSharable(server.state); !ok {
				argErr = fmt.Errorf("%w: %s", ErrHandlerNotSharable, expl)
				return
			}
			v.Share(server.state)
			params.userProvidedHandler = v
			params.handlerValProvided = true
		case *core.Mapping:
			if params.handlerValProvided {
				argErr = commonfmt.FmtErrArgumentProvidedAtLeastTwice(SERVER_HANDLING_ARG_NAME)
				return
			}
			if ok, expl := v.IsSharable(server.state); !ok {
				argErr = fmt.Errorf("%w: %s", ErrHandlerNotSharable, expl)
			}
			v.Share(server.state)

			params.userProvidedHandler = v
			params.handlerValProvided = true
		case *core.Object:
			if params.handlerValProvided {
				argErr = commonfmt.FmtErrArgumentProvidedAtLeastTwice(SERVER_HANDLING_ARG_NAME)
				continue
			}
			params.handlerValProvided = true
			err := readServerHandlingObject(ctx, v, server, &params)
			if err != nil {
				argErr = err
				return
			}
		default:
			argErr = fmt.Errorf("http.server: invalid argument of type %T", v)
		}
	}

	if params.effectiveAddr == "" {
		argErr = errors.New("no address provided")
		return
	}

	return
}

func readServerHandlingObject(ctx *core.Context, handlingParams *core.Object, server *HttpsServer, params *serverParams) error {

	err := handlingParams.ForEachEntry(func(propKey string, propVal core.Serializable) error {
		switch propKey {
		case HANDLING_DESC_ROUTING_PROPNAME:
			if !isValidHandlerValue(propVal) {
				return core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, SERVER_HANDLING_ARG_NAME)
			}

			if path, ok := propVal.(core.Path); ok {
				if !path.IsDirPath() {
					return commonfmt.FmtPropOfArgXShouldBeY(propKey, SERVER_HANDLING_ARG_NAME, "absolute if it's a path")
				}
				var err error
				propVal, err = path.ToAbs(ctx.GetFileSystem())
				if err != nil {
					return err
				}
			} else if obj, ok := propVal.(*core.Object); ok {
				properties := obj.PropertyNames(ctx)
				if slices.Contains(properties, STATIC_DIR_PROPNAME) {
					static, ok := obj.Prop(ctx, STATIC_DIR_PROPNAME).(core.Path)
					if !ok || !static.IsDirPath() {
						return commonfmt.FmtPropOfArgXShouldBeY(propKey, SERVER_HANDLING_ARG_NAME, symbolic.Stringify(HTTP_ROUTING_SYMB_OBJ))
					}
				}
				if slices.Contains(properties, DYNAMIC_DIR_PROPNAME) {
					static, ok := obj.Prop(ctx, DYNAMIC_DIR_PROPNAME).(core.Path)
					if !ok || !static.IsDirPath() {
						return commonfmt.FmtPropOfArgXShouldBeY(propKey, SERVER_HANDLING_ARG_NAME, symbolic.Stringify(HTTP_ROUTING_SYMB_OBJ))
					}
				}
			} else if psharable, ok := propVal.(core.PotentiallySharable); ok && utils.Ret0(psharable.IsSharable(server.state)) {
				psharable.Share(server.state)
			} else {
				return commonfmt.FmtPropOfArgXShouldBeY(propKey, SERVER_HANDLING_ARG_NAME, "sharable")
			}

			params.userProvidedHandler = propVal
		case HANDLING_DESC_DEFAULT_CSP_PROPNAME:
			csp, ok := propVal.(*ContentSecurityPolicy)
			if !ok {
				return core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, SERVER_HANDLING_ARG_NAME)
			}
			server.defaultCSP = csp
		case HANDLING_DESC_CERTIFICATE_PROPNAME:
			certVal, ok := propVal.(core.StringLike)
			if !ok {
				return core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, SERVER_HANDLING_ARG_NAME)
			}
			params.certificate = certVal.GetOrBuildString()
		case HANDLING_DESC_KEY_PROPNAME:
			secret, ok := propVal.(*core.Secret)
			if !ok {
				return core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, SERVER_HANDLING_ARG_NAME)
			}
			params.certKey = secret
		case HANDLING_DESC_SESSIONS_PROPNAME:
			sessionsDesc, ok := propVal.(*core.Object)
			if !ok {
				return core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, SERVER_HANDLING_ARG_NAME)
			}
			collection := sessionsDesc.Prop(ctx, SESSIONS_DESC_COLLECTION_PROPNAME).(*setcoll.Set)
			params.sessions = collection
		case HANDLING_DESC_DEFAULT_LIMITS_PROPNAME, HANDLING_DESC_MAX_LIMITS_PROPNAME:
			val, ok := propVal.(*core.Object)
			if !ok {
				return core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, SERVER_HANDLING_ARG_NAME)
			}

			var limits map[string]core.Limit
			if propKey == HANDLING_DESC_DEFAULT_LIMITS_PROPNAME {
				limits = params.defaultLimits
			} else {
				params.maxLimits = map[string]core.Limit{}
				limits = params.maxLimits
			}

			err := val.ForEachEntry(func(k string, v core.Serializable) error {
				limit, err := core.GetLimit(ctx, k, v)
				if err != nil {
					s := fmt.Sprintf("unknown limit %q", k)
					return commonfmt.FmtInvalidValueForPropXOfArgY(propKey, SERVER_HANDLING_ARG_NAME, s)
				}
				limits[k] = limit
				return nil
			})
			if err != nil {
				params.defaultLimits = nil
				params.maxLimits = nil
				return err
			}
		default:
			return commonfmt.FmtUnexpectedPropInArgX(propKey, SERVER_HANDLING_ARG_NAME)
		}

		return nil
	})

	if err != nil {
		return err
	}

	if params.userProvidedHandler == nil {
		return commonfmt.FmtMissingPropInArgX(HANDLING_DESC_ROUTING_PROPNAME, SERVER_HANDLING_ARG_NAME)
	}

	return nil
}
