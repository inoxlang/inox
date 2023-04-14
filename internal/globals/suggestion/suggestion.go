package internal

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"

	core "github.com/inox-project/inox/internal/core"
	_fs "github.com/inox-project/inox/internal/globals/fs"
	_s3 "github.com/inox-project/inox/internal/globals/s3"
	"github.com/inox-project/inox/internal/utils"

	parse "github.com/inox-project/inox/internal/parse"
)

type Suggestion struct {
	ShownString string         `json:"shownString"`
	Value       string         `json:"value"`
	Span        parse.NodeSpan `json:"span"`
}

var (
	CONTEXT_INDEPENDENT_STMT_STARTING_KEYWORDS = []string{"if", "drop-perms", "for", "assign", "switch", "match", "return", "assert"}
)

func FindSuggestions(state *core.TreeWalkState, chunk *parse.Chunk, cursorIndex int) []Suggestion {

	var suggestions []Suggestion
	var nodeAtCursor parse.Node
	var _parent parse.Node
	var deepestCall *parse.CallExpression
	var _ancestorChain []parse.Node

	//TODO: move following logic to parse package

	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, _ bool) (parse.TraversalAction, error) {
		span := node.Base().Span

		//if the cursor is not in the node's span we don't check the descendants of the node
		if int(span.Start) > cursorIndex || int(span.End) < cursorIndex {
			return parse.Prune, nil
		}

		if nodeAtCursor == nil || node.Base().IncludedIn(nodeAtCursor) {
			nodeAtCursor = node

			switch parent.(type) {
			case *parse.MemberExpression, *parse.IdentifierMemberExpression:
				nodeAtCursor = parent
				if len(ancestorChain) > 1 {
					_parent = ancestorChain[len(ancestorChain)-2]
				}
				_ancestorChain = utils.CopySlice(ancestorChain[:len(ancestorChain)-1])
			case *parse.PatternNamespaceMemberExpression:
				nodeAtCursor = parent
				if len(ancestorChain) > 1 {
					_parent = ancestorChain[len(ancestorChain)-2]
				}
				_ancestorChain = utils.CopySlice(ancestorChain[:len(ancestorChain)-1])
			default:
				_parent = parent
				_ancestorChain = utils.CopySlice(ancestorChain)
			}

			switch n := nodeAtCursor.(type) {
			case *parse.CallExpression:
				deepestCall = n
			}

		}

		return parse.Continue, nil
	}, nil)

	if nodeAtCursor == nil {
		return nil
	}

	switch n := nodeAtCursor.(type) {
	case *parse.PatternIdentifierLiteral:
		for name := range state.Global.Ctx.GetNamedPatterns() {
			if strings.HasPrefix(name, n.Name) {
				s := "%" + name
				suggestions = append(suggestions, Suggestion{
					ShownString: s,
					Value:       s,
				})
			}
		}
		for name := range state.Global.Ctx.GetPatternNamespaces() {
			if strings.HasPrefix(name, n.Name) {
				s := "%" + name + "."
				suggestions = append(suggestions, Suggestion{
					ShownString: s,
					Value:       s,
				})
			}
		}
	case *parse.PatternNamespaceIdentifierLiteral:
		namespace := state.Global.Ctx.ResolvePatternNamespace(n.Name)
		if namespace == nil {
			return nil
		}
		for patternName := range namespace.Patterns {
			s := "%" + n.Name + "." + patternName

			suggestions = append(suggestions, Suggestion{
				ShownString: s,
				Value:       s,
			})
		}
	case *parse.PatternNamespaceMemberExpression:
		namespace := state.Global.Ctx.ResolvePatternNamespace(n.Namespace.Name)
		if namespace == nil {
			return nil
		}
		for patternName := range namespace.Patterns {
			if strings.HasPrefix(patternName, n.MemberName.Name) {
				s := "%" + n.Namespace.Name + "." + patternName

				suggestions = append(suggestions, Suggestion{
					ShownString: s,
					Value:       s,
				})
			}
		}
	case *parse.Variable:
		for name := range state.CurrentLocalScope() {
			if strings.HasPrefix(name, n.Name) {
				suggestions = append(suggestions, Suggestion{
					ShownString: name,
					Value:       "$" + name,
				})
			}
		}
	case *parse.GlobalVariable:
		state.Global.Globals.Foreach(func(name string, _ core.Value) {
			if strings.HasPrefix(name, n.Name) {
				suggestions = append(suggestions, Suggestion{
					ShownString: name,
					Value:       "$$" + name,
				})
			}
		})
	case *parse.IdentifierLiteral:
		suggestions = handleIdentifierAndKeywordSuggestions(n, deepestCall, _ancestorChain, state)
	case *parse.IdentifierMemberExpression:
		suggestions = handleIdentifierMemberSuggestions(n, state)
	case *parse.MemberExpression:
		suggestions = handleMemberExpressionSuggestions(n, state)
	case *parse.CallExpression: //if a call is the deepest node at cursor it means we are not in an argument
		suggestions = handleNewCallArgumentSuggestions(n, cursorIndex, state)
	case *parse.RelativePathLiteral:
		suggestions = findPathSuggestions(state.Global.Ctx, n.Raw)
	case *parse.AbsolutePathLiteral:
		suggestions = findPathSuggestions(state.Global.Ctx, n.Raw)
	case *parse.URLLiteral:
		suggestions = findURLSuggestions(state.Global.Ctx, core.URL(n.Value), _parent)
	case *parse.HostLiteral:
		suggestions = findHostSuggestions(state.Global.Ctx, n.Value, _parent)
	case *parse.SchemeLiteral:
		suggestions = findHostSuggestions(state.Global.Ctx, n.Name, _parent)
	}

	for i, suggestion := range suggestions {
		if suggestion.Span == (parse.NodeSpan{}) {
			suggestion.Span = nodeAtCursor.Base().Span
		}
		suggestions[i] = suggestion
	}

	return suggestions
}

