package lsp

import (
	"io"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
)

type Options struct {
	// if Network is null, will use stdio
	Network string
	Address string

	StdioInput  io.Reader
	StdioOutput io.Writer

	TextDocumentSync                 defines.TextDocumentSyncKind
	CompletionProvider               *defines.CompletionOptions
	HoverProvider                    *defines.HoverOptions
	SignatureHelpProvider            *defines.SignatureHelpOptions
	DeclarationProvider              *defines.DeclarationOptions
	DefinitionProvider               *defines.DefinitionOptions
	TypeDefinitionProvider           *defines.TypeDefinitionOptions
	ImplementationProvider           *defines.ImplementationOptions
	ReferencesProvider               *defines.ReferenceOptions
	DocumentHighlightProvider        *defines.DocumentHighlightOptions
	DocumentSymbolProvider           *defines.DocumentSymbolOptions
	CodeActionProvider               *defines.CodeActionOptions
	CodeLensProvider                 *defines.CodeLensOptions
	DocumentLinkProvider             *defines.DocumentLinkOptions
	ColorProvider                    *defines.DocumentColorOptions
	ColorWithRegistrationProvider    *defines.DocumentColorRegistrationOptions
	WorkspaceSymbolProvider          *defines.WorkspaceSymbolOptions
	DocumentFormattingProvider       *defines.DocumentFormattingOptions
	DocumentRangeFormattingProvider  *defines.DocumentRangeFormattingOptions
	DocumentOnTypeFormattingProvider *defines.DocumentOnTypeFormattingOptions
	RenameProvider                   *defines.RenameOptions
	FoldingRangeProvider             *defines.FoldingRangeOptions
	SelectionRangeProvider           *defines.SelectionRangeOptions
	ExecuteCommandProvider           *defines.ExecuteCommandOptions
	CallHierarchyProvider            *defines.CallHierarchyOptions
	LinkProvider                     *defines.DocumentLinkOptions
	SemanticTokensProvider           *defines.SemanticTokensOptions
	Workspace                        *struct {
		FileOperations *defines.FileOperationOptions
	}
	MonikerProvider *defines.MonikerOptions
	Experimental    interface{}
}
