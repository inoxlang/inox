package codecompletion

import (
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/help"

	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"

	parse "github.com/inoxlang/inox/internal/parse"
)

var (
	CONTEXT_INDEPENDENT_STMT_STARTING_KEYWORDS                   = []string{"if", "drop-perms", "for", "assign", "switch", "match", "return", "assert"}
	GLOBALNAMES_WITHOUT_IDENT_CONVERSION_TO_VAR_IN_CMD_LIKE_CALL = []string{globalnames.HELP_FN}
)

// A Completion represents a single completion item.
type Completion struct {
	ShownString           string                    `json:"shownString"`
	Value                 string                    `json:"value"`
	ReplacedRange         parse.SourcePositionRange `json:"replacedRange"`
	Kind                  defines.CompletionItemKind
	LabelDetail           string
	MarkdownDocumentation string
}

// Mode informs FindCompletions about what environment the completions are intended for
// and where to get data.
type Mode int

const (
	ShellCompletions Mode = iota
	LspCompletions
)

func (m Mode) String() string {
	switch m {
	case ShellCompletions:
		return "shell-completions"
	case LspCompletions:
		return "LSP-completions"
	default:
		panic(core.ErrUnreachable)
	}
}

type SearchArgs struct {
	State       *core.TreeWalkState
	Chunk       *parse.ParsedChunkSource
	CursorIndex int
	Mode        Mode
	InputData   InputData
}

