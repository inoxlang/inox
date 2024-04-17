package codecompletion

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func findURLCompletions(ctx *core.Context, node *parse.URLLiteral, search completionSearch) (completions []Completion) {

	res, err := core.EvalSimpleValueLiteral(node, nil)
	if err != nil {
		return
	}
	u := res.(core.URL)
	urlString := string(u)

	if call, ok := search.parent.(*parse.CallExpression); ok {

		var S3_FNS = []string{"get", "delete", "ls"}

		if memb, ok := call.Callee.(*parse.IdentifierMemberExpression); ok &&
			memb.Left.Name == "s3" &&
			len(memb.PropertyNames) == 1 &&
			utils.SliceContains(S3_FNS, memb.PropertyNames[0].Name) &&
			strings.Contains(urlString, "/") {

			objects, err := s3_ns.S3List(ctx, u)
			if err == nil {
				prefix := urlString[:strings.LastIndex(urlString, "/")+1]
				for _, obj := range objects {

					val := prefix + filepath.Base(obj.Key)
					if strings.HasSuffix(obj.Key, "/") {
						val += "/"
					}

					completions = append(completions, Completion{
						ShownString: obj.Key,
						Value:       val,
						Kind:        defines.CompletionItemKindConstant,
						LabelDetail: "%" + core.URL_PATTERN.Name,
					})
				}
			}
		}
	}

	globalState := search.state.Global

	switch string(u.Scheme()) {
	case inoxconsts.LDB_SCHEME_NAME:
		dbHost := u.Host()
		dbName := dbHost.Name()
		path := string(u.Path())

		data, ok := globalState.SymbolicData.GetGlobalScopeData(node, search.ancestorChain)
		if !ok {
			return
		}

		var db *symbolic.DatabaseIL

		for _, variable := range data.Variables {
			if variable.Name == globalnames.DATABASES {
				ns, ok := variable.Value.(*symbolic.Namespace)
				if !ok {
					return
				}
				if !slices.Contains(ns.PropertyNames(), dbName) {
					return
				}
				db = ns.Prop(dbName).(*symbolic.DatabaseIL)
			}
		}

		if db == nil {
			return
		}

		for _, path := range db.GetPseudoPathCompletions(path, true) {
			urlPattern := string(dbHost) + path
			completions = append(completions, Completion{
				ShownString: urlPattern,
				Value:       urlPattern,
				Kind:        defines.CompletionItemKindText,
			})
		}
	}

	return completions
}

func findURLPatternCompletions(ctx *core.Context, node *parse.URLPatternLiteral, search completionSearch) (completions []Completion) {
	globalState := search.state.Global

	res, err := core.EvalSimpleValueLiteral(node, nil)
	if err != nil {
		return
	}
	p := res.(core.URLPattern)

	switch string(p.Scheme()) {
	case inoxconsts.LDB_SCHEME_NAME:
		dbHost := p.Host()
		dbName := dbHost.Name()

		pseudoPath, ok := p.PseudoPath()
		if !ok {
			return
		}

		data, ok := globalState.SymbolicData.GetGlobalScopeData(node, search.ancestorChain)
		if !ok {
			return
		}

		var db *symbolic.DatabaseIL

		for _, variable := range data.Variables {
			if variable.Name == globalnames.DATABASES {
				ns, ok := variable.Value.(*symbolic.Namespace)
				if !ok {
					return
				}
				if !slices.Contains(ns.PropertyNames(), dbName) {
					return
				}
				db = ns.Prop(dbName).(*symbolic.DatabaseIL)
			}
		}

		if db == nil {
			return
		}

		//TODO: determine why vscode does not show completions ending with '/*'.
		//TODO: determine why vscode does not show completions if the segment written by the user has more that one character.

		for _, path := range db.GetPseudoPathCompletions(pseudoPath, true) {
			urlPattern := "%" + string(dbHost) + path
			completions = append(completions, Completion{
				ShownString: urlPattern,
				Value:       urlPattern,
				Kind:        defines.CompletionItemKindText,
			})
		}
	}

	return
}

func findHostCompletions(ctx *core.Context, node parse.Node, search completionSearch) []Completion {
	value := ""
	scheme := ""
	hostPrefix := ""
	isHostPattern := false

	switch node := node.(type) {
	case *parse.HostLiteral:
		var ok bool
		value = node.Value
		scheme, hostPrefix, ok = strings.Cut(node.Value, "://")

		if !ok {
			return nil
		}
	case *parse.HostPatternLiteral:
		var ok bool
		value = node.Value
		scheme, hostPrefix, ok = strings.Cut(node.Value, "://")
		isHostPattern = true

		if !ok || strings.Contains(hostPrefix, "*") {
			return nil
		}
	case *parse.SchemeLiteral:
		value = node.Name
		scheme = node.Name
	default:
		return nil
	}

	var completions []Completion

	allDefinitions := ctx.GetAllHostDefinitions()

	for host := range allDefinitions {
		hostStr := string(host)
		if strings.HasPrefix(hostStr, value) {
			labelDetail := ""
			if isHostPattern {
				hostStr = "%" + hostStr
				labelDetail = "%" + core.HOSTPATTERN_PATTERN.Name
				//Adding '%' makes VSCode more likely to list the completions.
			} else {
				labelDetail = "%" + core.HOST_PATTERN.Name
			}

			completions = append(completions, Completion{
				ShownString:   hostStr,
				Value:         hostStr,
				Kind:          defines.CompletionItemKindConstant,
				LabelDetail:   labelDetail,
				ReplacedRange: search.chunk.GetSourcePosition(node.Base().Span),
			})
		}
	}

	switch scheme {
	case inoxconsts.LDB_SCHEME_NAME:
		//Nothing to suggest since databases should be in host definitions (suggested above).
	case "http", "https", "ws", "wss":
		if len(hostPrefix) > 0 && strings.HasPrefix("localhost", hostPrefix) {
			s := strings.Replace(value, hostPrefix, "localhost", 1)
			completions = append(completions, Completion{
				ShownString:   s,
				Value:         s,
				Kind:          defines.CompletionItemKindConstant,
				LabelDetail:   "%" + core.HOST_PATTERN.Name,
				ReplacedRange: search.chunk.GetSourcePosition(node.Base().Span),
			})
		}
	}

	return completions
}

func findHostAliasCompletions(ctx *core.Context, prefix string, parent parse.Node) []Completion {
	var completions []Completion

	//TODO
	// for alias, host := range ctx.GetHostAliases() {
	// 	if strings.HasPrefix(alias, prefix) {
	// 		str := "@" + alias
	// 		completions = append(completions, Completion{
	// 			ShownString: str,
	// 			Value:       str,
	// 			Kind:        defines.CompletionItemKindConstant,
	// 			LabelDetail: "%" + core.HOST_PATTERN.Name + " (" + string(host) + ")",
	// 		})
	// 	}
	// }

	return completions
}
