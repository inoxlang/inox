package internal

import (
	"bytes"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/permkind"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"

	parse "github.com/inoxlang/inox/internal/parse"
)

type Completion struct {
	ShownString   string                    `json:"shownString"`
	Value         string                    `json:"value"`
	ReplacedRange parse.SourcePositionRange `json:"replacedRange"`
	Kind          defines.CompletionItemKind
	Detail        string
}

const (
	MAJOR_PERM_KIND_TEXT = "major permission kind"
	MINOR_PERM_KIND_TEXT = "minor permission kind"
)

var (
	CONTEXT_INDEPENDENT_STMT_STARTING_KEYWORDS = []string{"if", "drop-perms", "for", "assign", "switch", "match", "return", "assert"}
)

type CompletionSearchArgs struct {
	State       *core.TreeWalkState
	Chunk       *parse.ParsedChunk
	CursorIndex int
	Mode        CompletionMode
}

type CompletionMode int

const (
	ShellCompletions CompletionMode = iota
	LspCompletions
)

func (m CompletionMode) String() string {
	switch m {
	case ShellCompletions:
		return "shell-completions"
	case LspCompletions:
		return "LSP-completions"
	default:
		panic(core.ErrUnreachable)
	}
}

func FindCompletions(args CompletionSearchArgs) []Completion {

	state := args.State
	chunk := args.Chunk
	cursorIndex := args.CursorIndex
	mode := args.Mode

	var completions []Completion
	var nodeAtCursor parse.Node
	var _parent parse.Node
	var deepestCall *parse.CallExpression
	var _ancestorChain []parse.Node

	parse.Walk(chunk.Node, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, _ bool) (parse.TraversalAction, error) {
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

	ctx := state.Global.Ctx

	switch n := nodeAtCursor.(type) {
	case *parse.PatternIdentifierLiteral:
		if mode == ShellCompletions {
			for name, patt := range state.Global.Ctx.GetNamedPatterns() {
				if !strings.HasPrefix(name, n.Name) {
					continue
				}
				detail, _ := core.GetStringifiedSymbolicValue(ctx, patt, false)

				s := "%" + name
				completions = append(completions, Completion{
					ShownString: s,
					Value:       s,
					Kind:        defines.CompletionItemKindInterface,
					Detail:      detail,
				})
			}
			for name, namespace := range state.Global.Ctx.GetPatternNamespaces() {
				detail, _ := core.GetStringifiedSymbolicValue(ctx, namespace, false)

				if !strings.HasPrefix(name, n.Name) {
					continue
				}
				s := "%" + name + "."
				completions = append(completions, Completion{
					ShownString: s,
					Value:       s,
					Kind:        defines.CompletionItemKindInterface,
					Detail:      detail,
				})
			}
		} else {
			contextData, _ := state.Global.SymbolicData.GetContextData(n, _ancestorChain)
			for _, patternData := range contextData.Patterns {
				if !strings.HasPrefix(patternData.Name, n.Name) {
					continue
				}

				s := "%" + patternData.Name
				completions = append(completions, Completion{
					ShownString: s,
					Value:       s,
					Kind:        defines.CompletionItemKindInterface,
					Detail:      symbolic.Stringify(patternData.Value),
				})
			}
			for _, namespaceData := range contextData.PatternNamespaces {
				if !strings.HasPrefix(namespaceData.Name, n.Name) {
					continue
				}

				s := "%" + namespaceData.Name + "."
				completions = append(completions, Completion{
					ShownString: s,
					Value:       s,
					Kind:        defines.CompletionItemKindInterface,
					Detail:      symbolic.Stringify(namespaceData.Value),
				})
			}
		}
	case *parse.PatternNamespaceIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
		var namespaceName string
		var memberName string

		switch node := n.(type) {
		case *parse.PatternNamespaceIdentifierLiteral:
			namespaceName = node.Name
		case *parse.PatternNamespaceMemberExpression:
			namespaceName = node.Namespace.Name
			memberName = node.MemberName.Name
		}

		if mode == ShellCompletions {
			namespace := state.Global.Ctx.ResolvePatternNamespace(namespaceName)
			if namespace == nil {
				return nil
			}

			for patternName, patternValue := range namespace.Patterns {
				if !strings.HasPrefix(patternName, memberName) {
					continue
				}

				s := "%" + namespaceName + "." + patternName
				detail, _ := core.GetStringifiedSymbolicValue(ctx, patternValue, false)

				completions = append(completions, Completion{
					ShownString: s,
					Value:       s,
					Kind:        defines.CompletionItemKindInterface,
					Detail:      detail,
				})
			}
		} else {
			contextData, _ := state.Global.SymbolicData.GetContextData(n, _ancestorChain)
			var namespace *symbolic.PatternNamespace
			for _, namespaceData := range contextData.PatternNamespaces {
				if namespaceData.Name == namespaceName {
					namespace = namespaceData.Value
					break
				}
			}
			if namespace == nil {
				return nil
			}

			namespace.ForEachPattern(func(patternName string, patternValue symbolic.Pattern) error {
				if !strings.HasPrefix(patternName, memberName) {
					return nil
				}

				s := "%" + namespaceName + "." + patternName

				completions = append(completions, Completion{
					ShownString: s,
					Value:       s,
					Kind:        defines.CompletionItemKindInterface,
					Detail:      symbolic.Stringify(patternValue),
				})

				return nil
			})
		}
	case *parse.Variable:
		var names []string
		var details []string
		if args.Mode == ShellCompletions {
			for name, varVal := range state.CurrentLocalScope() {

				if strings.HasPrefix(name, n.Name) {
					names = append(names, name)

					detail, _ := core.GetStringifiedSymbolicValue(ctx, varVal, false)
					details = append(details, detail)
				}
			}
		} else {
			scopeData, _ := state.Global.SymbolicData.GetLocalScopeData(n, _ancestorChain)
			for _, varData := range scopeData.Variables {
				if strings.HasPrefix(varData.Name, n.Name) {
					names = append(names, varData.Name)

					details = append(details, symbolic.Stringify(varData.Value))
				}
			}
		}

		for i, name := range names {
			completions = append(completions, Completion{
				ShownString: name,
				Value:       "$" + name,
				Kind:        defines.CompletionItemKindVariable,
				Detail:      details[i],
			})
		}
	case *parse.GlobalVariable:
		if mode == ShellCompletions {
			state.Global.Globals.Foreach(func(name string, varVal core.Value, _ bool) error {
				if strings.HasPrefix(name, n.Name) {
					detail, _ := core.GetStringifiedSymbolicValue(ctx, varVal, false)
					completions = append(completions, Completion{
						ShownString: name,
						Value:       "$$" + name,
						Kind:        defines.CompletionItemKindVariable,
						Detail:      detail,
					})
				}
				return nil
			})
		} else {
			scopeData, _ := state.Global.SymbolicData.GetGlobalScopeData(n, _ancestorChain)

			for _, varData := range scopeData.Variables {
				if strings.HasPrefix(varData.Name, n.Name) {
					completions = append(completions, Completion{
						ShownString: varData.Name,
						Value:       "$$" + varData.Name,
						Kind:        defines.CompletionItemKindVariable,
						Detail:      symbolic.Stringify(varData.Value),
					})
				}
			}
		}

	case *parse.IdentifierLiteral:
		completions = handleIdentifierAndKeywordCompletions(mode, n, deepestCall, _ancestorChain, _parent, int32(cursorIndex), chunk, state)
	case *parse.IdentifierMemberExpression:
		completions = handleIdentifierMemberCompletions(n, state, mode)
	case *parse.MemberExpression:
		completions = handleMemberExpressionCompletions(n, state, mode)
	case *parse.CallExpression: //if a call is the deepest node at cursor it means we are not in an argument
		completions = handleNewCallArgumentCompletions(n, cursorIndex, state, chunk)
	case *parse.RelativePathLiteral:
		completions = findPathCompletions(state.Global.Ctx, n.Raw)
	case *parse.AbsolutePathLiteral:
		completions = findPathCompletions(state.Global.Ctx, n.Raw)
	case *parse.URLLiteral:
		completions = findURLCompletions(state.Global.Ctx, core.URL(n.Value), _parent)
	case *parse.HostLiteral:
		completions = findHostCompletions(state.Global.Ctx, n.Value, _parent)
	case *parse.SchemeLiteral:
		completions = findHostCompletions(state.Global.Ctx, n.Name, _parent)

	case *parse.ObjectLiteral:
		completions = findObjectInteriorCompletions(n, _ancestorChain, _parent, int32(cursorIndex), chunk)
	}

	for i, completion := range completions {
		if completion.ReplacedRange.Span == (parse.NodeSpan{}) {
			span := nodeAtCursor.Base().Span
			completion.ReplacedRange = chunk.GetSourcePosition(span)
		}
		if completion.Kind == 0 {
			completion.Kind = defines.CompletionItemKindText
		}
		completions[i] = completion
	}

	return completions
}

func handleIdentifierAndKeywordCompletions(
	mode CompletionMode, ident *parse.IdentifierLiteral, deepestCall *parse.CallExpression,
	ancestors []parse.Node, parent parse.Node, cursorIndex int32, chunk *parse.ParsedChunk, state *core.TreeWalkState,
) []Completion {

	var completions []Completion

	if deepestCall != nil { //subcommand completions
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

			completionSet := make(map[Completion]bool)

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
				completion := Completion{
					ShownString: subcommandName,
					Value:       subcommandName,
					Kind:        defines.CompletionItemKindEnum,
				}
				if !completionSet[completion] {
					completions = append(completions, completion)
					completionSet[completion] = true
				}
			}
		}
	}

	//if in object
	if len(ancestors) > 2 &&
		parse.NodeIs(ancestors[len(ancestors)-1], (*parse.ObjectProperty)(nil)) &&
		parse.NodeIs(ancestors[len(ancestors)-2], (*parse.ObjectLiteral)(nil)) {

		//suggest sections of the manifest
		if parse.NodeIs(ancestors[len(ancestors)-3], (*parse.Manifest)(nil)) {
			for _, sectionName := range core.MANIFEST_SECTION_NAMES {
				if strings.HasPrefix(sectionName, ident.Name) {
					completions = append(completions, Completion{
						ShownString: sectionName,
						Value:       sectionName,
						Kind:        defines.CompletionItemKindVariable,
					})
				}
			}
			return completions
		}

		switch parent.(type) {
		case *parse.Manifest: //suggest all sections of the manifest
			for _, sectionName := range core.MANIFEST_SECTION_NAMES {
				completions = append(completions, Completion{
					ShownString: sectionName,
					Value:       sectionName,
					Kind:        defines.CompletionItemKindVariable,
				})
			}
		case *parse.ObjectProperty:

			//check if the current property is in an object describing one of the section of the manifest
			if len(ancestors) < 6 || !(parse.NodeIs(ancestors[len(ancestors)-2], (*parse.ObjectLiteral)(nil)) &&
				parse.NodeIs(ancestors[len(ancestors)-3], (*parse.ObjectProperty)(nil)) &&
				parse.NodeIs(ancestors[len(ancestors)-5], (*parse.Manifest)(nil))) {
				break
			}

			manifestObjProp := ancestors[len(ancestors)-3].(*parse.ObjectProperty)

			if manifestObjProp.HasImplicitKey() {
				break
			}

			switch manifestObjProp.Name() {
			case core.MANIFEST_PERMS_SECTION_NAME:
				for _, info := range permkind.PERMISSION_KINDS {
					if !strings.HasPrefix(info.Name, ident.Name) {
						continue
					}

					detail := MAJOR_PERM_KIND_TEXT

					if info.PermissionKind.IsMinor() {
						detail = MINOR_PERM_KIND_TEXT
					}

					completions = append(completions, Completion{
						ShownString: info.Name,
						Value:       info.Name,
						Kind:        defines.CompletionItemKindVariable,
						Detail:      detail,
					})
				}
			}

		}

	}

	//suggest local variables

	if mode == ShellCompletions {
		for name, varVal := range state.CurrentLocalScope() {
			if strings.HasPrefix(name, ident.Name) {
				detail, _ := core.GetStringifiedSymbolicValue(state.Global.Ctx, varVal, false)

				completions = append(completions, Completion{
					ShownString: name,
					Value:       name,
					Kind:        defines.CompletionItemKindVariable,
					Detail:      detail,
				})
			}
		}
	} else {
		scopeData, _ := state.Global.SymbolicData.GetLocalScopeData(ident, ancestors)
		for _, varData := range scopeData.Variables {
			if strings.HasPrefix(varData.Name, ident.Name) {
				completions = append(completions, Completion{
					ShownString: varData.Name,
					Value:       varData.Name,
					Kind:        defines.CompletionItemKindVariable,
					Detail:      symbolic.Stringify(varData.Value),
				})
			}
		}
	}

	//suggest global variables

	if mode == ShellCompletions {

		state.Global.Globals.Foreach(func(name string, varVal core.Value, _ bool) error {
			if strings.HasPrefix(name, ident.Name) {
				detail, _ := core.GetStringifiedSymbolicValue(state.Global.Ctx, varVal, false)

				completions = append(completions, Completion{
					ShownString: name,
					Value:       name,
					Kind:        defines.CompletionItemKindVariable,
					Detail:      detail,
				})
			}
			return nil
		})
	} else {
		scopeData, _ := state.Global.SymbolicData.GetGlobalScopeData(ident, ancestors)

		for _, varData := range scopeData.Variables {
			if strings.HasPrefix(varData.Name, ident.Name) {
				completions = append(completions, Completion{
					ShownString: varData.Name,
					Value:       varData.Name,
					Kind:        defines.CompletionItemKindVariable,
					Detail:      symbolic.Stringify(varData.Value),
				})
			}
		}
	}

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
						completions = append(completions, Completion{
							ShownString: keyword,
							Value:       keyword,
							Kind:        defines.CompletionItemKindKeyword,
						})
					}
				}
			}
		case *parse.WalkStatement:

			switch parent.(type) {
			case *parse.Block:
				if strings.HasPrefix("prune", ident.Name) {
					completions = append(completions, Completion{
						ShownString: "prune",
						Value:       "prune",
						Kind:        defines.CompletionItemKindKeyword,
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
				completions = append(completions, Completion{
					ShownString: keyword,
					Value:       keyword,
					Kind:        defines.CompletionItemKindKeyword,
				})
			}
		}
	}

	//suggest some keywords starting expressions

	for _, keyword := range []string{"udata", "Mapping", "concat"} {
		if strings.HasPrefix(keyword, ident.Name) {
			completions = append(completions, Completion{
				ShownString: keyword,
				Value:       keyword,
				Kind:        defines.CompletionItemKindKeyword,
			})
		}
	}

	return completions
}