func FindCompletions(args SearchArgs) []Completion {

	state := args.State
	chunk := args.Chunk
	cursorIndex := int32(args.CursorIndex)
	mode := args.Mode

	//Determine if the cursor is inside a comment.
	isCursorInsideOrAtEndOfComment := false

	tokens := args.Chunk.Node.Tokens //never modified

	for tokenIndex, token := range tokens {
		if cursorIndex > token.Span.End || cursorIndex < token.Span.Start {
			continue
		}

		if token.Type != parse.COMMENT {
			break
		}

		isAtEndOfLine := cursorIndex >= token.Span.End && tokenIndex < len(tokens)-1 && tokens[tokenIndex+1].Type == parse.NEWLINE

		if cursorIndex < token.Span.End || isAtEndOfLine {
			isCursorInsideOrAtEndOfComment = true
		}

		break
	}

	nodeAtCursor, parent, ancestors, deepestCall := getNodeAtCursor(cursorIndex, chunk.Node)

	if nodeAtCursor == nil {
		return nil
	}

	var completions []Completion

	search := completionSearch{
		state:         state,
		chunk:         chunk,
		cursorIndex:   cursorIndex,
		mode:          mode,
		parent:        parent,
		ancestorChain: ancestors,
		inputData:     args.InputData,
	}

	if isCursorInsideOrAtEndOfComment {
		completions = handleCompletionInsideComment()
	} else {
		switch n := nodeAtCursor.(type) {
		case *parse.PatternIdentifierLiteral:
			completions = handlePatternIdentCompletions(n, search)
		case *parse.PatternNamespaceIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
			completions = handlePatternNamespaceCompletions(n, search)
		case *parse.Variable:
			completions = handleLocalVariableCompletions(n, search)
		case *parse.GlobalVariable:
			completions = handleGlobalVariableCompletions(n, search)
		case *parse.IdentifierLiteral:
			completions = handleIdentifierAndKeywordCompletions(n, deepestCall, search)
		case *parse.IdentifierMemberExpression:
			completions = handleIdentifierMemberCompletions(n, search)
		case *parse.MemberExpression:
			completions = handleMemberExpressionCompletions(n, search)
		case *parse.DoubleColonExpression:
			completions = handleDoubleColonExpressionCompletions(n, search)
		case *parse.CallExpression: //if a call is the deepest node at cursor it means we are not in an argument
			completions = handleNewCallArgumentCompletions(n, search)
		case *parse.DoubleQuotedStringLiteral:
			completions = findStringCompletions(n, search)
		case *parse.MultilineStringLiteral:
			completions = findStringCompletions(n, search)
		case *parse.RelativePathLiteral:
			completions = findPathCompletions(state.Global.Ctx, n.Raw, n, search)
		case *parse.AbsolutePathLiteral:
			completions = findPathCompletions(state.Global.Ctx, n.Raw, n, search)
		case *parse.URLLiteral:
			completions = findURLCompletions(state.Global.Ctx, n, search)
		case *parse.URLPatternLiteral:
			completions = findURLPatternCompletions(state.Global.Ctx, n, search)
		case *parse.HostLiteral:
			completions = findHostCompletions(state.Global.Ctx, n.Value, parent)
		case *parse.SchemeLiteral:
			completions = findHostCompletions(state.Global.Ctx, n.Name, parent)
		case *parse.InvalidAliasRelatedNode:
			if len(n.Raw) > 0 && !strings.Contains(n.Raw, "/") {
				completions = findHostAliasCompletions(state.Global.Ctx, n.Raw[1:], parent)
			}
		case *parse.ObjectLiteral:
			completions = findObjectInteriorCompletions(n, search)
		case *parse.RecordLiteral:
			completions = findRecordInteriorCompletions(n, search)
		case *parse.DictionaryLiteral:
			completions = findDictionaryInteriorCompletions(n, search)
		case *parse.XMLOpeningElement:
			completions = findXMLOpeningElementInteriorCompletions(n, search)
		case *parse.XMLClosingElement:
			if cursorIndex == n.Span.Start && utils.Implements[*parse.XMLElement](parent) {
				completions = findHyperscriptScriptCompletions(parent.(*parse.XMLElement), search)
			}
		case *parse.XMLElement:
			completions = findHyperscriptScriptCompletions(n, search)
		case *parse.HyperscriptAttributeShorthand:
			completions = findHyperscriptAttributeCompletions(n, search)
		}
	}

	//Set unitialized .ReplacedRange fields of completions.

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

type completionSearch struct {
	state         *core.TreeWalkState
	chunk         *parse.ParsedChunkSource
	cursorIndex   int32
	mode          Mode
	parent        parse.Node
	ancestorChain []parse.Node
	inputData     InputData
}

func handlePatternIdentCompletions(n *parse.PatternIdentifierLiteral, search completionSearch) []Completion {
	state := search.state
	ctx := state.Global.Ctx
	ancestorChain := search.ancestorChain
	chunk := search.chunk
	mode := search.mode

	var completions []Completion

	if mode == ShellCompletions {
		for name, patt := range state.Global.Ctx.GetNamedPatterns() {
			if !hasPrefixCaseInsensitive(name, n.Name) {
				continue
			}
			detail, _ := core.GetStringifiedSymbolicValue(ctx, patt, false)

			hasPercent := parse.GetFirstTokenString(n, chunk.Node)[0] == '%'
			s := name
			if hasPercent {
				s = "%" + s
			}

			completions = append(completions, Completion{
				ShownString: s,
				Value:       s,
				Kind:        defines.CompletionItemKindInterface,
				LabelDetail: detail,
			})
		}
		for name, namespace := range state.Global.Ctx.GetPatternNamespaces() {
			detail, _ := core.GetStringifiedSymbolicValue(ctx, namespace, false)

			if !hasPrefixCaseInsensitive(name, n.Name) {
				continue
			}

			hasPercent := parse.GetFirstTokenString(n, chunk.Node)[0] == '%'
			s := name
			if hasPercent {
				s = "%" + s
			}

			completions = append(completions, Completion{
				ShownString: s,
				Value:       s,
				Kind:        defines.CompletionItemKindInterface,
				LabelDetail: detail,
			})
		}
	} else {
		contextData, _ := state.Global.SymbolicData.GetContextData(n, ancestorChain)
		for _, patternData := range contextData.Patterns {
			if !hasPrefixCaseInsensitive(patternData.Name, n.Name) {
				continue
			}

			s := patternData.Name
			if !n.Unprefixed {
				s = "%" + s
			}
			completions = append(completions, Completion{
				ShownString: s,
				Value:       s,
				Kind:        defines.CompletionItemKindInterface,
				LabelDetail: symbolic.Stringify(patternData.Value),
			})
		}
		for _, namespaceData := range contextData.PatternNamespaces {
			if !hasPrefixCaseInsensitive(namespaceData.Name, n.Name) {
				continue
			}

			s := namespaceData.Name + "."
			if !n.Unprefixed {
				s = "%" + s
			}
			completions = append(completions, Completion{
				ShownString: s,
				Value:       s,
				Kind:        defines.CompletionItemKindInterface,
				LabelDetail: symbolic.Stringify(namespaceData.Value),
			})
		}
	}

	return completions
}

func handlePatternNamespaceCompletions(n parse.Node, search completionSearch) []Completion {
	state := search.state
	ctx := state.Global.Ctx
	ancestorChain := search.ancestorChain
	mode := search.mode

	var completions []Completion

	var namespaceName string
	var memberName string
	var prefixed bool

	switch node := n.(type) {
	case *parse.PatternNamespaceIdentifierLiteral:
		namespaceName = node.Name
		prefixed = !node.Unprefixed
	case *parse.PatternNamespaceMemberExpression:
		namespaceName = node.Namespace.Name
		memberName = node.MemberName.Name
		prefixed = !node.Namespace.Unprefixed
	}

	if mode == ShellCompletions {
		namespace := state.Global.Ctx.ResolvePatternNamespace(namespaceName)
		if namespace == nil {
			return nil
		}

		for patternName, patternValue := range namespace.Patterns {
			if !hasPrefixCaseInsensitive(patternName, memberName) {
				continue
			}

			s := namespaceName + "." + patternName
			if prefixed {
				s = "%" + s
			}
			detail, _ := core.GetStringifiedSymbolicValue(ctx, patternValue, false)

			completions = append(completions, Completion{
				ShownString: s,
				Value:       s,
				Kind:        defines.CompletionItemKindInterface,
				LabelDetail: detail,
			})
		}
	} else {
		contextData, _ := state.Global.SymbolicData.GetContextData(n, ancestorChain)
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
			if !hasPrefixCaseInsensitive(patternName, memberName) {
				return nil
			}

			s := namespaceName + "." + patternName
			if prefixed {
				s = "%" + s
			}
			completions = append(completions, Completion{
				ShownString: s,
				Value:       s,
				Kind:        defines.CompletionItemKindInterface,
				LabelDetail: symbolic.Stringify(patternValue),
			})

			return nil
		})
	}

	return completions
}

