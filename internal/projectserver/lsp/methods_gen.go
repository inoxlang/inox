// code gen by methods_gen_test.go, do not edit!
package lsp

import (
	"context"

	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

type Methods struct {
	Opt                                        Config
	onInitialize                               func(ctx context.Context, req *defines.InitializeParams) (*defines.InitializeResult, *defines.InitializeError)
	onInitialized                              func(ctx context.Context, req *defines.InitializeParams) error
	onShutdown                                 func(ctx context.Context, req *defines.NoParams) error
	onExit                                     func(ctx context.Context, req *defines.NoParams) error
	onDidChangeConfiguration                   func(ctx context.Context, req *defines.DidChangeConfigurationParams) error
	onDidChangeWatchedFiles                    func(ctx context.Context, req *defines.DidChangeWatchedFilesParams) error
	onDidOpenTextDocument                      func(ctx context.Context, req *defines.DidOpenTextDocumentParams) error
	onDidChangeTextDocument                    func(ctx context.Context, req *defines.DidChangeTextDocumentParams) error
	onDidCloseTextDocument                     func(ctx context.Context, req *defines.DidCloseTextDocumentParams) error
	onWillSaveTextDocument                     func(ctx context.Context, req *defines.WillSaveTextDocumentParams) error
	onDidSaveTextDocument                      func(ctx context.Context, req *defines.DidSaveTextDocumentParams) error
	onExecuteCommand                           func(ctx context.Context, req *defines.ExecuteCommandParams) error
	onHover                                    func(ctx context.Context, req *defines.HoverParams) (*defines.Hover, error)
	onCompletion                               func(ctx context.Context, req *defines.CompletionParams) (*[]defines.CompletionItem, error)
	onCompletionResolve                        func(ctx context.Context, req *defines.CompletionItem) (*defines.CompletionItem, error)
	onSignatureHelp                            func(ctx context.Context, req *defines.SignatureHelpParams) (*defines.SignatureHelp, error)
	onDeclaration                              func(ctx context.Context, req *defines.DeclarationParams) (*[]defines.LocationLink, error)
	onDefinition                               func(ctx context.Context, req *defines.DefinitionParams) (*[]defines.LocationLink, error)
	onTypeDefinition                           func(ctx context.Context, req *defines.TypeDefinitionParams) (*[]defines.LocationLink, error)
	onImplementation                           func(ctx context.Context, req *defines.ImplementationParams) (*[]defines.LocationLink, error)
	onReferences                               func(ctx context.Context, req *defines.ReferenceParams) (*[]defines.Location, error)
	onDocumentHighlight                        func(ctx context.Context, req *defines.DocumentHighlightParams) (*[]defines.DocumentHighlight, error)
	onDocumentSymbolWithSliceDocumentSymbol    func(ctx context.Context, req *defines.DocumentSymbolParams) (*[]defines.DocumentSymbol, error)
	onDocumentSymbolWithSliceSymbolInformation func(ctx context.Context, req *defines.DocumentSymbolParams) (*[]defines.SymbolInformation, error)
	onWorkspaceSymbol                          func(ctx context.Context, req *defines.WorkspaceSymbolParams) (*[]defines.SymbolInformation, error)
	onCodeActionWithSliceCommand               func(ctx context.Context, req *defines.CodeActionParams) (*[]defines.Command, error)
	onCodeActionWithSliceCodeAction            func(ctx context.Context, req *defines.CodeActionParams) (*[]defines.CodeAction, error)
	onCodeActionResolve                        func(ctx context.Context, req *defines.CodeAction) (*defines.CodeAction, error)
	onCodeLens                                 func(ctx context.Context, req *defines.CodeLensParams) (*[]defines.CodeLens, error)
	onCodeLensResolve                          func(ctx context.Context, req *defines.CodeLens) (*defines.CodeLens, error)
	onDocumentFormatting                       func(ctx context.Context, req *defines.DocumentFormattingParams) (*[]defines.TextEdit, error)
	onDocumentRangeFormatting                  func(ctx context.Context, req *defines.DocumentRangeFormattingParams) (*[]defines.TextEdit, error)
	onDocumentOnTypeFormatting                 func(ctx context.Context, req *defines.DocumentOnTypeFormattingParams) (*[]defines.TextEdit, error)
	onRenameRequest                            func(ctx context.Context, req *defines.RenameParams) (*defines.WorkspaceEdit, error)
	onPrepareRename                            func(ctx context.Context, req *defines.PrepareRenameParams) (*defines.Range, error)
	onDocumentLinks                            func(ctx context.Context, req *defines.DocumentLinkParams) (*[]defines.DocumentLink, error)
	onDocumentLinkResolve                      func(ctx context.Context, req *defines.DocumentLink) (*defines.DocumentLink, error)
	onDocumentColor                            func(ctx context.Context, req *defines.DocumentColorParams) (*[]defines.ColorInformation, error)
	onColorPresentation                        func(ctx context.Context, req *defines.ColorPresentationParams) (*[]defines.ColorPresentation, error)
	onFoldingRanges                            func(ctx context.Context, req *defines.FoldingRangeParams) (*[]defines.FoldingRange, error)
	onSelectionRanges                          func(ctx context.Context, req *defines.SelectionRangeParams) (*[]defines.SelectionRange, error)
}

func (m *Methods) OnInitialize(f func(ctx context.Context, req *defines.InitializeParams) (result *defines.InitializeResult, err *defines.InitializeError)) {
	m.onInitialize = f
}

func (m *Methods) initialize(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.InitializeParams)
	if m.onInitialize != nil {
		res, err := m.onInitialize(ctx, params)
		e := wrapErrorToRespError(err, 1)
		return res, e
	}

	res, err := m.builtinInitialize(ctx, params)
	e := wrapErrorToRespError(err, 1)
	return res, e

}