func handleIdentifierMemberCompletions(n *parse.IdentifierMemberExpression, state *core.TreeWalkState, mode CompletionMode) []Completion {

	var buff *bytes.Buffer
	var curr any
	var ok bool

	if mode == ShellCompletions {
		curr, ok = state.Get(n.Left.Name)
	} else {
		curr, ok = state.Global.SymbolicData.GetMostSpecificNodeValue(n.Left)
	}

	if !ok {
		return nil
	}

	buff = bytes.NewBufferString(n.Left.Name)

	isLastPropPresent := len(n.PropertyNames) > 0 && (n.Err == nil || n.Err.Kind() != parse.UnterminatedMemberExpr)

	//we get the next property until we reach the last property's name
	for i, propName := range n.PropertyNames {
		var propertyNames []string
		if mode == ShellCompletions {
			propertyNames = curr.(core.IProps).PropertyNames(state.Global.Ctx)
		} else {
			// if the at one point in the member chain a value is any we have no completions to propose
			// so we just return an empty list
			if symbolic.IsAny(curr.(symbolic.SymbolicValue)) {
				return nil
			}
			propertyNames = symbolic.GetAllPropertyNames(curr.(symbolic.IProps))
		}

		found := false
		for _, name := range propertyNames {
			if name == propName.Name { //property's name is valid
				if i == len(n.PropertyNames)-1 && (n.Err == nil || n.Err.Kind() != parse.UnterminatedMemberExpr) { //if last
					return nil
				}
				buff.WriteRune('.')
				buff.WriteString(propName.Name)

				switch iprops := curr.(type) {
				case core.IProps:
					curr = iprops.Prop(state.Global.Ctx, name)
				case symbolic.IProps:
					curr = iprops.Prop(name)
				default:
					panic(core.ErrUnreachable)
				}
				found = true
				break
			}
		}

		if !found && i < len(n.PropertyNames)-1 { //if not last
			return nil
		}
	}

	s := buff.String()

	return suggestPropertyNames(s, curr, n.PropertyNames, isLastPropPresent, state.Global, mode)
}