func handleLocalVariableCompletions(n *parse.Variable, search completionSearch) []Completion {
	state := search.state
	ctx := state.Global.Ctx
	mode := search.mode
	ancestorChain := search.ancestorChain

	var completions []Completion

	var names []string
	var labelDetails []string
	if mode == ShellCompletions {
		for name, varVal := range state.CurrentLocalScope() {

			if hasPrefixCaseInsensitive(name, n.Name) {
				names = append(names, name)

				detail, _ := core.GetStringifiedSymbolicValue(ctx, varVal, false)
				labelDetails = append(labelDetails, detail)
			}
		}
	} else {
		scopeData, _ := state.Global.SymbolicData.GetLocalScopeData(n, ancestorChain)
		for _, varData := range scopeData.Variables {
			if hasPrefixCaseInsensitive(varData.Name, n.Name) {
				names = append(names, varData.Name)

				labelDetails = append(labelDetails, symbolic.Stringify(varData.Value))
			}
		}
	}

	for i, name := range names {
		completions = append(completions, Completion{
			ShownString: name,
			Value:       "$" + name,
			Kind:        defines.CompletionItemKindVariable,
			LabelDetail: labelDetails[i],
		})
	}
	return completions
}

func handleGlobalVariableCompletions(n *parse.GlobalVariable, search completionSearch) []Completion {
	state := search.state
	ctx := state.Global.Ctx
	mode := search.mode
	ancestorChain := search.ancestorChain

	var completions []Completion

	if mode == ShellCompletions {
		state.Global.Globals.Foreach(func(name string, varVal core.Value, _ bool) error {
			if hasPrefixCaseInsensitive(name, n.Name) {
				detail, _ := core.GetStringifiedSymbolicValue(ctx, varVal, false)
				completions = append(completions, Completion{
					ShownString: name,
					Value:       "$$" + name,
					Kind:        defines.CompletionItemKindVariable,
					LabelDetail: detail,
				})
			}
			return nil
		})
	} else {
		scopeData, _ := state.Global.SymbolicData.GetGlobalScopeData(n, ancestorChain)

		for _, varData := range scopeData.Variables {
			if hasPrefixCaseInsensitive(varData.Name, n.Name) {
				completions = append(completions, Completion{
					ShownString: varData.Name,
					Value:       "$$" + varData.Name,
					Kind:        defines.CompletionItemKindVariable,
					LabelDetail: symbolic.Stringify(varData.Value),
				})
			}
		}
	}

	return completions
}