func (m *Methods) initializeMethodInfo() *jsonrpc.MethodInfo {

	return &jsonrpc.MethodInfo{
		Name: "initialize",
		NewRequest: func() interface{} {
			return &defines.InitializeParams{}
		},
		Handler:    m.initialize,
		RateLimits: []int{},
	}
}

func (m *Methods) OnInitialized(f func(ctx context.Context, req *defines.InitializeParams) (err error)) {
	m.onInitialized = f
}

func (m *Methods) initialized(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.InitializeParams)
	if m.onInitialized != nil {
		err := m.onInitialized(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) initializedMethodInfo() *jsonrpc.MethodInfo {

	if m.onInitialized == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "initialized",
		NewRequest: func() interface{} {
			return &defines.InitializeParams{}
		},
		Handler:    m.initialized,
		RateLimits: []int{},
	}
}

func (m *Methods) OnShutdown(f func(ctx context.Context, req *defines.NoParams) (err error)) {
	m.onShutdown = f
}

func (m *Methods) shutdown(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.NoParams)
	if m.onShutdown != nil {
		err := m.onShutdown(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) shutdownMethodInfo() *jsonrpc.MethodInfo {

	if m.onShutdown == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "shutdown",
		NewRequest: func() interface{} {
			return &defines.NoParams{}
		},
		Handler:    m.shutdown,
		RateLimits: []int{},
	}
}

func (m *Methods) OnExit(f func(ctx context.Context, req *defines.NoParams) (err error)) {
	m.onExit = f
}