func handleMemberExpressionCompletions(n *parse.MemberExpression, state *core.TreeWalkState, mode CompletionMode) []Completion {
	ok := true
	buff := bytes.NewBufferString("")

	var exprPropertyNames = []*parse.IdentifierLiteral{n.PropertyName}
	left := n.Left
	isLastPropPresent := n.PropertyName != nil

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
		case *parse.SelfExpression:
			buff.WriteString("self")
			break loop
		default:
			return nil
		}
	}
	var curr any

	switch left := left.(type) {
	case *parse.GlobalVariable:
		if mode == ShellCompletions {
			if curr, ok = state.Global.Globals.CheckedGet(left.Name); !ok {
				return nil
			}
		} else {
			if curr, ok = state.Global.SymbolicData.GetMostSpecificNodeValue(left); !ok {
				return nil
			}
		}
	case *parse.Variable:
		if mode == ShellCompletions {
			if curr, ok = state.Get(left.Name); !ok {
				return nil
			}
		} else {
			if curr, ok = state.Global.SymbolicData.GetMostSpecificNodeValue(left); !ok {
				return nil
			}
		}
	case *parse.SelfExpression:
		if mode == ShellCompletions {
			//TODO
			return nil
		} else {
			if curr, ok = state.Global.SymbolicData.GetMostSpecificNodeValue(left); !ok {
				return nil
			}
		}
	default:
		panic(core.ErrUnreachable)
	}

	for i, propNameNode := range exprPropertyNames {
		if propNameNode == nil { //unterminated member expression
			break
		}
		found := false

		var propertyNames []string
		if mode == ShellCompletions {
			propertyNames = curr.(core.IProps).PropertyNames(state.Global.Ctx)
		} else {
			// if the at one point in the member chain a value is any we have no completions to propose
			// so we just return an empty list
			if symbolic.IsAny(curr.(symbolic.SymbolicValue)) {
				return nil
			}
			propertyNames = symbolic.GetAllPropertyNames(curr.(symbolic.IProps))
		}

		//we search for the property name that matches the node
		//if we find it we add '.<property name>' to the buffer
		for _, name := range propertyNames {
			if name == propNameNode.Name {
				buff.WriteRune('.')
				buff.WriteString(propNameNode.Name)

				switch iprops := curr.(type) {
				case core.IProps:
					curr = iprops.Prop(state.Global.Ctx, name)
				case symbolic.IProps:
					curr = iprops.Prop(name)
				default:
					panic(core.ErrUnreachable)
				}
				found = true
				break
			}
		}

		if !found && i < len(exprPropertyNames)-1 { //if not last
			return nil
		}
	}

	return suggestPropertyNames(buff.String(), curr, exprPropertyNames, isLastPropPresent, state.Global, mode)
}

