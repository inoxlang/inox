package lsp

import (
	"context"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
)

func (m *Methods) builtinInitialize(ctx context.Context, req *defines.InitializeParams) (defines.InitializeResult, error) {
	resp := defines.InitializeResult{}
	resp.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindFull
	if m.Opt.CompletionProvider != nil {
		resp.Capabilities.CompletionProvider = m.Opt.CompletionProvider
	} else if m.onCompletion != nil {
		resp.Capabilities.CompletionProvider = &defines.CompletionOptions{
			TriggerCharacters: &[]string{"."},
		}
	}
	if m.Opt.HoverProvider != nil {
		resp.Capabilities.HoverProvider = m.Opt.HoverProvider
	} else if m.onHover != nil {
		resp.Capabilities.HoverProvider = true
	}
	if m.Opt.SignatureHelpProvider != nil {
		resp.Capabilities.SignatureHelpProvider = m.Opt.SignatureHelpProvider
	} else if m.onSignatureHelp != nil {
		resp.Capabilities.SignatureHelpProvider = &defines.SignatureHelpOptions{
			TriggerCharacters: &[]string{"(", ","},
		}
	}
	if m.Opt.DeclarationProvider != nil {
		resp.Capabilities.DeclarationProvider = m.Opt.DeclarationProvider
	} else if m.onDeclaration != nil {
		resp.Capabilities.DeclarationProvider = true
	}
	if m.Opt.DefinitionProvider != nil {
		resp.Capabilities.DefinitionProvider = m.Opt.DefinitionProvider
	} else if m.onDefinition != nil {
		resp.Capabilities.DefinitionProvider = true
	}
	if m.Opt.TypeDefinitionProvider != nil {
		resp.Capabilities.TypeDefinitionProvider = m.Opt.TypeDefinitionProvider
	} else if m.onTypeDefinition != nil {
		resp.Capabilities.TypeDefinitionProvider = true
	}
	if m.Opt.ImplementationProvider != nil {
		resp.Capabilities.ImplementationProvider = m.Opt.ImplementationProvider
	} else if m.onImplementation != nil {
		resp.Capabilities.ImplementationProvider = true
	}

	if m.Opt.ReferencesProvider != nil {
		resp.Capabilities.ReferencesProvider = m.Opt.ReferencesProvider
	} else if m.onReferences != nil {
		resp.Capabilities.ReferencesProvider = true
	}

	if m.Opt.DocumentHighlightProvider != nil {
		resp.Capabilities.DocumentHighlightProvider = m.Opt.DocumentHighlightProvider
	} else if m.onDocumentHighlight != nil {
		resp.Capabilities.DocumentHighlightProvider = true
	}

	if m.Opt.DocumentSymbolProvider != nil {
		resp.Capabilities.DocumentSymbolProvider = m.Opt.DocumentSymbolProvider
	} else if m.onDocumentSymbolWithSliceDocumentSymbol != nil {
		resp.Capabilities.DocumentSymbolProvider = true
	} else if m.onDocumentSymbolWithSliceSymbolInformation != nil {
		resp.Capabilities.DocumentSymbolProvider = true
	}
	if m.Opt.CodeActionProvider != nil {
		resp.Capabilities.CodeActionProvider = m.Opt.CodeActionProvider
	} else if m.onCodeActionWithSliceCodeAction != nil {
		resp.Capabilities.CodeActionProvider = true
	} else if m.onCodeActionWithSliceCommand != nil {
		resp.Capabilities.CodeActionProvider = true
	}
	if m.Opt.CodeLensProvider != nil {
		resp.Capabilities.CodeLensProvider = m.Opt.CodeLensProvider
	} else if m.onCodeLens != nil {
		t := true
		resp.Capabilities.CodeLensProvider = &defines.CodeLensOptions{WorkDoneProgressOptions: defines.WorkDoneProgressOptions{WorkDoneProgress: &t}, ResolveProvider: &t}
	}
	if m.Opt.DocumentLinkProvider != nil {
		resp.Capabilities.DocumentLinkProvider = m.Opt.DocumentLinkProvider
	} else if m.onDocumentLinks != nil {
		t := true
		resp.Capabilities.DocumentLinkProvider = &defines.DocumentLinkOptions{WorkDoneProgressOptions: defines.WorkDoneProgressOptions{WorkDoneProgress: &t}, ResolveProvider: &t}
	}
	if m.Opt.ColorProvider != nil {
		resp.Capabilities.ColorProvider = m.Opt.ColorProvider
	} else if m.onDocumentColor != nil {
		resp.Capabilities.ColorProvider = true
	}
	if m.Opt.WorkspaceSymbolProvider != nil {
		resp.Capabilities.WorkspaceSymbolProvider = m.Opt.WorkspaceSymbolProvider
	} else if m.onWorkspaceSymbol != nil {
		resp.Capabilities.WorkspaceSymbolProvider = true
	}
	if m.Opt.DocumentFormattingProvider != nil {
		resp.Capabilities.DocumentFormattingProvider = m.Opt.DocumentFormattingProvider
	} else if m.onDocumentFormatting != nil {
		resp.Capabilities.DocumentFormattingProvider = true
	}
	if m.Opt.DocumentRangeFormattingProvider != nil {
		resp.Capabilities.DocumentRangeFormattingProvider = m.Opt.DocumentRangeFormattingProvider
	} else if m.onDocumentRangeFormatting != nil {
		resp.Capabilities.DocumentRangeFormattingProvider = true
	}
	if m.Opt.DocumentOnTypeFormattingProvider != nil {
		resp.Capabilities.DocumentOnTypeFormattingProvider = m.Opt.DocumentOnTypeFormattingProvider
	} else if m.onDocumentOnTypeFormatting != nil {
		// TODO
		resp.Capabilities.DocumentOnTypeFormattingProvider = &defines.DocumentOnTypeFormattingOptions{}
	}
	if m.Opt.RenameProvider != nil {
		resp.Capabilities.RenameProvider = m.Opt.RenameProvider
	} else if m.onPrepareRename != nil {
		resp.Capabilities.RenameProvider = true
	}
	if m.Opt.FoldingRangeProvider != nil {
		resp.Capabilities.FoldingRangeProvider = m.Opt.FoldingRangeProvider
	} else if m.onFoldingRanges != nil {
		resp.Capabilities.FoldingRangeProvider = true
	}
	if m.Opt.SelectionRangeProvider != nil {
		resp.Capabilities.SelectionRangeProvider = m.Opt.SelectionRangeProvider
	} else if m.onSelectionRanges != nil {
		resp.Capabilities.SelectionRangeProvider = true
	}
	if m.Opt.ExecuteCommandProvider != nil {
		resp.Capabilities.ExecuteCommandProvider = m.Opt.ExecuteCommandProvider
	} else if m.onExecuteCommand != nil {
		// TODO
		resp.Capabilities.ExecuteCommandProvider = &defines.ExecuteCommandOptions{}
	}
	if m.Opt.DocumentLinkProvider != nil {
		resp.Capabilities.DocumentLinkProvider = m.Opt.DocumentLinkProvider
	} else if m.onDocumentLinks != nil {
		// TODO
		resp.Capabilities.DocumentLinkProvider = &defines.DocumentLinkOptions{}
	}
	if m.Opt.SemanticTokensProvider != nil {
		resp.Capabilities.SemanticTokensProvider = m.Opt.SemanticTokensProvider
	}

	if m.Opt.MonikerProvider != nil {
		resp.Capabilities.MonikerProvider = m.Opt.MonikerProvider
	}

	if m.Opt.CallHierarchyProvider != nil {
		resp.Capabilities.CallHierarchyProvider = m.Opt.CallHierarchyProvider
	}

	//}
	//if m.onMon != nil{
	//	resp.Capabilities.MonikerProvider = true
	//}
	//if m.onTypeHierarchy != nil{
	//	resp.Capabilities.TypeHierarchyProvider = true
	//}

	return resp, nil
}
