package projectserver

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
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

	signatureHelp, ok := getSignatureHelpAt(line, column, chunk, state)
	if !ok {
		logs.Println(NO_DATA_MSG)
		return &defines.SignatureHelp{}, nil
	}

	return signatureHelp, nil
}

func getSignatureHelpAt(line, column int32, chunk *parse.ParsedChunkSource, state *core.GlobalState) (*defines.SignatureHelp, bool) {
	if state == nil || state.SymbolicData == nil {
		return nil, false
	}

	cursorSpan := chunk.GetLineColumnSingeCharSpan(line, column)
	node, ancestors, ok := chunk.GetNodeAndChainAtSpan(cursorSpan)

	if !ok || node == nil {
		return nil, false
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
		return nil, false
	}

	callee := closestCallExpr.Callee
	if callee == nil {
		return nil, false
	}

	//get signature information
	calleeValue, ok := state.SymbolicData.GetMostSpecificNodeValue(callee)
	if !ok {
		if calleeValue == nil {
			return nil, false
		}
	}

	stringifiedCallee, stringifiedCalleeRegions := symbolic.StringifyGetRegions(calleeValue)

	signatureInformation := defines.SignatureInformation{
		Label: stringifiedCallee,
	}

	//number of parameters including the variadic parameter
	paramCount := -1

	switch val := calleeValue.(type) {
	case *symbolic.GoFunction:
		markdown, ok := help.HelpForSymbolicGoFunc(val, help.HelpMessageConfig{Format: help.MarkdownFormat})
		if ok {
			signatureInformation.Documentation = defines.MarkupContent{
				Kind:  defines.MarkupKindMarkdown,
				Value: markdown,
			}
		}

		params := val.ParametersExceptCtx()
		paramCount = len(params)
	case *symbolic.Function:
		goFunc, ok := val.OriginGoFunction()
		if ok {
			markdown, ok := help.HelpForSymbolicGoFunc(goFunc, help.HelpMessageConfig{Format: help.MarkdownFormat})
			if ok {
				signatureInformation.Documentation = defines.MarkupContent{
					Kind:  defines.MarkupKindMarkdown,
					Value: markdown,
				}
			}
		}

		params := val.NonVariadicParameters()
		paramCount = len(params)
	case *symbolic.InoxFunction:
		params := val.Parameters()
		paramCount = len(params)
	}

	//get the parameter labels

	var parameterInfos []defines.ParameterInformation

	filter := prettyprint.RegionFilter{
		ExactDepth: 0,
		Kind:       prettyprint.ParamNameTypeRegion,
	}
	stringifiedCalleeRegions.FilteredForEach(filter, func(r prettyprint.Region) error {
		parameterInfos = append(parameterInfos, defines.ParameterInformation{
			Label: r.SubString(stringifiedCallee),
		})
		return nil
	})
	signatureInformation.Parameters = &parameterInfos

	//create the signature help with signatureInformation as the only signature

	zero := uint(0)
	signatureHelp := &defines.SignatureHelp{
		Signatures:      []defines.SignatureInformation{signatureInformation},
		ActiveSignature: &zero,
	}

	//determine the active parameter
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

	return signatureHelp, true
}
