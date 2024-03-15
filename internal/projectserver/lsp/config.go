package lsp

import (
	"io"
	"net/http"

	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/rs/zerolog"
)

type Config struct {
	// if Network is null, will use stdio
	Network           string
	Address           string //examples: localhost:8305, :8305
	Certificate       string
	CertificateKey    string
	MaxWebsocketPerIp int
	BehindCloudProxy  bool
	HttpHandler       http.Handler

	OnSession jsonrpc.SessionCreationCallbackFn

	StdioInput  io.Reader
	StdioOutput io.Writer
	Logger      zerolog.Logger

	MessageReaderWriter jsonrpc.MessageReaderWriter

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