func (m *Methods) exit(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.NoParams)
	if m.onExit != nil {
		err := m.onExit(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) exitMethodInfo() *jsonrpc.MethodInfo {

	if m.onExit == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "exit",
		NewRequest: func() interface{} {
			return &defines.NoParams{}
		},
		Handler:    m.exit,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDidChangeConfiguration(f func(ctx context.Context, req *defines.DidChangeConfigurationParams) (err error)) {
	m.onDidChangeConfiguration = f
}

func (m *Methods) didChangeConfiguration(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DidChangeConfigurationParams)
	if m.onDidChangeConfiguration != nil {
		err := m.onDidChangeConfiguration(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) didChangeConfigurationMethodInfo() *jsonrpc.MethodInfo {

	if m.onDidChangeConfiguration == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "workspace/didChangeConfiguration",
		NewRequest: func() interface{} {
			return &defines.DidChangeConfigurationParams{}
		},
		Handler:    m.didChangeConfiguration,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDidChangeWatchedFiles(f func(ctx context.Context, req *defines.DidChangeWatchedFilesParams) (err error)) {
	m.onDidChangeWatchedFiles = f
}

func (m *Methods) didChangeWatchedFiles(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DidChangeWatchedFilesParams)
	if m.onDidChangeWatchedFiles != nil {
		err := m.onDidChangeWatchedFiles(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) didChangeWatchedFilesMethodInfo() *jsonrpc.MethodInfo {

	if m.onDidChangeWatchedFiles == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "workspace/didChangeWatchedFiles",
		NewRequest: func() interface{} {
			return &defines.DidChangeWatchedFilesParams{}
		},
		Handler:    m.didChangeWatchedFiles,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDidOpenTextDocument(f func(ctx context.Context, req *defines.DidOpenTextDocumentParams) (err error)) {
	m.onDidOpenTextDocument = f
}

func (m *Methods) didOpenTextDocument(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DidOpenTextDocumentParams)
	if m.onDidOpenTextDocument != nil {
		err := m.onDidOpenTextDocument(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) didOpenTextDocumentMethodInfo() *jsonrpc.MethodInfo {

	if m.onDidOpenTextDocument == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/didOpen",
		NewRequest: func() interface{} {
			return &defines.DidOpenTextDocumentParams{}
		},
		Handler:    m.didOpenTextDocument,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDidChangeTextDocument(f func(ctx context.Context, req *defines.DidChangeTextDocumentParams) (err error)) {
	m.onDidChangeTextDocument = f
}

func (m *Methods) didChangeTextDocument(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DidChangeTextDocumentParams)
	if m.onDidChangeTextDocument != nil {
		err := m.onDidChangeTextDocument(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) didChangeTextDocumentMethodInfo() *jsonrpc.MethodInfo {

	if m.onDidChangeTextDocument == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/didChange",
		NewRequest: func() interface{} {
			return &defines.DidChangeTextDocumentParams{}
		},
		Handler:    m.didChangeTextDocument,
		RateLimits: []int{20, 50, 300},
	}
}

func (m *Methods) OnDidCloseTextDocument(f func(ctx context.Context, req *defines.DidCloseTextDocumentParams) (err error)) {
	m.onDidCloseTextDocument = f
}

func (m *Methods) didCloseTextDocument(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DidCloseTextDocumentParams)
	if m.onDidCloseTextDocument != nil {
		err := m.onDidCloseTextDocument(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) didCloseTextDocumentMethodInfo() *jsonrpc.MethodInfo {

	if m.onDidCloseTextDocument == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/didClose",
		NewRequest: func() interface{} {
			return &defines.DidCloseTextDocumentParams{}
		},
		Handler:    m.didCloseTextDocument,
		RateLimits: []int{},
	}
}

func (m *Methods) OnWillSaveTextDocument(f func(ctx context.Context, req *defines.WillSaveTextDocumentParams) (err error)) {
	m.onWillSaveTextDocument = f
}

func (m *Methods) willSaveTextDocument(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.WillSaveTextDocumentParams)
	if m.onWillSaveTextDocument != nil {
		err := m.onWillSaveTextDocument(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) willSaveTextDocumentMethodInfo() *jsonrpc.MethodInfo {

	if m.onWillSaveTextDocument == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/willSave",
		NewRequest: func() interface{} {
			return &defines.WillSaveTextDocumentParams{}
		},
		Handler:    m.willSaveTextDocument,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDidSaveTextDocument(f func(ctx context.Context, req *defines.DidSaveTextDocumentParams) (err error)) {
	m.onDidSaveTextDocument = f
}

func (m *Methods) didSaveTextDocument(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DidSaveTextDocumentParams)
	if m.onDidSaveTextDocument != nil {
		err := m.onDidSaveTextDocument(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) didSaveTextDocumentMethodInfo() *jsonrpc.MethodInfo {

	if m.onDidSaveTextDocument == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/didSave",
		NewRequest: func() interface{} {
			return &defines.DidSaveTextDocumentParams{}
		},
		Handler:    m.didSaveTextDocument,
		RateLimits: []int{},
	}
}

func (m *Methods) OnExecuteCommand(f func(ctx context.Context, req *defines.ExecuteCommandParams) (err error)) {
	m.onExecuteCommand = f
}

func (m *Methods) executeCommand(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.ExecuteCommandParams)
	if m.onExecuteCommand != nil {
		err := m.onExecuteCommand(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return nil, e
	}
	return nil, nil
}

func (m *Methods) executeCommandMethodInfo() *jsonrpc.MethodInfo {

	if m.onExecuteCommand == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "executeCommand",
		NewRequest: func() interface{} {
			return &defines.ExecuteCommandParams{}
		},
		Handler:    m.executeCommand,
		RateLimits: []int{},
	}
}

func (m *Methods) OnHover(f func(ctx context.Context, req *defines.HoverParams) (result *defines.Hover, err error)) {
	m.onHover = f
}

func (m *Methods) hover(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.HoverParams)
	if m.onHover != nil {
		res, err := m.onHover(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) hoverMethodInfo() *jsonrpc.MethodInfo {

	if m.onHover == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/hover",
		NewRequest: func() interface{} {
			return &defines.HoverParams{}
		},
		Handler:    m.hover,
		RateLimits: []int{},
	}
}

func (m *Methods) OnCompletion(f func(ctx context.Context, req *defines.CompletionParams) (result *[]defines.CompletionItem, err error)) {
	m.onCompletion = f
}

func (m *Methods) completion(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.CompletionParams)
	if m.onCompletion != nil {
		res, err := m.onCompletion(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) completionMethodInfo() *jsonrpc.MethodInfo {

	if m.onCompletion == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/completion",
		NewRequest: func() interface{} {
			return &defines.CompletionParams{}
		},
		Handler:    m.completion,
		RateLimits: []int{10, 30, 100},
	}
}

func (m *Methods) OnCompletionResolve(f func(ctx context.Context, req *defines.CompletionItem) (result *defines.CompletionItem, err error)) {
	m.onCompletionResolve = f
}

func (m *Methods) completionResolve(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.CompletionItem)
	if m.onCompletionResolve != nil {
		res, err := m.onCompletionResolve(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) completionResolveMethodInfo() *jsonrpc.MethodInfo {

	if m.onCompletionResolve == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "completionItem/resolve",
		NewRequest: func() interface{} {
			return &defines.CompletionItem{}
		},
		Handler:    m.completionResolve,
		RateLimits: []int{},
	}
}

func (m *Methods) OnSignatureHelp(f func(ctx context.Context, req *defines.SignatureHelpParams) (result *defines.SignatureHelp, err error)) {
	m.onSignatureHelp = f
}

func (m *Methods) signatureHelp(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.SignatureHelpParams)
	if m.onSignatureHelp != nil {
		res, err := m.onSignatureHelp(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) signatureHelpMethodInfo() *jsonrpc.MethodInfo {

	if m.onSignatureHelp == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/signatureHelp",
		NewRequest: func() interface{} {
			return &defines.SignatureHelpParams{}
		},
		Handler:    m.signatureHelp,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDeclaration(f func(ctx context.Context, req *defines.DeclarationParams) (result *[]defines.LocationLink, err error)) {
	m.onDeclaration = f
}

func (m *Methods) declaration(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DeclarationParams)
	if m.onDeclaration != nil {
		res, err := m.onDeclaration(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) declarationMethodInfo() *jsonrpc.MethodInfo {

	if m.onDeclaration == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/declaration",
		NewRequest: func() interface{} {
			return &defines.DeclarationParams{}
		},
		Handler:    m.declaration,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDefinition(f func(ctx context.Context, req *defines.DefinitionParams) (result *[]defines.LocationLink, err error)) {
	m.onDefinition = f
}

func (m *Methods) definition(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DefinitionParams)
	if m.onDefinition != nil {
		res, err := m.onDefinition(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) definitionMethodInfo() *jsonrpc.MethodInfo {

	if m.onDefinition == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/definition",
		NewRequest: func() interface{} {
			return &defines.DefinitionParams{}
		},
		Handler:    m.definition,
		RateLimits: []int{},
	}
}

func (m *Methods) OnTypeDefinition(f func(ctx context.Context, req *defines.TypeDefinitionParams) (result *[]defines.LocationLink, err error)) {
	m.onTypeDefinition = f
}

func (m *Methods) typeDefinition(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.TypeDefinitionParams)
	if m.onTypeDefinition != nil {
		res, err := m.onTypeDefinition(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) typeDefinitionMethodInfo() *jsonrpc.MethodInfo {

	if m.onTypeDefinition == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/typeDefinition",
		NewRequest: func() interface{} {
			return &defines.TypeDefinitionParams{}
		},
		Handler:    m.typeDefinition,
		RateLimits: []int{},
	}
}

func (m *Methods) OnImplementation(f func(ctx context.Context, req *defines.ImplementationParams) (result *[]defines.LocationLink, err error)) {
	m.onImplementation = f
}

func (m *Methods) implementation(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.ImplementationParams)
	if m.onImplementation != nil {
		res, err := m.onImplementation(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) implementationMethodInfo() *jsonrpc.MethodInfo {

	if m.onImplementation == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/implementation",
		NewRequest: func() interface{} {
			return &defines.ImplementationParams{}
		},
		Handler:    m.implementation,
		RateLimits: []int{},
	}
}

func (m *Methods) OnReferences(f func(ctx context.Context, req *defines.ReferenceParams) (result *[]defines.Location, err error)) {
	m.onReferences = f
}

func (m *Methods) references(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.ReferenceParams)
	if m.onReferences != nil {
		res, err := m.onReferences(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) referencesMethodInfo() *jsonrpc.MethodInfo {

	if m.onReferences == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/references",
		NewRequest: func() interface{} {
			return &defines.ReferenceParams{}
		},
		Handler:    m.references,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDocumentHighlight(f func(ctx context.Context, req *defines.DocumentHighlightParams) (result *[]defines.DocumentHighlight, err error)) {
	m.onDocumentHighlight = f
}

func (m *Methods) documentHighlight(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DocumentHighlightParams)
	if m.onDocumentHighlight != nil {
		res, err := m.onDocumentHighlight(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) documentHighlightMethodInfo() *jsonrpc.MethodInfo {

	if m.onDocumentHighlight == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/documentHighlight",
		NewRequest: func() interface{} {
			return &defines.DocumentHighlightParams{}
		},
		Handler:    m.documentHighlight,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDocumentSymbolWithSliceDocumentSymbol(f func(ctx context.Context, req *defines.DocumentSymbolParams) (result *[]defines.DocumentSymbol, err error)) {
	m.onDocumentSymbolWithSliceDocumentSymbol = f
}

func (m *Methods) documentSymbolWithSliceDocumentSymbol(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DocumentSymbolParams)
	if m.onDocumentSymbolWithSliceDocumentSymbol != nil {
		res, err := m.onDocumentSymbolWithSliceDocumentSymbol(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) documentSymbolWithSliceDocumentSymbolMethodInfo() *jsonrpc.MethodInfo {

	if m.onDocumentSymbolWithSliceDocumentSymbol == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/documentSymbol",
		NewRequest: func() interface{} {
			return &defines.DocumentSymbolParams{}
		},
		Handler:    m.documentSymbolWithSliceDocumentSymbol,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDocumentSymbolWithSliceSymbolInformation(f func(ctx context.Context, req *defines.DocumentSymbolParams) (result *[]defines.SymbolInformation, err error)) {
	m.onDocumentSymbolWithSliceSymbolInformation = f
}

func (m *Methods) documentSymbolWithSliceSymbolInformation(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DocumentSymbolParams)
	if m.onDocumentSymbolWithSliceSymbolInformation != nil {
		res, err := m.onDocumentSymbolWithSliceSymbolInformation(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) documentSymbolWithSliceSymbolInformationMethodInfo() *jsonrpc.MethodInfo {

	if m.onDocumentSymbolWithSliceSymbolInformation == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/documentSymbol",
		NewRequest: func() interface{} {
			return &defines.DocumentSymbolParams{}
		},
		Handler:    m.documentSymbolWithSliceSymbolInformation,
		RateLimits: []int{},
	}
}

func (m *Methods) OnWorkspaceSymbol(f func(ctx context.Context, req *defines.WorkspaceSymbolParams) (result *[]defines.SymbolInformation, err error)) {
	m.onWorkspaceSymbol = f
}

func (m *Methods) workspaceSymbol(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.WorkspaceSymbolParams)
	if m.onWorkspaceSymbol != nil {
		res, err := m.onWorkspaceSymbol(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) workspaceSymbolMethodInfo() *jsonrpc.MethodInfo {

	if m.onWorkspaceSymbol == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "workspace/symbol",
		NewRequest: func() interface{} {
			return &defines.WorkspaceSymbolParams{}
		},
		Handler:    m.workspaceSymbol,
		RateLimits: []int{},
	}
}

func (m *Methods) OnCodeActionWithSliceCommand(f func(ctx context.Context, req *defines.CodeActionParams) (result *[]defines.Command, err error)) {
	m.onCodeActionWithSliceCommand = f
}

func (m *Methods) codeActionWithSliceCommand(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.CodeActionParams)
	if m.onCodeActionWithSliceCommand != nil {
		res, err := m.onCodeActionWithSliceCommand(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) codeActionWithSliceCommandMethodInfo() *jsonrpc.MethodInfo {

	if m.onCodeActionWithSliceCommand == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/codeAction",
		NewRequest: func() interface{} {
			return &defines.CodeActionParams{}
		},
		Handler:    m.codeActionWithSliceCommand,
		RateLimits: []int{},
	}
}

func (m *Methods) OnCodeActionWithSliceCodeAction(f func(ctx context.Context, req *defines.CodeActionParams) (result *[]defines.CodeAction, err error)) {
	m.onCodeActionWithSliceCodeAction = f
}

func (m *Methods) codeActionWithSliceCodeAction(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.CodeActionParams)
	if m.onCodeActionWithSliceCodeAction != nil {
		res, err := m.onCodeActionWithSliceCodeAction(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) codeActionWithSliceCodeActionMethodInfo() *jsonrpc.MethodInfo {

	if m.onCodeActionWithSliceCodeAction == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/codeAction",
		NewRequest: func() interface{} {
			return &defines.CodeActionParams{}
		},
		Handler:    m.codeActionWithSliceCodeAction,
		RateLimits: []int{},
	}
}

func (m *Methods) OnCodeActionResolve(f func(ctx context.Context, req *defines.CodeAction) (result *defines.CodeAction, err error)) {
	m.onCodeActionResolve = f
}

func (m *Methods) codeActionResolve(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.CodeAction)
	if m.onCodeActionResolve != nil {
		res, err := m.onCodeActionResolve(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) codeActionResolveMethodInfo() *jsonrpc.MethodInfo {

	if m.onCodeActionResolve == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "codeAction/resolve",
		NewRequest: func() interface{} {
			return &defines.CodeAction{}
		},
		Handler:    m.codeActionResolve,
		RateLimits: []int{},
	}
}

func (m *Methods) OnCodeLens(f func(ctx context.Context, req *defines.CodeLensParams) (result *[]defines.CodeLens, err error)) {
	m.onCodeLens = f
}

func (m *Methods) codeLens(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.CodeLensParams)
	if m.onCodeLens != nil {
		res, err := m.onCodeLens(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) codeLensMethodInfo() *jsonrpc.MethodInfo {

	if m.onCodeLens == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/codeLens",
		NewRequest: func() interface{} {
			return &defines.CodeLensParams{}
		},
		Handler:    m.codeLens,
		RateLimits: []int{},
	}
}

func (m *Methods) OnCodeLensResolve(f func(ctx context.Context, req *defines.CodeLens) (result *defines.CodeLens, err error)) {
	m.onCodeLensResolve = f
}

func (m *Methods) codeLensResolve(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.CodeLens)
	if m.onCodeLensResolve != nil {
		res, err := m.onCodeLensResolve(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) codeLensResolveMethodInfo() *jsonrpc.MethodInfo {

	if m.onCodeLensResolve == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "codeLens/resolve",
		NewRequest: func() interface{} {
			return &defines.CodeLens{}
		},
		Handler:    m.codeLensResolve,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDocumentFormatting(f func(ctx context.Context, req *defines.DocumentFormattingParams) (result *[]defines.TextEdit, err error)) {
	m.onDocumentFormatting = f
}

func (m *Methods) documentFormatting(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DocumentFormattingParams)
	if m.onDocumentFormatting != nil {
		res, err := m.onDocumentFormatting(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) documentFormattingMethodInfo() *jsonrpc.MethodInfo {

	if m.onDocumentFormatting == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/formatting",
		NewRequest: func() interface{} {
			return &defines.DocumentFormattingParams{}
		},
		Handler:    m.documentFormatting,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDocumentRangeFormatting(f func(ctx context.Context, req *defines.DocumentRangeFormattingParams) (result *[]defines.TextEdit, err error)) {
	m.onDocumentRangeFormatting = f
}

func (m *Methods) documentRangeFormatting(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DocumentRangeFormattingParams)
	if m.onDocumentRangeFormatting != nil {
		res, err := m.onDocumentRangeFormatting(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) documentRangeFormattingMethodInfo() *jsonrpc.MethodInfo {

	if m.onDocumentRangeFormatting == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/rangeFormatting",
		NewRequest: func() interface{} {
			return &defines.DocumentRangeFormattingParams{}
		},
		Handler:    m.documentRangeFormatting,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDocumentOnTypeFormatting(f func(ctx context.Context, req *defines.DocumentOnTypeFormattingParams) (result *[]defines.TextEdit, err error)) {
	m.onDocumentOnTypeFormatting = f
}

func (m *Methods) documentOnTypeFormatting(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DocumentOnTypeFormattingParams)
	if m.onDocumentOnTypeFormatting != nil {
		res, err := m.onDocumentOnTypeFormatting(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) documentOnTypeFormattingMethodInfo() *jsonrpc.MethodInfo {

	if m.onDocumentOnTypeFormatting == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/onTypeFormatting",
		NewRequest: func() interface{} {
			return &defines.DocumentOnTypeFormattingParams{}
		},
		Handler:    m.documentOnTypeFormatting,
		RateLimits: []int{},
	}
}

func (m *Methods) OnRenameRequest(f func(ctx context.Context, req *defines.RenameParams) (result *defines.WorkspaceEdit, err error)) {
	m.onRenameRequest = f
}

func (m *Methods) renameRequest(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.RenameParams)
	if m.onRenameRequest != nil {
		res, err := m.onRenameRequest(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) renameRequestMethodInfo() *jsonrpc.MethodInfo {

	if m.onRenameRequest == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/rename",
		NewRequest: func() interface{} {
			return &defines.RenameParams{}
		},
		Handler:    m.renameRequest,
		RateLimits: []int{},
	}
}

func (m *Methods) OnPrepareRename(f func(ctx context.Context, req *defines.PrepareRenameParams) (result *defines.Range, err error)) {
	m.onPrepareRename = f
}

func (m *Methods) prepareRename(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.PrepareRenameParams)
	if m.onPrepareRename != nil {
		res, err := m.onPrepareRename(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) prepareRenameMethodInfo() *jsonrpc.MethodInfo {

	if m.onPrepareRename == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/rename",
		NewRequest: func() interface{} {
			return &defines.PrepareRenameParams{}
		},
		Handler:    m.prepareRename,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDocumentLinks(f func(ctx context.Context, req *defines.DocumentLinkParams) (result *[]defines.DocumentLink, err error)) {
	m.onDocumentLinks = f
}

func (m *Methods) documentLinks(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DocumentLinkParams)
	if m.onDocumentLinks != nil {
		res, err := m.onDocumentLinks(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) documentLinksMethodInfo() *jsonrpc.MethodInfo {

	if m.onDocumentLinks == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/documentLink",
		NewRequest: func() interface{} {
			return &defines.DocumentLinkParams{}
		},
		Handler:    m.documentLinks,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDocumentLinkResolve(f func(ctx context.Context, req *defines.DocumentLink) (result *defines.DocumentLink, err error)) {
	m.onDocumentLinkResolve = f
}

func (m *Methods) documentLinkResolve(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DocumentLink)
	if m.onDocumentLinkResolve != nil {
		res, err := m.onDocumentLinkResolve(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) documentLinkResolveMethodInfo() *jsonrpc.MethodInfo {

	if m.onDocumentLinkResolve == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "documentLink/resolve",
		NewRequest: func() interface{} {
			return &defines.DocumentLink{}
		},
		Handler:    m.documentLinkResolve,
		RateLimits: []int{},
	}
}

func (m *Methods) OnDocumentColor(f func(ctx context.Context, req *defines.DocumentColorParams) (result *[]defines.ColorInformation, err error)) {
	m.onDocumentColor = f
}

func (m *Methods) documentColor(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.DocumentColorParams)
	if m.onDocumentColor != nil {
		res, err := m.onDocumentColor(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) documentColorMethodInfo() *jsonrpc.MethodInfo {

	if m.onDocumentColor == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/documentColor",
		NewRequest: func() interface{} {
			return &defines.DocumentColorParams{}
		},
		Handler:    m.documentColor,
		RateLimits: []int{},
	}
}

func (m *Methods) OnColorPresentation(f func(ctx context.Context, req *defines.ColorPresentationParams) (result *[]defines.ColorPresentation, err error)) {
	m.onColorPresentation = f
}

func (m *Methods) colorPresentation(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.ColorPresentationParams)
	if m.onColorPresentation != nil {
		res, err := m.onColorPresentation(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) colorPresentationMethodInfo() *jsonrpc.MethodInfo {

	if m.onColorPresentation == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/colorPresentation",
		NewRequest: func() interface{} {
			return &defines.ColorPresentationParams{}
		},
		Handler:    m.colorPresentation,
		RateLimits: []int{},
	}
}

func (m *Methods) OnFoldingRanges(f func(ctx context.Context, req *defines.FoldingRangeParams) (result *[]defines.FoldingRange, err error)) {
	m.onFoldingRanges = f
}

func (m *Methods) foldingRanges(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.FoldingRangeParams)
	if m.onFoldingRanges != nil {
		res, err := m.onFoldingRanges(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) foldingRangesMethodInfo() *jsonrpc.MethodInfo {

	if m.onFoldingRanges == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/foldingRange",
		NewRequest: func() interface{} {
			return &defines.FoldingRangeParams{}
		},
		Handler:    m.foldingRanges,
		RateLimits: []int{},
	}
}

func (m *Methods) OnSelectionRanges(f func(ctx context.Context, req *defines.SelectionRangeParams) (result *[]defines.SelectionRange, err error)) {
	m.onSelectionRanges = f
}

func (m *Methods) selectionRanges(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*defines.SelectionRangeParams)
	if m.onSelectionRanges != nil {
		res, err := m.onSelectionRanges(ctx, params)
		e := wrapErrorToRespError(err, 0)
		return res, e
	}
	return nil, nil
}

func (m *Methods) selectionRangesMethodInfo() *jsonrpc.MethodInfo {

	if m.onSelectionRanges == nil {
		return nil
	}
	return &jsonrpc.MethodInfo{
		Name: "textDocument/selectionRange",
		NewRequest: func() interface{} {
			return &defines.SelectionRangeParams{}
		},
		Handler:    m.selectionRanges,
		RateLimits: []int{},
	}
}

func (m *Methods) GetMethods() []*jsonrpc.MethodInfo {
	return []*jsonrpc.MethodInfo{
		m.initializeMethodInfo(),
		m.initializedMethodInfo(),
		m.shutdownMethodInfo(),
		m.exitMethodInfo(),
		m.didChangeConfigurationMethodInfo(),
		m.didChangeWatchedFilesMethodInfo(),
		m.didOpenTextDocumentMethodInfo(),
		m.didChangeTextDocumentMethodInfo(),
		m.didCloseTextDocumentMethodInfo(),
		m.willSaveTextDocumentMethodInfo(),
		m.didSaveTextDocumentMethodInfo(),
		m.executeCommandMethodInfo(),
		m.hoverMethodInfo(),
		m.completionMethodInfo(),
		m.completionResolveMethodInfo(),
		m.signatureHelpMethodInfo(),
		m.declarationMethodInfo(),
		m.definitionMethodInfo(),
		m.typeDefinitionMethodInfo(),
		m.implementationMethodInfo(),
		m.referencesMethodInfo(),
		m.documentHighlightMethodInfo(),
		m.documentSymbolWithSliceDocumentSymbolMethodInfo(),
		m.documentSymbolWithSliceSymbolInformationMethodInfo(),
		m.workspaceSymbolMethodInfo(),
		m.codeActionWithSliceCommandMethodInfo(),
		m.codeActionWithSliceCodeActionMethodInfo(),
		m.codeActionResolveMethodInfo(),
		m.codeLensMethodInfo(),
		m.codeLensResolveMethodInfo(),
		m.documentFormattingMethodInfo(),
		m.documentRangeFormattingMethodInfo(),
		m.documentOnTypeFormattingMethodInfo(),
		m.renameRequestMethodInfo(),
		m.prepareRenameMethodInfo(),
		m.documentLinksMethodInfo(),
		m.documentLinkResolveMethodInfo(),
		m.documentColorMethodInfo(),
		m.colorPresentationMethodInfo(),
		m.foldingRangesMethodInfo(),
		m.selectionRangesMethodInfo(),
	}
}
