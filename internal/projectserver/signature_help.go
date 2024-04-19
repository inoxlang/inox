package projectserver

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

type signatureHelpParams struct {
	fpath        string
	line, column int32
	session      *jsonrpc.Session

	project         *project.Project
	lspFilesystem   *Filesystem
	chunkCache      *parse.ChunkCache
	memberAuthToken string
}

func getSignatureHelp(handlingCtx *core.Context, params signatureHelpParams) (*defines.SignatureHelp, error) {
	const NO_DATA_MSG = "no data"

	fpath, line, column, rpcSession, memberAuthToken := params.fpath, params.line, params.column, params.session, params.memberAuthToken

	preparationResult, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:              fpath,
		requiresState:      true,
		requiresCache:      true,
		alwaysForcePrepare: true,

		rpcSession:      rpcSession,
		project:         params.project,
		lspFilesystem:   params.lspFilesystem,
		inoxChunkCache:  params.chunkCache,
		memberAuthToken: memberAuthToken,
	})

	if !ok {
		return &defines.SignatureHelp{}, nil
	}

	state := preparationResult.state
	chunk := preparationResult.chunk
	cachedOrGotCache := preparationResult.cachedOrGotCache

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
		rpcSession.LoggerPrintln(NO_DATA_MSG)
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
		callExprIndex   int = -1
	)

	closestCallExpr, ok = node.(*parse.CallExpression)

	if !ok { //search in ancestors
		callExpr, index, ok := parse.FindClosestMaxDistance(ancestors, (*parse.CallExpression)(nil), 2)
		if ok {
			closestCallExpr = callExpr
			callExprIndex = index
		}
	}

	if closestCallExpr == nil {
		return nil, false
	}

	callee := closestCallExpr.Callee
	if callee == nil {
		return nil, false
	}

	//Get signature information.
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

		params := val.Parameters()
		paramCount = len(params)
	case *symbolic.InoxFunction:
		params := val.Parameters()
		paramCount = len(params)
	}

	//Get the parameter labels.

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

	//Create the signature help with signatureInformation as the only signature.

	zero := uint(0)
	signatureHelp := &defines.SignatureHelp{
		Signatures:      []defines.SignatureInformation{signatureInformation},
		ActiveSignature: &zero,
	}

	//Determine the active parameter.

	activeParamIndex := parse.DetermineActiveParameterIndex(cursorSpan, node, closestCallExpr, callExprIndex, ancestors)
	activeParamIndex = min(activeParamIndex, paramCount-1)

	//Add the active parameter index to the signature help.
	if activeParamIndex >= 0 {
		index := uint(activeParamIndex)
		signatureHelp.ActiveParameter = &index
	}

	return signatureHelp, true
}