func handleIdentifierAndKeywordSuggestions(ident *parse.IdentifierLiteral, deepestCall *parse.CallExpression, ancestors []parse.Node, state *core.TreeWalkState) []Suggestion {

	var suggestions []Suggestion

	if deepestCall != nil { //subcommand suggestions
		argIndex := -1

		for i, arg := range deepestCall.Arguments {
			if core.SamePointer(ident, arg) {
				argIndex = i
				break
			}
		}

		if argIndex >= 0 {
			calleeIdent, ok := deepestCall.Callee.(*parse.IdentifierLiteral)
			if !ok {
				return nil
			}

			subcommandIdentChain := make([]*parse.IdentifierLiteral, 0)
			for _, arg := range deepestCall.Arguments {
				idnt, ok := arg.(*parse.IdentifierLiteral)
				if !ok {
					break
				}
				subcommandIdentChain = append(subcommandIdentChain, idnt)
			}

			suggestionSet := make(map[Suggestion]bool)

			for _, perm := range state.Global.Ctx.GetGrantedPermissions() {
				cmdPerm, ok := perm.(core.CommandPermission)
				if !ok ||
					cmdPerm.CommandName.UnderlyingString() != calleeIdent.Name ||
					len(subcommandIdentChain) > len(cmdPerm.SubcommandNameChain) ||
					len(cmdPerm.SubcommandNameChain) == 0 ||
					!strings.HasPrefix(cmdPerm.SubcommandNameChain[argIndex], ident.Name) {
					continue
				}

				subcommandName := cmdPerm.SubcommandNameChain[argIndex]
				suggestion := Suggestion{
					ShownString: subcommandName,
					Value:       subcommandName,
				}
				if !suggestionSet[suggestion] {
					suggestions = append(suggestions, suggestion)
					suggestionSet[suggestion] = true
				}
			}
		}
	}

	//suggest global & local variables

	state.Global.Globals.Foreach(func(name string, _ core.Value) {
		if strings.HasPrefix(name, ident.Name) {
			suggestions = append(suggestions, Suggestion{
				ShownString: name,
				Value:       name,
			})
		}
	})

	for name := range state.CurrentLocalScope() {
		if strings.HasPrefix(name, ident.Name) {
			suggestions = append(suggestions, Suggestion{
				ShownString: name,
				Value:       name,
			})
		}
	}

	parent := ancestors[len(ancestors)-1]

	//suggest context dependent keywords

	for i := len(ancestors) - 1; i >= 0; i-- {
		if parse.IsScopeContainerNode(ancestors[i]) {
			break
		}
		switch ancestors[i].(type) {
		case *parse.ForStatement:

			switch parent.(type) {
			case *parse.Block:
				for _, keyword := range []string{"break", "continue"} {
					if strings.HasPrefix(keyword, ident.Name) {
						suggestions = append(suggestions, Suggestion{
							ShownString: keyword,
							Value:       keyword,
						})
					}
				}
			}
		case *parse.WalkStatement:

			switch parent.(type) {
			case *parse.Block:
				if strings.HasPrefix("prune", ident.Name) {
					suggestions = append(suggestions, Suggestion{
						ShownString: "prune",
						Value:       "prune",
					})
				}
			}
		}
	}

	//suggest context independent keywords starting statements

	for _, keyword := range CONTEXT_INDEPENDENT_STMT_STARTING_KEYWORDS {

		if strings.HasPrefix(keyword, ident.Name) {
			switch parent.(type) {
			case *parse.Block, *parse.InitializationBlock, *parse.EmbeddedModule, *parse.Chunk:
				suggestions = append(suggestions, Suggestion{
					ShownString: keyword,
					Value:       keyword,
				})
			}
		}
	}

	//suggest some keywords starting expressions

	for _, keyword := range []string{"udata", "Mapping", "concat"} {
		if strings.HasPrefix(keyword, ident.Name) {
			suggestions = append(suggestions, Suggestion{
				ShownString: keyword,
				Value:       keyword,
			})
		}
	}

	return suggestions
}

