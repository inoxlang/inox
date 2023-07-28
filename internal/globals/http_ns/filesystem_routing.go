package http_ns

import (
	"net/http"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"golang.org/x/exp/slices"
)

func createHandleDynamic(server *HttpServer, routingDirPath core.Path) handlerFn {
	return func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
		fls := server.state.Ctx.GetFileSystem()
		path := req.Path
		if !path.IsAbsolute() {
			panic(core.ErrUnreachable)
		}

		if path.IsDirPath() && path != "/" {
			rw.writeStatus(http.StatusNotFound)
			return
		}

		if slices.Contains(strings.Split(path.UnderlyingString(), "/"), "..") {
			rw.writeStatus(http.StatusNotFound)
			return
		}

		modulePath := fls.Join(string(routingDirPath), string(path)+".ix")

		_, err := fls.Stat(modulePath)
		if err != nil {
			modulePath = fls.Join(string(routingDirPath), string(path), "index.ix")
		}

		_, err = fls.Stat(modulePath)
		if err != nil {
			rw.writeStatus(http.StatusNotFound)
			return
		}

		handlerCtx := handlerGlobalState.Ctx

		//TODO: check the file is not writable

		result, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     modulePath,
			ParentContext:             handlerCtx,
			ParentContextRequired:     true,
			ParsingCompilationContext: handlerCtx,
			Out:                       handlerGlobalState.Out,
			LogOut:                    handlerGlobalState.Logger,

			FullAccessToDatabases: false, //databases should be passed by parent state
			IgnoreHighRiskScore:   true,
			PreinitFilesystem:     handlerCtx.GetFileSystem(),
		})

		if err != nil {
			handlerGlobalState.Logger.Err(err).Send()
			rw.writeStatus(http.StatusNotFound)
			return
		}

		respondWithMappingResult(handlingArguments{
			value:        result,
			req:          req,
			rw:           rw,
			state:        handlerGlobalState,
			server:       server,
			logger:       handlerGlobalState.Logger,
			isMiddleware: false,
		})
	}
}
