package project_server

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func getSignatureHelp(fpath string, line, column int32, handlingCtx *core.Context, session *jsonrpc.Session) (*defines.SignatureHelp, error) {

	const NO_DATA_MSG = "no data"

	state, _, chunk, cachedOrGotCache, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:         fpath,
		session:       session,
		requiresState: true,
		requiresCache: true,
	})
	if !ok {
		return &defines.SignatureHelp{}, nil
	}

	if !cachedOrGotCache && state != nil {
		//teardown in separate goroutine to return quickly
		defer func() {
			go func() {
				defer utils.Recover()
				state.Ctx.CancelGracefully()
			}()
		}()
	}

	if state == nil || state.SymbolicData == nil {
		logs.Println(NO_DATA_MSG)
		return &defines.SignatureHelp{}, nil
	}

	cursorSpan := chunk.GetLineColumnSingeCharSpan(line, column)
	node, ancestors, ok := chunk.GetNodeAndChainAtSpan(cursorSpan)

	if !ok || node == nil {
		logs.Println(NO_DATA_MSG)
		return &defines.SignatureHelp{}, nil
	}

	var (
		closestCallExpr *parse.CallExpression
		ancestorIndex   int = -1
	)

	closestCallExpr, ok = node.(*parse.CallExpression)

	if !ok { //search in ancestors
		callExpr, index, ok := parse.FindClosestMaxDistance(ancestors, (*parse.CallExpression)(nil), 2)
		if ok {
			closestCallExpr = callExpr
			ancestorIndex = index
		}
	}

	if closestCallExpr == nil {
		logs.Println(NO_DATA_MSG)
		return &defines.SignatureHelp{}, nil
	}

	callee := closestCallExpr.Callee
	if callee == nil {
		logs.Println(NO_DATA_MSG)
		return &defines.SignatureHelp{}, nil
	}

	//get signature information
	calleeValue, ok := state.SymbolicData.GetMostSpecificNodeValue(callee)
	if !ok {
		if calleeValue == nil {
			logs.Println(NO_DATA_MSG)
			return &defines.SignatureHelp{}, nil
		}
	}

	signatureInformation := defines.SignatureInformation{
		Label: symbolic.Stringify(calleeValue),
	}

	//number of parameters including the variadic parameter
	paramCount := -1

	switch val := calleeValue.(type) {
	case *symbolic.GoFunction:
		markdown, ok := help.HelpForSymbolicGoFunc(val, help.HelpMessageConfig{Format: help.MarkdownFormat})
		if ok {
			signatureInformation.Documentation = markdown
		}

		params := val.ParametersExceptCtx()
		paramCount = len(params)
		var parameterInfos []defines.ParameterInformation

		for _, param := range params {
			parameterInfos = append(parameterInfos, defines.ParameterInformation{
				Label: symbolic.Stringify(param),
			})
		}

		signatureInformation.Parameters = &parameterInfos
	case *symbolic.Function:
		goFunc, ok := val.OriginGoFunction()
		if ok {
			markdown, ok := help.HelpForSymbolicGoFunc(goFunc, help.HelpMessageConfig{Format: help.MarkdownFormat})
			if ok {
				signatureInformation.Documentation = markdown
			}
		}

		params := val.NonVariadicParameters()
		paramCount = len(params)

		var parameterInfos []defines.ParameterInformation

		for _, param := range params {
			parameterInfos = append(parameterInfos, defines.ParameterInformation{
				Label: symbolic.Stringify(param),
			})
		}

		if val.IsVariadic() {
			variadicParam := val.VariadicParamElem()
			parameterInfos = append(parameterInfos, defines.ParameterInformation{
				Label: "..." + symbolic.Stringify(variadicParam),
			})
		}

		signatureInformation.Parameters = &parameterInfos
	case *symbolic.InoxFunction:
		params := val.Parameters()
		paramCount = len(params)

		var parameterInfos []defines.ParameterInformation

		for i, param := range params {
			label := symbolic.Stringify(param)

			if val.IsVariadic() && i == len(params)-1 {
				label = "..." + label
			}

			parameterInfos = append(parameterInfos, defines.ParameterInformation{
				Label: label,
			})
		}

		signatureInformation.Parameters = &parameterInfos
	}

	zero := uint(0)
	signatureHelp := &defines.SignatureHelp{
		Signatures:      []defines.SignatureInformation{signatureInformation},
		ActiveSignature: &zero,
	}

	//find the argument node
	var argNode parse.Node

	if closestCallExpr == ancestors[len(ancestors)-1] {
		argNode = node
	} else if ancestorIndex >= 0 {
		argNode = ancestors[ancestorIndex+1]
	}

	argNodeIndex := -1
	if argNode != nil {
		for i, n := range closestCallExpr.Arguments {
			if n == argNode {
				argNodeIndex = i
				break
			}
		}
	} else if len(closestCallExpr.Arguments) > 0 { //find the argument on the left of the cursor
		for i, n := range closestCallExpr.Arguments {
			if cursorSpan.Start >= n.Base().Span.End {
				argNodeIndex = i

				// increment argNodeIndex if the cursor is after a comma located after the current argument.
				for _, token := range parse.GetTokens(closestCallExpr, chunk.Node, false) {
					if cursorSpan.End >= token.Span.End && token.Type == parse.COMMA {
						argNodeIndex++
						break
					}
				}
			}
		}
	} else {
		argNodeIndex = 0
	}

	argNodeIndex = min(argNodeIndex, paramCount-1)

	//add the active parameter index to the signature help
	if argNodeIndex >= 0 {
		index := uint(argNodeIndex)
		signatureHelp.ActiveParameter = &index
	}

	return signatureHelp, nil
}
