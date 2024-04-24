package codecompletion

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

const GO_WRAPPER_FUNCTION_NAME_SUFFIX = "-fm"

func handleCallArgumentCompletions(callNode *parse.CallExpression, search completionSearch) (completions []Completion) {

	if callNode.CommandLikeSyntax {
		completions = append(completions, handleCommandLikeCallArgumentCompletions(callNode, search)...)
	}

	symbolicData := search.state.Global.SymbolicData
	if symbolicData == nil {
		return
	}

	symbolicCallee, ok := symbolicData.GetMostSpecificNodeValue(callNode.Callee)
	if !ok {
		return
	}

	var methodContainer symbolic.Value //can be nil

	switch calleeNode := callNode.Callee.(type) {
	case *parse.IdentifierMemberExpression:
		if len(calleeNode.PropertyNames) == 1 {
			methodContainer, _ = symbolicData.GetMostSpecificNodeValue(calleeNode.Left)
		} else {
			methodContainer, _ = symbolicData.GetMostSpecificNodeValue(calleeNode.PropertyNames[len(calleeNode.PropertyNames)-2])
		}
	case *parse.MemberExpression:
		methodContainer, _ = symbolicData.GetMostSpecificNodeValue(calleeNode.Left)
	}

	abstractFn, ok := symbolicCallee.(*symbolic.Function)
	if ok {
		goFn, ok := abstractFn.OriginGoFunction()
		if ok {
			symbolicCallee = goFn
		}
	}

	//Determine the index of the parameter.

	cursorSpan := parse.NodeSpan{Start: search.cursorIndex, End: search.cursorIndex}
	nodeAtSpan := callNode
	paramIndex := parse.DetermineActiveParameterIndex(cursorSpan, nodeAtSpan, callNode, -1, search.ancestorChain)

	//Suggest completions.

	switch fn := symbolicCallee.(type) {
	case *symbolic.InoxFunction:
	case *symbolic.GoFunction:
		var parametersExceptCtx []symbolic.Value
		if abstractFn != nil {
			parametersExceptCtx = abstractFn.Parameters()
		} else {
			parametersExceptCtx = fn.ParametersExceptCtx()
		}

		funcName := getNormalizedGoFuncName(fn)

		_ = funcName
		_ = methodContainer
		_ = parametersExceptCtx
		_ = paramIndex

		//switch funcName {
		//case getFuncName((*symbolic.DatabaseIL).UpdateSchema):
		//}
	}

	return
}

func handleCommandLikeCallArgumentCompletions(n *parse.CallExpression, search completionSearch) []Completion {
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

// getNormalizedGoFuncName returns the name of the passed function without the -fm suffix.
func getNormalizedGoFuncName(fn any) string {
	funcName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	isClosureOrMethod := strings.HasSuffix(funcName, GO_WRAPPER_FUNCTION_NAME_SUFFIX)

	if isClosureOrMethod {
		//Remove suffix
		funcName = strings.TrimSuffix(funcName, GO_WRAPPER_FUNCTION_NAME_SUFFIX)
	}
	return funcName
}