func handleIdentifierAndKeywordCompletions(ident *parse.IdentifierLiteral, deepestCall *parse.CallExpression, search completionSearch) []Completion {
	ancestors := search.ancestorChain
	state := search.state
	parent := search.parent
	mode := search.mode

	var completions []Completion

	//subcommand completions
	if deepestCall != nil && deepestCall.CommandLikeSyntax {
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
				goto after_subcommand_completions
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

after_subcommand_completions:

	ancestorCount := len(ancestors)

	isPropName := false

	//if the identifier is the name of an object's property
	if ancestorCount > 2 &&
		utils.Implements[*parse.ObjectProperty](ancestors[ancestorCount-1]) &&
		utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-2]) &&
		utils.Implements[*parse.ObjectProperty](parent) {

		isPropName = true
		completions = append(completions, findObjectPropertyNameCompletions(ident, parent.(*parse.ObjectProperty), ancestors, search)...)
	}

	//if in record
	if len(ancestors) > 2 &&
		utils.Implements[*parse.ObjectProperty](ancestors[len(ancestors)-1]) &&
		utils.Implements[*parse.RecordLiteral](ancestors[len(ancestors)-2]) {

		isPropName = true

		recordLiteral := ancestors[len(ancestors)-2].(*parse.RecordLiteral)

		properties, ok := state.Global.SymbolicData.GetAllowedNonPresentProperties(recordLiteral)
		if ok {
			for _, name := range properties {
				if hasPrefixCaseInsensitive(name, ident.Name) {
					completions = append(completions, Completion{
						ShownString: name,
						Value:       name,
						Kind:        defines.CompletionItemKindProperty,
					})
				}
			}
		}
	}

	if isPropName {
		return completions
	}

	switch p := parent.(type) {
	case *parse.XMLAttribute:
		attribute := p
		//if name
		switch {
		case ident == attribute.Name:
			completions = findXmlAttributeNameCompletions(ident, attribute, ancestors)
		}
		return completions
	case *parse.XMLOpeningElement:
		//if tag name
		switch {
		case ident == p.Name:
			completions = findXmlTagAndTagNameCompletions(ident, search)
		}
		return completions
	}

	callExpr, ok := parent.(*parse.CallExpression)
	isCommandLikeCallArgument := ok && callExpr.CommandLikeSyntax && utils.Some(callExpr.Arguments, func(e parse.Node) bool {
		return ident == e
	})

	//----- suggest variables and keywords -----

	//suggest local variables
	if mode == ShellCompletions {
		for name, varVal := range state.CurrentLocalScope() {
			if hasPrefixCaseInsensitive(name, ident.Name) {
				detail, _ := core.GetStringifiedSymbolicValue(state.Global.Ctx, varVal, false)

				if isCommandLikeCallArgument {
					name = "$" + name
				}

				completions = append(completions, Completion{
					ShownString: name,
					Value:       name,
					Kind:        defines.CompletionItemKindVariable,
					LabelDetail: detail,
				})
			}
		}
	} else {
		scopeData, _ := state.Global.SymbolicData.GetLocalScopeData(ident, ancestors)
		for _, varData := range scopeData.Variables {
			if hasPrefixCaseInsensitive(varData.Name, ident.Name) {

				name := varData.Name
				if isCommandLikeCallArgument {
					name = "$" + name
				}

				completions = append(completions, Completion{
					ShownString: name,
					Value:       name,
					Kind:        defines.CompletionItemKindVariable,
					LabelDetail: symbolic.Stringify(varData.Value),
				})
			}
		}
	}

	//suggest global variables

	if mode == ShellCompletions {

		state.Global.Globals.Foreach(func(name string, varVal core.Value, _ bool) error {
			if hasPrefixCaseInsensitive(name, ident.Name) {
				detail, _ := core.GetStringifiedSymbolicValue(state.Global.Ctx, varVal, false)

				if isCommandLikeCallArgument {
					ident, ok := callExpr.Callee.(*parse.IdentifierLiteral)
					if !ok || !slices.Contains(GLOBALNAMES_WITHOUT_IDENT_CONVERSION_TO_VAR_IN_CMD_LIKE_CALL, ident.Name) {
						name = "$$" + name
					}
				}

				completions = append(completions, Completion{
					ShownString: name,
					Value:       name,
					Kind:        defines.CompletionItemKindVariable,
					LabelDetail: detail,
				})
			}
			return nil
		})
	} else {
		scopeData, _ := state.Global.SymbolicData.GetGlobalScopeData(ident, ancestors)

		for _, varData := range scopeData.Variables {
			if hasPrefixCaseInsensitive(varData.Name, ident.Name) {

				name := varData.Name
				if isCommandLikeCallArgument {
					ident, ok := callExpr.Callee.(*parse.IdentifierLiteral)
					if !ok || !slices.Contains(GLOBALNAMES_WITHOUT_IDENT_CONVERSION_TO_VAR_IN_CMD_LIKE_CALL, ident.Name) {
						name = "$$" + name
					}
				}

				completion := Completion{
					ShownString: name,
					Value:       name,
					Kind:        defines.CompletionItemKindVariable,
					LabelDetail: symbolic.Stringify(varData.Value),
				}

				symbolicFunc, ok := varData.Value.(*symbolic.GoFunction)
				if ok {
					help, ok := help.HelpForSymbolicGoFunc(symbolicFunc, helpMessageConfig)
					if ok {
						completion.MarkdownDocumentation = help
					}
				}

				completions = append(completions, completion)
			}
		}
	}

	//suggest context-dependent keywords

	for i := len(ancestors) - 1; i >= 0; i-- {
		if parse.IsScopeContainerNode(ancestors[i]) {
			break
		}
		switch ancestors[i].(type) {
		case *parse.ForStatement:

			switch parent.(type) {
			case *parse.Block:
				for _, keyword := range []string{"break", "continue"} {
					if hasPrefixCaseInsensitive(keyword, ident.Name) {
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
				if hasPrefixCaseInsensitive("prune", ident.Name) {
					completions = append(completions, Completion{
						ShownString: "prune",
						Value:       "prune",
						Kind:        defines.CompletionItemKindKeyword,
					})
				}
			}
		}
	}

	//suggest context-independent statement-starting keywords

	switch parent.(type) {
	case *parse.Block, *parse.InitializationBlock, *parse.EmbeddedModule, *parse.Chunk:
		for _, keyword := range CONTEXT_INDEPENDENT_STMT_STARTING_KEYWORDS {

			if hasPrefixCaseInsensitive(keyword, ident.Name) {
				completions = append(completions, Completion{
					ShownString: keyword,
					Value:       keyword,
					Kind:        defines.CompletionItemKindKeyword,
				})
			}
		}
	}

	//suggest some expression-starting keywords

	for _, keyword := range []string{"treedata", "Mapping", "concat"} {
		if hasPrefixCaseInsensitive(keyword, ident.Name) {
			completions = append(completions, Completion{
				ShownString: keyword,
				Value:       keyword,
				Kind:        defines.CompletionItemKindKeyword,
			})
		}
	}

	return completions
}

func handleIdentifierMemberCompletions(n *parse.IdentifierMemberExpression, search completionSearch) []Completion {
	state := search.state
	mode := search.mode
	chunk := search.chunk

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

	isLastPropPresent := len(n.PropertyNames) > 0 && (n.Err == nil || n.Err.Kind != parse.UnterminatedMemberExpr)

	var replacedRange parse.SourcePositionRange
	if isLastPropPresent {
		replacedRange = chunk.GetSourcePosition(n.PropertyNames[len(n.PropertyNames)-1].Span)
	} else {
		replacedRange = chunk.GetSourcePosition(n.Span)
		replacedRange.StartColumn = replacedRange.EndColumn
		replacedRange.StartLine = replacedRange.EndLine
		replacedRange.Span.Start = replacedRange.Span.End
	}
	// '.'
	replacedRange.Span.Start -= 1
	replacedRange.StartColumn -= 1

	//we get the next property until we reach the last property's name
	for i, propName := range n.PropertyNames {
		var propertyNames []string
		if mode == ShellCompletions {
			propertyNames = curr.(core.IProps).PropertyNames(state.Global.Ctx)
		} else {
			// if at one point in the member chain a value is any we have no completions to propose
			// so we just return an empty list
			if symbolic.IsAny(curr.(symbolic.Value)) {
				return nil
			}
			iprops, ok := curr.(symbolic.IProps)
			// if the at one point in the member chain a value has no properties we have no completions to propose
			// so we just return an empty list
			if !ok {
				return nil
			}
			propertyNames = symbolic.GetAllPropertyNames(iprops)
		}

		found := false
		for _, name := range propertyNames {
			if name == propName.Name { //property's name is valid
				if i == len(n.PropertyNames)-1 && (n.Err == nil || n.Err.Kind != parse.UnterminatedMemberExpr) { //if last
					return nil
				}

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

	return suggestPropertyNames(curr, n.PropertyNames, isLastPropPresent, replacedRange, state.Global, mode)
}

func handleMemberExpressionCompletions(n *parse.MemberExpression, search completionSearch) []Completion {
	state := search.state
	mode := search.mode
	chunk := search.chunk

	ok := true

	var exprPropertyNames = []*parse.IdentifierLiteral{n.PropertyName}
	left := n.Left
	isLastPropPresent := n.PropertyName != nil

	var replacedRange parse.SourcePositionRange
	if isLastPropPresent {
		replacedRange = chunk.GetSourcePosition(n.PropertyName.Span)
	} else {
		replacedRange = chunk.GetSourcePosition(n.Span)
		replacedRange.StartColumn = replacedRange.EndColumn
		replacedRange.StartLine = replacedRange.EndLine
		replacedRange.Span.Start = replacedRange.Span.End
	}
	// '.'
	replacedRange.Span.Start -= 1
	replacedRange.StartColumn -= 1

	var curr any

loop:
	for {
		switch l := left.(type) {
		case *parse.MemberExpression:
			left = l.Left
			exprPropertyNames = append([]*parse.IdentifierLiteral{l.PropertyName}, exprPropertyNames...)
		case *parse.DoubleColonExpression:
			val, ok := state.Global.SymbolicData.GetMostSpecificNodeValue(l.Element)
			if !ok {
				return nil
			}
			curr = val
			break loop
		case *parse.GlobalVariable:
			break loop
		case *parse.Variable:
			break loop
		case *parse.SelfExpression:
			break loop
		default:
			return nil
		}
	}

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
	case *parse.DoubleColonExpression:
		//ok
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
			if symbolic.IsAnyOrAnySerializable(curr.(symbolic.Value)) {
				return nil
			}
			// if the at one point in the member chain a value has no properties we have no completions to propose
			// so we just return an empty list
			iprops, ok := curr.(symbolic.IProps)
			if !ok {
				return nil
			}
			propertyNames = symbolic.GetAllPropertyNames(iprops)
		}

		//we search for the property name that matches the node
		//if we find it we add '.<property name>' to the buffer
		for _, name := range propertyNames {
			if name == propNameNode.Name {
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

	return suggestPropertyNames(curr, exprPropertyNames, isLastPropPresent, replacedRange, state.Global, mode)
}

func handleDoubleColonExpressionCompletions(n *parse.DoubleColonExpression, search completionSearch) (completions []Completion) {
	state := search.state
	mode := search.mode
	chunk := search.chunk

	if mode == ShellCompletions {
		//TODO: support ?
		return
	}

	leftVal, ok := state.Global.SymbolicData.GetMostSpecificNodeValue(n.Left)
	if !ok {
		return nil
	}

	var replacedRange parse.SourcePositionRange
	if n.Element == nil {
		replacedRange = chunk.GetSourcePosition(n.Span)
		replacedRange.StartColumn = replacedRange.EndColumn
		replacedRange.StartLine = replacedRange.EndLine
		replacedRange.Span.Start = replacedRange.Span.End
	} else {
		replacedRange = chunk.GetSourcePosition(n.Element.Span)
	}

	referencedEntity, _ := state.Global.SymbolicData.GetURLReferencedEntity(n)
	if referencedEntity != nil {
		switch referencedEntity := referencedEntity.(type) {
		case symbolic.IProps:
			for _, propName := range referencedEntity.PropertyNames() {
				if n.Element != nil && !hasPrefixCaseInsensitive(propName, n.Element.Name) {
					continue
				}

				propValue := referencedEntity.Prop(propName)
				propDetail := symbolic.Stringify(propValue)

				completions = append(completions, Completion{
					ShownString:   propName,
					Value:         propName,
					Kind:          defines.CompletionItemKindProperty,
					LabelDetail:   propDetail,
					ReplacedRange: replacedRange,
				})
			}
		}
		return completions
	}

	switch l := leftVal.(type) {
	case *symbolic.Object:
		l.ForEachEntry(func(propName string, propValue symbolic.Value) error {
			if symbolic.IsAnyOrAnySerializable(propValue) || utils.Ret0(symbolic.IsSharable(propValue)) {
				return nil
			}

			propDetail := symbolic.Stringify(propValue)

			completions = append(completions, Completion{
				ShownString:   propName,
				Value:         propName,
				Kind:          defines.CompletionItemKindProperty,
				LabelDetail:   propDetail,
				ReplacedRange: replacedRange,
			})
			return nil
		})
	}

	extensions, _ := state.Global.SymbolicData.GetAvailableTypeExtensions(n)

	for _, ext := range extensions {
		for _, propExpr := range ext.PropertyExpressions {
			if n.Element == nil || hasPrefixCaseInsensitive(propExpr.Name, n.Element.Name) {
				labelDetail := ""
				var kind defines.CompletionItemKind
				if propExpr.Method == nil {
					kind = defines.CompletionItemKindProperty
					printConfig := parse.PrintConfig{}
					labelDetail = "computed property(" + parse.SPrint(propExpr.Expression, chunk.Node, printConfig) + ")"
				} else {
					kind = defines.CompletionItemKindMethod
					labelDetail = "(extension method) " + symbolic.Stringify(propExpr.Method)
				}

				completions = append(completions, Completion{
					ShownString:   propExpr.Name,
					Value:         propExpr.Name,
					Kind:          kind,
					ReplacedRange: replacedRange,
					LabelDetail:   labelDetail,
				})
			}
		}
	}

	return
}

func suggestPropertyNames(
	curr interface{}, exprPropNames []*parse.IdentifierLiteral, isLastPropPresent bool,
	replacedRange parse.SourcePositionRange, state *core.GlobalState, mode Mode,
) []Completion {
	var completions []Completion
	var propNames []string
	var propLabelDetails []string
	var optionalProps []bool
	var markdownDocumentations []string

	//we get all property names
	switch v := curr.(type) {
	case core.IProps:
		propNames = v.PropertyNames(state.Ctx)
		propLabelDetails = utils.MapSlice(propNames, func(name string) string {
			propVal := v.Prop(state.Ctx, name)
			detail, _ := core.GetStringifiedSymbolicValue(state.Ctx, propVal, false)

			//add markdown documentation if help is found.
			var markdownDocumentation string
			goFunc, ok := propVal.(*core.GoFunction)
			if ok {
				markdownDocumentation, _ = help.HelpForGoFunc(goFunc, helpMessageConfig)
			}
			markdownDocumentations = append(markdownDocumentations, markdownDocumentation)

			return detail
		})
	case symbolic.IProps:
		propNames = symbolic.GetAllPropertyNames(v)
		propLabelDetails = utils.MapSlice(propNames, func(name string) string {
			propVal := v.Prop(name)
			stringified := symbolic.Stringify(propVal)

			//add markdown documentation if help is found.
			var markdownDocumentation string
			goFunc, ok := propVal.(*symbolic.GoFunction)
			if ok {
				markdownDocumentation, _ = help.HelpForSymbolicGoFunc(goFunc, helpMessageConfig)
			}
			markdownDocumentations = append(markdownDocumentations, markdownDocumentation)

			return stringified
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
				ShownString:           op + propName,
				Value:                 op + propName,
				Kind:                  defines.CompletionItemKindProperty,
				LabelDetail:           propLabelDetails[i],
				ReplacedRange:         replacedRange,
				MarkdownDocumentation: markdownDocumentations[i],
			})
		}
	} else {
		//we suggest all property names which start with the last name in the member expression

		propNamePrefix := exprPropNames[len(exprPropNames)-1].Name

		for i, propName := range propNames {

			if !hasPrefixCaseInsensitive(propName, propNamePrefix) {
				continue
			}

			op := "."
			if len(optionalProps) != 0 && optionalProps[i] {
				op = ".?"
			}

			completions = append(completions, Completion{
				ShownString:           op + propName,
				Value:                 op + propName,
				Kind:                  defines.CompletionItemKindProperty,
				LabelDetail:           propLabelDetails[i],
				ReplacedRange:         replacedRange,
				MarkdownDocumentation: markdownDocumentations[i],
			})
		}
	}
	return completions
}

func handleNewCallArgumentCompletions(n *parse.CallExpression, search completionSearch) []Completion {
	cursorIndex := search.cursorIndex
	state := search.state
	chunk := search.chunk

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

func findObjectInteriorCompletions(n *parse.ObjectLiteral, search completionSearch) (completions []Completion) {
	chunk := search.chunk
	cursorIndex := int32(search.cursorIndex)
	ancestors := search.ancestorChain

	interiorSpan, err := parse.GetInteriorSpan(n, chunk.Node)
	if err != nil {
		return nil
	}

	if !interiorSpan.HasPositionEndIncluded(cursorIndex) {
		return nil
	}

	pos := chunk.GetSourcePosition(parse.NodeSpan{Start: cursorIndex, End: cursorIndex})

	properties, ok := search.state.Global.SymbolicData.GetAllowedNonPresentProperties(n)
	if ok {
		for _, name := range properties {
			completions = append(completions, Completion{
				ShownString:   name,
				Value:         name,
				Kind:          defines.CompletionItemKindProperty,
				ReplacedRange: pos,
			})
		}
	}

	switch parent := search.parent.(type) {
	case *parse.Manifest: //suggest sections of the manifest that are not present
	manifest_sections_loop:
		for _, sectionName := range core.MANIFEST_SECTION_NAMES {
			for _, prop := range n.Properties {
				if !prop.HasImplicitKey() && prop.Name() == sectionName {
					continue manifest_sections_loop
				}
			}

			suffix := ": "
			valueCompletion, ok := MANIFEST_SECTION_DEFAULT_VALUE_COMPLETIONS[sectionName]
			if ok {
				suffix += valueCompletion
			}

			completions = append(completions, Completion{
				ShownString:           sectionName + suffix,
				Value:                 sectionName + suffix,
				MarkdownDocumentation: MANIFEST_SECTION_DOC[sectionName],
				Kind:                  defines.CompletionItemKindVariable,
				ReplacedRange:         pos,
			})
		}
	case *parse.ImportStatement: //suggest sections of the module import config that are not present
	mod_import_sections_loop:
		for _, sectionName := range core.IMPORT_CONFIG_SECTION_NAMES {
			for _, prop := range n.Properties {
				if !prop.HasImplicitKey() && prop.Name() == sectionName {
					continue mod_import_sections_loop
				}
			}

			suffix := ": "
			valueCompletion, ok := MODULE_IMPORT_SECTION_DEFAULT_VALUE_COMPLETIONS[sectionName]
			if ok {
				suffix += valueCompletion
			}

			completions = append(completions, Completion{
				ShownString:           sectionName + suffix,
				Value:                 sectionName + suffix,
				MarkdownDocumentation: MODULE_IMPORT_SECTION_DOC[sectionName],
				Kind:                  defines.CompletionItemKindVariable,
				ReplacedRange:         pos,
			})
		}
	case *parse.SpawnExpression:
		if n != parent.Meta {
			break
		}
		//suggest sections of the lthread meta object that are not present
	lthread_meta_sections_loop:
		for _, sectionName := range symbolic.LTHREAD_SECTION_NAMES {
			for _, prop := range n.Properties {
				if !prop.HasImplicitKey() && prop.Name() == sectionName {
					continue lthread_meta_sections_loop
				}
			}

			suffix := ": "
			valueCompletion, ok := LTHREAD_META_SECTION_DEFAULT_VALUE_COMPLETIONS[sectionName]
			if ok {
				suffix += valueCompletion
			}

			completions = append(completions, Completion{
				ShownString:           sectionName + suffix,
				Value:                 sectionName + suffix,
				LabelDetail:           LTHREAD_META_SECTION_LABEL_DETAILS[sectionName],
				MarkdownDocumentation: LTHREAD_META_SECTION_DOC[sectionName],
				Kind:                  defines.CompletionItemKindVariable,
				ReplacedRange:         pos,
			})
		}
	case *parse.ObjectProperty:
		if parent.HasImplicitKey() || len(ancestors) < 3 {
			return
		}

		//allowed permissions in module import statement
		if len(ancestors) >= 5 &&
			parent.HasNameEqualTo(core.IMPORT_CONFIG__ALLOW_PROPNAME) &&
			utils.Implements[*parse.ImportStatement](ancestors[len(ancestors)-3]) {

			for _, info := range permkind.PERMISSION_KINDS {
				//ignore kinds that are already present.
				if n.HasNamedProp(info.Name) {
					continue
				}

				detail := MAJOR_PERM_KIND_TEXT

				if info.PermissionKind.IsMinor() {
					detail = MINOR_PERM_KIND_TEXT
				}

				completions = append(completions, Completion{
					ShownString:   info.Name,
					Value:         info.Name,
					Kind:          defines.CompletionItemKindVariable,
					ReplacedRange: pos,
					LabelDetail:   detail,
				})
			}
		}

		switch greatGrandParent := ancestors[len(ancestors)-3].(type) {
		case *parse.Manifest:
			switch parent.Name() {
			case core.MANIFEST_PERMS_SECTION_NAME: //permissions section
				for _, info := range permkind.PERMISSION_KINDS {
					//ignore kinds that are already present.
					if n.HasNamedProp(info.Name) {
						continue
					}

					detail := MAJOR_PERM_KIND_TEXT

					if info.PermissionKind.IsMinor() {
						detail = MINOR_PERM_KIND_TEXT
					}

					completions = append(completions, Completion{
						ShownString:   info.Name,
						Value:         info.Name,
						Kind:          defines.CompletionItemKindVariable,
						ReplacedRange: pos,
						LabelDetail:   detail,
					})
				}
			}
		default:
			_ = greatGrandParent
		}

		if len(ancestors) < 5 {
			break
		}

		manifestSectionName := ""
		var sectionProperty *parse.ObjectProperty

		ancestorCount := len(ancestors)

		if utils.Implements[*parse.Manifest](ancestors[ancestorCount-5]) &&
			utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-4]) &&
			utils.Implements[*parse.ObjectProperty](ancestors[ancestorCount-3]) &&
			ancestors[ancestorCount-3].(*parse.ObjectProperty).Key != nil {
			sectionProperty = ancestors[ancestorCount-3].(*parse.ObjectProperty)
			manifestSectionName = sectionProperty.Name()
		}

		if sectionProperty == nil || sectionProperty.Value == nil {
			break
		}

		//the cursor is located in the span of an object inside a manifest section.

		switch manifestSectionName {
		case core.MANIFEST_DATABASES_SECTION_NAME:
			//suggest database description's properties

			_, ok := sectionProperty.Value.(*parse.ObjectLiteral)
			if !ok {
				break
			}
			dbDescription := n

			for _, descPropName := range core.MANIFEST_DATABASE_PROPNAMES {
				//ignore properties that are already present.
				if dbDescription.HasNamedProp(descPropName) {
					continue
				}

				suffix := ": "
				valueCompletion, ok := MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS[descPropName]
				if ok {
					suffix += valueCompletion
				}

				completions = append(completions, Completion{
					ShownString:           descPropName + suffix,
					Value:                 descPropName + suffix,
					Kind:                  defines.CompletionItemKindVariable,
					MarkdownDocumentation: MANIFEST_DB_DESC_DOC[descPropName],
					ReplacedRange:         pos,
				})
			}
		}
	}

	return
}

func findRecordInteriorCompletions(n *parse.RecordLiteral, search completionSearch) (completions []Completion) {
	cursorIndex := int32(search.cursorIndex)
	chunk := search.chunk

	interiorSpan, err := parse.GetInteriorSpan(n, chunk.Node)
	if err != nil {
		return nil
	}

	if !interiorSpan.HasPositionEndIncluded(cursorIndex) {
		return nil
	}

	pos := chunk.GetSourcePosition(parse.NodeSpan{Start: cursorIndex, End: cursorIndex})

	properties, ok := search.state.Global.SymbolicData.GetAllowedNonPresentProperties(n)
	if ok {
		for _, name := range properties {
			completions = append(completions, Completion{
				ShownString:   name,
				Value:         name,
				Kind:          defines.CompletionItemKindProperty,
				ReplacedRange: pos,
			})
		}
	}
	return
}

func findDictionaryInteriorCompletions(n *parse.DictionaryLiteral, search completionSearch) (completions []Completion) {
	cursorIndex := int32(search.cursorIndex)
	chunk := search.chunk

	interiorSpan, err := parse.GetInteriorSpan(n, chunk.Node)
	if err != nil {
		return nil
	}

	if !interiorSpan.HasPositionEndIncluded(cursorIndex) {
		return nil
	}

	pos := chunk.GetSourcePosition(parse.NodeSpan{Start: cursorIndex, End: cursorIndex})

	properties, ok := search.state.Global.SymbolicData.GetAllowedNonPresentKeys(n)
	if ok {
		for _, name := range properties {
			completions = append(completions, Completion{
				ShownString:   name,
				Value:         name,
				Kind:          defines.CompletionItemKindProperty,
				ReplacedRange: pos,
			})
		}
	}

	return
}

func findStringCompletions(strLit parse.SimpleValueLiteral, search completionSearch) (completions []Completion) {

	switch strLit.(type) {
	case *parse.DoubleQuotedStringLiteral, *parse.MultilineStringLiteral:
	default:
		return nil
	}

	// in attribute
	if attribute, ok := search.parent.(*parse.XMLAttribute); ok {
		switch {
		case strLit == attribute.Value:
			completions = findXMLAttributeValueCompletions(strLit, attribute, search)
		}

		return completions
	}

	return
}

func hasPrefixCaseInsensitive(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
}

func getNodeAtCursor(cursorIndex int32, chunk *parse.Chunk) (nodeAtCursor, _parent parse.Node, ancestors []parse.Node, deepestCall *parse.CallExpression) {
	//search node at cursor
	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, _ bool) (parse.TraversalAction, error) {
		span := node.Base().Span

		//if the cursor is not in the node's span we don't check the descendants of the node
		if span.Start > cursorIndex || span.End < cursorIndex {
			return parse.Prune, nil
		}

		if nodeAtCursor == nil || node.Base().IncludedIn(nodeAtCursor) {
			nodeAtCursor = node

			switch p := parent.(type) {
			case *parse.MemberExpression, *parse.IdentifierMemberExpression:
				nodeAtCursor = parent
				if len(ancestorChain) > 1 {
					_parent = ancestorChain[len(ancestorChain)-2]
				}
				ancestors = slices.Clone(ancestorChain[:len(ancestorChain)-1])
			case *parse.DoubleColonExpression:
				if nodeAtCursor == p.Element {
					nodeAtCursor = parent
					if len(ancestorChain) > 1 {
						_parent = ancestorChain[len(ancestorChain)-2]
					}
					ancestors = slices.Clone(ancestorChain[:len(ancestorChain)-1])
				}
			case *parse.PatternNamespaceMemberExpression:
				nodeAtCursor = parent
				if len(ancestorChain) > 1 {
					_parent = ancestorChain[len(ancestorChain)-2]
				}
				ancestors = slices.Clone(ancestorChain[:len(ancestorChain)-1])
			default:
				_parent = parent
				ancestors = slices.Clone(ancestorChain)
			}

			switch n := nodeAtCursor.(type) {
			case *parse.CallExpression:
				deepestCall = n
			}
		}

		return parse.ContinueTraversal, nil
	}, nil)

	return
}