func suggestPropertyNames(
	s string, curr interface{}, exprPropNames []*parse.IdentifierLiteral, isLastPropPresent bool,
	state *core.GlobalState, mode CompletionMode,
) []Completion {
	var completions []Completion
	var propNames []string
	var propDetails []string
	var optionalProps []bool

	//we get all property names
	switch v := curr.(type) {
	case core.IProps:
		propNames = v.PropertyNames(state.Ctx)
		propDetails = utils.MapSlice(propNames, func(name string) string {
			propVal := v.Prop(state.Ctx, name)
			detail, _ := core.GetStringifiedSymbolicValue(state.Ctx, propVal, false)
			return detail
		})
	case symbolic.IProps:
		propNames = symbolic.GetAllPropertyNames(v)
		propDetails = utils.MapSlice(propNames, func(name string) string {
			propVal := v.Prop(name)
			return symbolic.Stringify(propVal)
		})
		optionalProps = utils.MapSlice(propNames, func(name string) bool {
			return symbolic.IsPropertyOptional(v, name)
		})
	}

	if !isLastPropPresent {
		//we suggest all property names

		for i, propName := range propNames {
			op := "."
			if len(optionalProps) != 0 && optionalProps[i] {
				op = ".?"
			}

			completions = append(completions, Completion{
				ShownString: s + op + propName,
				Value:       s + op + propName,
				Kind:        defines.CompletionItemKindProperty,
				Detail:      propDetails[i],
			})
		}
	} else {
		//we suggest all property names which start with the last name in the member expression

		propNamePrefix := exprPropNames[len(exprPropNames)-1].Name

		for i, propName := range propNames {

			if !strings.HasPrefix(propName, propNamePrefix) {
				continue
			}

			op := "."
			if len(optionalProps) != 0 && optionalProps[i] {
				op = ".?"
			}

			completions = append(completions, Completion{
				ShownString: s + op + propName,
				Value:       s + op + propName,
				Kind:        defines.CompletionItemKindProperty,
				Detail:      propDetails[i],
			})
		}
	}
	return completions
}