func handleIdentifierMemberSuggestions(n *parse.IdentifierMemberExpression, state *core.TreeWalkState) []Suggestion {

	curr, ok := state.Get(n.Left.Name)
	if !ok {
		return nil
	}

	buff := bytes.NewBufferString(n.Left.Name)

	//we get the next property until we reach the last property's name
	for i, propName := range n.PropertyNames {
		iprops, ok := curr.(core.IProps)
		if !ok {
			return nil
		}

		found := false
		for _, name := range iprops.PropertyNames(state.Global.Ctx) {
			if name == propName.Name {
				if i == len(n.PropertyNames)-1 { //if last
					return nil
				}
				buff.WriteRune('.')
				buff.WriteString(propName.Name)
				curr = iprops.Prop(state.Global.Ctx, name)
				found = true
				break
			}
		}

		if !found && i < len(n.PropertyNames)-1 { //if not last
			return nil
		}
	}

	s := buff.String()

	return suggestPropertyNames(s, curr, n.PropertyNames, state.Global)
}

func handleMemberExpressionSuggestions(n *parse.MemberExpression, state *core.TreeWalkState) []Suggestion {
	ok := true
	buff := bytes.NewBufferString("")

	var exprPropertyNames = []*parse.IdentifierLiteral{n.PropertyName}
	left := n.Left

loop:
	for {
		switch l := left.(type) {
		case *parse.MemberExpression:
			left = l.Left
			exprPropertyNames = append([]*parse.IdentifierLiteral{l.PropertyName}, exprPropertyNames...)
		case *parse.GlobalVariable:
			buff.WriteString(l.Str())
			break loop
		case *parse.Variable:
			buff.WriteString(l.Str())
			break loop
		default:
			return nil
		}
	}
	var curr core.Value

	switch left := left.(type) {
	case *parse.GlobalVariable:
		if curr, ok = state.Global.Globals.CheckedGet(left.Name); !ok {
			return nil
		}
	case *parse.Variable:
		if curr, ok = state.Get(left.Name); !ok {
			return nil
		}
	}

	for i, propName := range exprPropertyNames {
		if propName == nil {
			break
		}
		iprops, ok := curr.(core.IProps)
		if !ok {
			return nil
		}
		found := false
		for _, name := range iprops.PropertyNames(state.Global.Ctx) {
			if name == propName.Name {
				buff.WriteRune('.')
				buff.WriteString(propName.Name)
				curr = iprops.Prop(state.Global.Ctx, name)
				found = true
				break
			}
		}
		if !found && i < len(exprPropertyNames)-1 { //if not last
			return nil
		}
	}

	return suggestPropertyNames(buff.String(), curr, exprPropertyNames, state.Global)
}

func suggestPropertyNames(s string, curr interface{}, exprPropertyNames []*parse.IdentifierLiteral, state *core.GlobalState) []Suggestion {
	var suggestions []Suggestion
	var propNames []string

	//we get all property names
	switch v := curr.(type) {
	case core.IProps:
		propNames = v.PropertyNames(state.Ctx)
	}

	isLastPropPresent := len(exprPropertyNames) > 0 && exprPropertyNames[len(exprPropertyNames)-1] != nil

	if !isLastPropPresent {
		//we suggest all property names

		for _, propName := range propNames {
			suggestions = append(suggestions, Suggestion{
				ShownString: s + "." + propName,
				Value:       s + "." + propName,
			})
		}
	} else {
		//we suggest all property names which start with the last name in the member expression

		propNamePrefix := exprPropertyNames[len(exprPropertyNames)-1].Name

		for _, propName := range propNames {

			if !strings.HasPrefix(propName, propNamePrefix) {
				continue
			}

			suggestions = append(suggestions, Suggestion{
				ShownString: s + "." + propName,
				Value:       s + "." + propName,
			})
		}
	}
	return suggestions
}

