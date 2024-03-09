package codecompletion

import (
	"path"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns/spec"

	parse "github.com/inoxlang/inox/internal/parse"
)

var (
	COMMON_CTX_ENTRY_NAMES = []string{}
)

func findPathCompletions(ctx *core.Context, rawPath string, pathLiteral parse.SimpleValueLiteral, search completionSearch) []Completion {

	switch parent := search.parent.(type) {
	case *parse.CallExpression:
		//ctx_data(/abs-path ....) ---> suggest context data entry names
		if rawPath != "" && rawPath[0] == '/' && parent.IsCalleeNamed(globalnames.CTX_DATA_FN) && len(parent.Arguments) > 0 && parent.Arguments[0] == pathLiteral {
			return findCtxDataEntryNameCompletions(ctx, rawPath, pathLiteral, search)
		}
	}

	return findFilePathCompletions(ctx, rawPath, search)
}

func findCtxDataEntryNameCompletions(ctx *core.Context, rawPath string, pathLiteral parse.SimpleValueLiteral, search completionSearch) (completions []Completion) {

	replacedRange := search.chunk.GetSourcePosition(pathLiteral.Base().Span)
	onlySlash := rawPath == "/"

	if onlySlash || strings.HasPrefix(string(http_ns.SESSION_CTX_DATA_KEY), rawPath) {
		s := string(http_ns.SESSION_CTX_DATA_KEY)
		completions = append(completions, Completion{
			ShownString:   s,
			Value:         s,
			Kind:          defines.CompletionItemKindConstant,
			ReplacedRange: replacedRange,
			LabelDetail:   "this entry is set for req.s with a session cookie",
		})
	}

	if onlySlash || strings.HasPrefix(string(http_ns.PATH_PARAMS_CTX_DATA_NAMESPACE), rawPath) {

		namespaceName := string(http_ns.PATH_PARAMS_CTX_DATA_NAMESPACE)
		s := namespaceName + "url_param_name_example"

		completions = append(completions, Completion{
			ShownString:   s,
			Value:         s,
			Kind:          defines.CompletionItemKindConstant,
			ReplacedRange: replacedRange,
		})

		//Suggest parameter names based on existing endpoints.
		api := search.inputData.ServerAPI
		if api != nil {
			api.ForEachHandlerModule(func(mod *core.ModulePreparationCache, endpoint *spec.ApiEndpoint, operation spec.ApiOperation) error {
				endpoint.ForEachPathSegment(func(segment spec.EndpointPathSegment) (_ error) {
					if segment.ParameterName == "" {
						return
					}

					entryKey := namespaceName + segment.ParameterName

					completions = append(completions, Completion{
						ShownString:   entryKey,
						Value:         entryKey,
						LabelDetail:   "[HTTP endpoint] " + endpoint.PathWithParams(),
						Kind:          defines.CompletionItemKindConstant,
						ReplacedRange: replacedRange,
					})
					return
				})
				return nil
			})
		}
	}

	return
}

func findFilePathCompletions(ctx *core.Context, pth string, _ completionSearch) []Completion {
	var completions []Completion

	fls := ctx.GetFileSystem()
	dir := path.Dir(pth)
	base := path.Base(pth)

	if core.Path(pth).IsDirPath() {
		base = ""
	}

	entries, err := fs_ns.ListFiles(ctx, core.ToValueOptionalParam(core.Path(dir+"/")))
	if err != nil {
		return nil
	}

	for _, e := range entries {
		name := string(e.BaseName_)
		if strings.HasPrefix(name, base) {
			pth := path.Join(dir, name)

			if !parse.HasPathLikeStart(pth) {
				pth = "./" + pth
			}

			stat, _ := fls.Stat(pth)
			if stat.IsDir() {
				pth += "/"
			}

			completions = append(completions, Completion{
				ShownString: name,
				Value:       pth,
				Kind:        defines.CompletionItemKindConstant,
				LabelDetail: "%" + core.PATH_PATTERN.Name,
			})
		}
	}

	return completions
}
