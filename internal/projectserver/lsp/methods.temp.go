package lsp

import "github.com/inoxlang/inox/internal/projectserver/lsp/defines"

type method struct {
	Name          string
	RegisterName  string
	Args          interface{}
	Result        interface{}
	Error         interface{}
	Code          interface{}
	ProgressToken interface{}
	WithBuiltin   bool

	// List of the maximum number of calls allowed  windows with increasing durations (1s, 10s, and 100s).
	// Example: [10, 50, 200] means at most 10 calls in 1s, 50 calls in 50s and 200 calls in 100s.
	RateLimits    []int
	SensitiveData bool
}

type or []interface{}

type _any struct{ _ int }

var methods = []method{
	{
		Name:        "Initialize",
		Args:        defines.InitializeParams{},
		Result:      defines.InitializeResult{},
		Error:       defines.InitializeError{},
		Code:        defines.InitializeErrorUnknownProtocolVersion,
		WithBuiltin: true,
	},
	{
		Name: "Initialized",
		Args: defines.InitializeParams{},
	},
	{
		Name: "Shutdown",
		Args: defines.NoParams{},
	},
	{
		Name: "Exit",
		Args: defines.NoParams{},
	},
	{
		Name:          "DidChangeConfiguration",
		RegisterName:  "workspace/didChangeConfiguration",
		Args:          defines.DidChangeConfigurationParams{},
		SensitiveData: true,
	},
	{
		Name:         "DidChangeWatchedFiles",
		RegisterName: "workspace/didChangeWatchedFiles",
		Args:         defines.DidChangeWatchedFilesParams{},
	},
	{
		Name:         "DidOpenTextDocument",
		RegisterName: "textDocument/didOpen",
		Args:         defines.DidOpenTextDocumentParams{},
	},
	{
		Name:         "DidChangeTextDocument",
		RegisterName: "textDocument/didChange",
		Args:         defines.DidChangeTextDocumentParams{},
		RateLimits:   []int{20, 60, 300},
	},
	{
		Name:         "DidCloseTextDocument",
		RegisterName: "textDocument/didClose",
		Args:         defines.DidCloseTextDocumentParams{},
	},
	{
		Name:         "WillSaveTextDocument",
		RegisterName: "textDocument/willSave",
		Args:         defines.WillSaveTextDocumentParams{},
	},
	{
		Name:          "DidSaveTextDocument",
		RegisterName:  "textDocument/didSave",
		Args:          defines.DidSaveTextDocumentParams{},
		SensitiveData: true,
	},
	{
		Name:          "ExecuteCommand",
		Args:          defines.ExecuteCommandParams{},
		Result:        interface{}(nil),
		Error:         nil,
		ProgressToken: nil,
	},
	{
		Name:         "Hover",
		RegisterName: "textDocument/hover",
		Args:         defines.HoverParams{},
		Result:       defines.Hover{},
	},
	{
		Name:          "Completion",
		RegisterName:  "textDocument/completion",
		Args:          defines.CompletionParams{},
		Result:        []defines.CompletionItem{},
		ProgressToken: []defines.CompletionItem{},
		RateLimits:    []int{10, 35, 100},
	},
	{
		Name:         "CompletionResolve",
		RegisterName: "completionItem/resolve",
		Args:         defines.CompletionItem{},
		Result:       defines.CompletionItem{},
	},
	{
		Name:         "SignatureHelp",
		RegisterName: "textDocument/signatureHelp",
		Args:         defines.SignatureHelpParams{},
		Result:       defines.SignatureHelp{},
	},
	{
		Name:          "Declaration",
		RegisterName:  "textDocument/declaration",
		Args:          defines.DeclarationParams{},
		Result:        []defines.LocationLink{},
		ProgressToken: or{[]defines.Location{}, []defines.LocationLink{}},
	},
	{
		Name:          "Definition",
		RegisterName:  "textDocument/definition",
		Args:          defines.DefinitionParams{},
		Result:        []defines.LocationLink{},
		ProgressToken: or{[]defines.Location{}, []defines.LocationLink{}},
	},
	{
		Name:          "TypeDefinition",
		RegisterName:  "textDocument/typeDefinition",
		Args:          defines.TypeDefinitionParams{},
		Result:        []defines.LocationLink{},
		ProgressToken: or{[]defines.Location{}, []defines.LocationLink{}},
	},
	{
		Name:          "Implementation",
		RegisterName:  "textDocument/implementation",
		Args:          defines.ImplementationParams{},
		Result:        []defines.LocationLink{},
		ProgressToken: or{[]defines.Location{}, []defines.LocationLink{}},
	},
	{
		Name:          "References",
		RegisterName:  "textDocument/references",
		Args:          defines.ReferenceParams{},
		Result:        []defines.Location{},
		ProgressToken: []defines.Location{},
	},
	{
		Name:          "DocumentHighlight",
		RegisterName:  "textDocument/documentHighlight",
		Args:          defines.DocumentHighlightParams{},
		Result:        []defines.DocumentHighlight{},
		ProgressToken: []defines.DocumentHighlight{},
	},
	{
		Name:          "DocumentSymbol",
		RegisterName:  "textDocument/documentSymbol",
		Args:          defines.DocumentSymbolParams{},
		Result:        or{[]defines.DocumentSymbol{}, []defines.SymbolInformation{}},
		ProgressToken: or{[]defines.DocumentSymbol{}, []defines.SymbolInformation{}},
	},
	{
		Name:          "WorkspaceSymbol",
		RegisterName:  "workspace/symbol",
		Args:          defines.WorkspaceSymbolParams{},
		Result:        []defines.SymbolInformation{},
		ProgressToken: []defines.SymbolInformation{},
	},
	{
		Name:          "CodeAction",
		RegisterName:  "textDocument/codeAction",
		Args:          defines.CodeActionParams{},
		Result:        or{[]defines.Command{}, []defines.CodeAction{}},
		ProgressToken: or{[]defines.Command{}, []defines.CodeAction{}},
	},
	{
		Name:         "CodeActionResolve",
		RegisterName: "codeAction/resolve",
		Args:         defines.CodeAction{},
		Result:       defines.CodeAction{},
	},
	{
		Name:          "CodeLens",
		RegisterName:  "textDocument/codeLens",
		Args:          defines.CodeLensParams{},
		Result:        []defines.CodeLens{},
		ProgressToken: []defines.CodeLens{},
	},
	{
		Name:         "CodeLensResolve",
		RegisterName: "codeLens/resolve",
		Args:         defines.CodeLens{},
		Result:       defines.CodeLens{},
	},
	{
		Name:          "DocumentFormatting",
		RegisterName:  "textDocument/formatting",
		Args:          defines.DocumentFormattingParams{},
		Result:        []defines.TextEdit{},
		SensitiveData: true,
	},
	{
		Name:          "DocumentRangeFormatting",
		RegisterName:  "textDocument/rangeFormatting",
		Args:          defines.DocumentRangeFormattingParams{},
		Result:        []defines.TextEdit{},
		SensitiveData: true,
	},
	{
		Name:          "DocumentOnTypeFormatting",
		RegisterName:  "textDocument/onTypeFormatting",
		Args:          defines.DocumentOnTypeFormattingParams{},
		Result:        []defines.TextEdit{},
		SensitiveData: true,
	},
	{
		Name:          "RenameRequest",
		RegisterName:  "textDocument/rename",
		Args:          defines.RenameParams{},
		Result:        defines.WorkspaceEdit{},
		SensitiveData: true,
	},
	{
		Name:          "PrepareRename",
		RegisterName:  "textDocument/rename",
		Args:          defines.PrepareRenameParams{},
		Result:        defines.Range{},
		SensitiveData: true,
	},
	{
		Name:          "DocumentLinks",
		RegisterName:  "textDocument/documentLink",
		Args:          defines.DocumentLinkParams{},
		Result:        []defines.DocumentLink{},
		ProgressToken: []defines.DocumentLink{},
	},
	{
		Name:         "DocumentLinkResolve",
		RegisterName: "documentLink/resolve",
		Args:         defines.DocumentLink{},
		Result:       defines.DocumentLink{},
	},
	{
		Name:          "DocumentColor",
		RegisterName:  "textDocument/documentColor",
		Args:          defines.DocumentColorParams{},
		Result:        []defines.ColorInformation{},
		ProgressToken: []defines.ColorInformation{},
	},
	{
		Name:          "ColorPresentation",
		RegisterName:  "textDocument/colorPresentation",
		Args:          defines.ColorPresentationParams{},
		Result:        []defines.ColorPresentation{},
		ProgressToken: []defines.ColorPresentation{},
	},
	{
		Name:          "FoldingRanges",
		RegisterName:  "textDocument/foldingRange",
		Args:          defines.FoldingRangeParams{},
		Result:        []defines.FoldingRange{},
		ProgressToken: []defines.FoldingRange{},
	},
	{
		Name:          "SelectionRanges",
		RegisterName:  "textDocument/selectionRange",
		Args:          defines.SelectionRangeParams{},
		Result:        []defines.SelectionRange{},
		ProgressToken: []defines.SelectionRange{},
	},
	{
		Name:         "DocumentDiagnostic",
		RegisterName: "textDocument/diagnostic",
		Args:         defines.DocumentDiagnosticParams{},
		Result:       _any{},
	},
}