func handleNewCallArgumentCompletions(n *parse.CallExpression, cursorIndex int, state *core.TreeWalkState, chunk *parse.ParsedChunk) []Completion {
	var completions []Completion
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

	completionSet := make(map[Completion]bool)

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
			span := parse.NodeSpan{Start: int32(cursorIndex), End: int32(cursorIndex + 1)}

			completion := Completion{
				ShownString:   name,
				Value:         name,
				ReplacedRange: chunk.GetSourcePosition(span),
				Kind:          defines.CompletionItemKindEnum,
			}
			if !completionSet[completion] {
				completions = append(completions, completion)
				completionSet[completion] = true
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
		span := parse.NodeSpan{Start: int32(cursorIndex), End: int32(cursorIndex + 1)}

		completion := Completion{
			ShownString:   subcommandName,
			Value:         subcommandName,
			ReplacedRange: chunk.GetSourcePosition(span),
			Kind:          defines.CompletionItemKindEnum,
		}
		if !completionSet[completion] {
			completions = append(completions, completion)
			completionSet[completion] = true
		}
	}
	return completions
}

func findObjectInteriorCompletions(n *parse.ObjectLiteral, ancestors []parse.Node, parent parse.Node, cursorIndex int32, chunk *parse.ParsedChunk) (completions []Completion) {
	interiorSpan, err := parse.GetInteriorSpan(n)
	if err != nil {
		return nil
	}

	if !interiorSpan.HasPositionEndIncluded(cursorIndex) {
		return nil
	}

	pos := chunk.GetSourcePosition(parse.NodeSpan{Start: cursorIndex, End: cursorIndex})

	switch parent := parent.(type) {
	case *parse.Manifest: //suggest all sections of the manifest
		for _, sectionName := range core.MANIFEST_SECTION_NAMES {
			completions = append(completions, Completion{
				ShownString:   sectionName,
				Value:         sectionName,
				Kind:          defines.CompletionItemKindVariable,
				ReplacedRange: pos,
			})
		}
	case *parse.ObjectProperty:
		if parent.HasImplicitKey() || len(ancestors) < 3 {
			return nil
		}

		//grandParent := ancestors[len(ancestors)-2]

		switch greatGrandParent := ancestors[len(ancestors)-3].(type) {
		case *parse.Manifest:
			switch parent.Name() {
			case core.MANIFEST_PERMS_SECTION_NAME: //permissions section
				for _, info := range permkind.PERMISSION_KINDS {
					detail := MAJOR_PERM_KIND_TEXT

					if info.PermissionKind.IsMinor() {
						detail = MINOR_PERM_KIND_TEXT
					}

					completions = append(completions, Completion{
						ShownString:   info.Name,
						Value:         info.Name,
						Kind:          defines.CompletionItemKindVariable,
						ReplacedRange: pos,
						Detail:        detail,
					})
				}
			}
		default:
			_ = greatGrandParent
		}

	}

	return
}