func handleNewCallArgumentSuggestions(n *parse.CallExpression, cursorIndex int, state *core.TreeWalkState) []Suggestion {
	var suggestions []Suggestion
	calleeIdent, ok := n.Callee.(*parse.IdentifierLiteral)
	if !ok {
		return nil
	}

	subcommandIdentChain := make([]*parse.IdentifierLiteral, 0)
	for _, arg := range n.Arguments {
		idnt, ok := arg.(*parse.IdentifierLiteral)
		if !ok {
			break
		}
		subcommandIdentChain = append(subcommandIdentChain, idnt)
	}

	suggestionSet := make(map[Suggestion]bool)

top_loop:
	for _, perm := range state.Global.Ctx.GetGrantedPermissions() {
		cmdPerm, ok := perm.(core.CommandPermission)
		if !ok ||
			cmdPerm.CommandName.UnderlyingString() != calleeIdent.Name ||
			len(subcommandIdentChain) >= len(cmdPerm.SubcommandNameChain) ||
			len(cmdPerm.SubcommandNameChain) == 0 {
			continue
		}

		if len(subcommandIdentChain) == 0 {
			name := cmdPerm.SubcommandNameChain[0]
			suggestion := Suggestion{
				ShownString: name,
				Value:       name,
				Span:        parse.NodeSpan{Start: int32(cursorIndex), End: int32(cursorIndex + 1)},
			}
			if !suggestionSet[suggestion] {
				suggestions = append(suggestions, suggestion)
				suggestionSet[suggestion] = true
			}
			continue
		}

		holeIndex := -1
		identIndex := 0

		for i, name := range cmdPerm.SubcommandNameChain {
			if name != subcommandIdentChain[identIndex].Name {
				if holeIndex >= 0 {
					continue top_loop
				}
				holeIndex = i
			} else {
				if identIndex == len(subcommandIdentChain)-1 {
					if holeIndex < 0 {
						holeIndex = i + 1
					}
					break
				}
				identIndex++
			}
		}
		subcommandName := cmdPerm.SubcommandNameChain[holeIndex]
		suggestion := Suggestion{
			ShownString: subcommandName,
			Value:       subcommandName,
			Span:        parse.NodeSpan{Start: int32(cursorIndex), End: int32(cursorIndex + 1)},
		}
		if !suggestionSet[suggestion] {
			suggestions = append(suggestions, suggestion)
			suggestionSet[suggestion] = true
		}
	}
	return suggestions
}

func findPathSuggestions(ctx *core.Context, pth string) []Suggestion {
	var suggestions []Suggestion

	dir := path.Dir(pth)
	base := path.Base(pth)

	if core.Path(pth).IsDirPath() {
		base = ""
	}

	entries, err := _fs.ListFiles(ctx, core.Path(dir+"/"))
	if err != nil {
		return nil
	}

	for _, e := range entries {
		name := string(e.Name)
		if strings.HasPrefix(name, base) {
			pth := path.Join(dir, name)

			if !parse.HasPathLikeStart(pth) {
				pth = "./" + pth
			}

			stat, _ := os.Stat(pth)
			if stat.IsDir() {
				pth += "/"
			}

			suggestions = append(suggestions, Suggestion{
				ShownString: name,
				Value:       pth,
			})
		}
	}

	return suggestions
}

func findURLSuggestions(ctx *core.Context, u core.URL, parent parse.Node) []Suggestion {
	var suggestions []Suggestion

	urlString := string(u)

	if call, ok := parent.(*parse.CallExpression); ok {

		var S3_FNS = []string{"get", "delete", "ls"}

		if memb, ok := call.Callee.(*parse.IdentifierMemberExpression); ok &&
			memb.Left.Name == "s3" &&
			len(memb.PropertyNames) == 1 &&
			utils.SliceContains(S3_FNS, memb.PropertyNames[0].Name) &&
			strings.Contains(urlString, "/") {

			objects, err := _s3.S3List(ctx, u)
			if err == nil {
				prefix := urlString[:strings.LastIndex(urlString, "/")+1]
				for _, obj := range objects {

					val := prefix + filepath.Base(obj.Key)
					if strings.HasSuffix(obj.Key, "/") {
						val += "/"
					}

					suggestions = append(suggestions, Suggestion{
						ShownString: obj.Key,
						Value:       val,
					})
				}
			}
		}
	}

	return suggestions
}

func findHostSuggestions(ctx *core.Context, prefix string, parent parse.Node) []Suggestion {
	var suggestions []Suggestion

	allData := ctx.GetAllHostResolutionData()

	for host := range allData {
		hostStr := string(host)
		if strings.HasPrefix(hostStr, prefix) {
			suggestions = append(suggestions, Suggestion{
				ShownString: hostStr,
				Value:       hostStr,
			})
		}
	}

	{ //localhost
		scheme, realHost, ok := strings.Cut(prefix, "://")

		var schemes = []string{"http", "https", "file", "ws", "wss"}

		if ok && utils.SliceContains(schemes, scheme) && len(realHost) > 0 && strings.HasPrefix("localhost", realHost) {
			s := strings.Replace(prefix, realHost, "localhost", 1)
			suggestions = append(suggestions, Suggestion{
				ShownString: s,
				Value:       s,
			})
		}

	}

	return suggestions
}
