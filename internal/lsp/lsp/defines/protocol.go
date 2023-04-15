package defines

type ClientCapabilities struct {
	_ClientCapabilities
	WorkspaceFoldersClientCapabilities
	ConfigurationClientCapabilities
	WorkDoneProgressClientCapabilities
}
type ServerCapabilities struct {
	_ServerCapabilities
	WorkspaceFoldersServerCapabilities
}
type InitializeParams struct {
	_InitializeParams
	WorkspaceFoldersInitializeParams
}

type _ServerCapabilities struct {

	// Defines how text documents are synced. Is either a detailed structure defining each notification or
	// for backwards compatibility the TextDocumentSyncKind number.
	TextDocumentSync interface{} `json:"textDocumentSync,omitempty"` // TextDocumentSyncOptions, TextDocumentSyncKind,

	// The server provides completion support.
	CompletionProvider *CompletionOptions `json:"completionProvider,omitempty"`

	// The server provides hover support.
	HoverProvider interface{} `json:"hoverProvider,omitempty"` // bool, HoverOptions,

	// The server provides signature help support.
	SignatureHelpProvider *SignatureHelpOptions `json:"signatureHelpProvider,omitempty"`

	// The server provides Goto Declaration support.
	DeclarationProvider interface{} `json:"declarationProvider,omitempty"` // bool, DeclarationOptions, DeclarationRegistrationOptions,

	// The server provides goto definition support.
	DefinitionProvider interface{} `json:"definitionProvider,omitempty"` // bool, DefinitionOptions,

	// The server provides Goto Type Definition support.
	TypeDefinitionProvider interface{} `json:"typeDefinitionProvider,omitempty"` // bool, TypeDefinitionOptions, TypeDefinitionRegistrationOptions,

	// The server provides Goto Implementation support.
	ImplementationProvider interface{} `json:"implementationProvider,omitempty"` // bool, ImplementationOptions, ImplementationRegistrationOptions,

	// The server provides find references support.
	ReferencesProvider interface{} `json:"referencesProvider,omitempty"` // bool, ReferenceOptions,

	// The server provides document highlight support.
	DocumentHighlightProvider interface{} `json:"documentHighlightProvider,omitempty"` // bool, DocumentHighlightOptions,

	// The server provides document symbol support.
	DocumentSymbolProvider interface{} `json:"documentSymbolProvider,omitempty"` // bool, DocumentSymbolOptions,

	// The server provides code actions. CodeActionOptions may only be
	// specified if the client states that it supports
	// `codeActionLiteralSupport` in its initial `initialize` request.
	CodeActionProvider interface{} `json:"codeActionProvider,omitempty"` // bool, CodeActionOptions,

	// The server provides code lens.
	CodeLensProvider *CodeLensOptions `json:"codeLensProvider,omitempty"`

	// The server provides document link support.
	DocumentLinkProvider *DocumentLinkOptions `json:"documentLinkProvider,omitempty"`

	// The server provides color provider support.
	ColorProvider interface{} `json:"colorProvider,omitempty"` // bool, DocumentColorOptions, DocumentColorRegistrationOptions,

	// The server provides workspace symbol support.
	WorkspaceSymbolProvider interface{} `json:"workspaceSymbolProvider,omitempty"` // bool, WorkspaceSymbolOptions,

	// The server provides document formatting.
	DocumentFormattingProvider interface{} `json:"documentFormattingProvider,omitempty"` // bool, DocumentFormattingOptions,

	// The server provides document range formatting.
	DocumentRangeFormattingProvider interface{} `json:"documentRangeFormattingProvider,omitempty"` // bool, DocumentRangeFormattingOptions,

	// The server provides document formatting on typing.
	DocumentOnTypeFormattingProvider *DocumentOnTypeFormattingOptions `json:"documentOnTypeFormattingProvider,omitempty"`

	// The server provides rename support. RenameOptions may only be
	// specified if the client states that it supports
	// `prepareSupport` in its initial `initialize` request.
	RenameProvider interface{} `json:"renameProvider,omitempty"` // bool, RenameOptions,

	// The server provides folding provider support.
	FoldingRangeProvider interface{} `json:"foldingRangeProvider,omitempty"` // bool, FoldingRangeOptions, FoldingRangeRegistrationOptions,

	// The server provides selection range support.
	SelectionRangeProvider interface{} `json:"selectionRangeProvider,omitempty"` // bool, SelectionRangeOptions, SelectionRangeRegistrationOptions,

	// The server provides execute command support.
	ExecuteCommandProvider *ExecuteCommandOptions `json:"executeCommandProvider,omitempty"`

	// The server provides call hierarchy support.
	//
	// @since 3.16.0
	CallHierarchyProvider interface{} `json:"callHierarchyProvider,omitempty"` // bool, CallHierarchyOptions, CallHierarchyRegistrationOptions,

	// The server provides linked editing range support.
	//
	// @since 3.16.0
	LinkedEditingRangeProvider interface{} `json:"linkedEditingRangeProvider,omitempty"` // bool, LinkedEditingRangeOptions, LinkedEditingRangeRegistrationOptions,

	// The server provides semantic tokens support.
	//
	// @since 3.16.0
	SemanticTokensProvider interface{} `json:"semanticTokensProvider,omitempty"` // SemanticTokensOptions, SemanticTokensRegistrationOptions,

	// Window specific server capabilities.
	Workspace *struct {

		// The server is interested in notificationsrequests for operations on files.
		//
		// @since 3.16.0
		FileOperations *FileOperationOptions `json:"fileOperations,omitempty"`
	} `json:"workspace,omitempty"`

	// The server provides moniker support.
	//
	// @since 3.16.0
	MonikerProvider interface{} `json:"monikerProvider,omitempty"` // bool, MonikerOptions, MonikerRegistrationOptions,

	// Experimental server capabilities.
	Experimental interface{} `json:"experimental,omitempty"`
}

/**
 * @deprecated Use ApplyWorkspaceEditResult instead.
 */
type ApplyWorkspaceEditResponse ApplyWorkspaceEditResult

/**
 * General parameters to to register for an notification or to register a provider.
 */
type Registration struct {

	// The id used to register the request. The id can be used to deregister
	// the request again.
	Id string `json:"id,omitempty"`

	// The method to register for.
	Method string `json:"method,omitempty"`

	// Options necessary for the registration.
	RegisterOptions interface{} `json:"registerOptions,omitempty"`
}

type RegistrationParams struct {
	Registrations []Registration `json:"registrations,omitempty"`
}

/**
 * General parameters to unregister a request or notification.
 */
type Unregistration struct {

	// The id used to unregister the request or notification. Usually an id
	// provided during the register request.
	Id string `json:"id,omitempty"`

	// The method to unregister for.
	Method string `json:"method,omitempty"`
}

type ProgressToken interface{} // number | string

type WorkDoneProgressParams struct {

	// An optional token that a server can use to report work done progress.
	WorkDoneToken *ProgressToken `json:"workDoneToken,omitempty"`
}

type PartialResultParams struct {

	// An optional token that a server can use to report partial results (e.g. streaming) to
	// the client.
	PartialResultToken *ProgressToken `json:"partialResultToken,omitempty"`
}

/**
 * A parameter literal used in requests to pass a text document and a position inside that
 * document.
 */
type TextDocumentPositionParams struct {

	// The text document.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The position inside the text document.
	Position Position `json:"position,omitempty"`
}

/**
 * Workspace specific client capabilities.
 */
type WorkspaceClientCapabilities struct {

	// The client supports applying batch edits
	// to the workspace by supporting the request
	// 'workspaceapplyEdit'
	ApplyEdit *bool `json:"applyEdit,omitempty"`

	// Capabilities specific to `WorkspaceEdit`s
	WorkspaceEdit *WorkspaceEditClientCapabilities `json:"workspaceEdit,omitempty"`

	// Capabilities specific to the `workspacedidChangeConfiguration` notification.
	DidChangeConfiguration *DidChangeConfigurationClientCapabilities `json:"didChangeConfiguration,omitempty"`

	// Capabilities specific to the `workspacedidChangeWatchedFiles` notification.
	DidChangeWatchedFiles *DidChangeWatchedFilesClientCapabilities `json:"didChangeWatchedFiles,omitempty"`

	// Capabilities specific to the `workspacesymbol` request.
	Symbol *WorkspaceSymbolClientCapabilities `json:"symbol,omitempty"`

	// Capabilities specific to the `workspaceexecuteCommand` request.
	ExecuteCommand *ExecuteCommandClientCapabilities `json:"executeCommand,omitempty"`

	// Capabilities specific to the semantic token requests scoped to the
	// workspace.
	//
	// @since 3.16.0.
	SemanticTokens *SemanticTokensWorkspaceClientCapabilities `json:"semanticTokens,omitempty"`

	// Capabilities specific to the code lens requests scoped to the
	// workspace.
	//
	// @since 3.16.0.
	CodeLens *CodeLensWorkspaceClientCapabilities `json:"codeLens,omitempty"`

	// The client has support for file notificationsrequests for user operations on files.
	//
	// Since 3.16.0
	FileOperations *FileOperationClientCapabilities `json:"fileOperations,omitempty"`

	// Capabilities specific to the inline values requests scoped to the
	// workspace.
	//
	// @since 3.17.0.
	InlineValues *InlineValuesWorkspaceClientCapabilities `json:"inlineValues,omitempty"`
}

/**
 * Completion client capabilities
 */
type CompletionClientCapabilities struct {

	// Whether completion supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// The client supports the following `CompletionItem` specific
	// capabilities.
	CompletionItem interface{} `json:"completionItem,omitempty"` //i, n, t, e, r, f, a, c, e, {, },  ,  , /, /,

	CompletionItemKind *struct {

		// The completion item kind values the client supports. When this
		// property exists the client also guarantees that it will
		// handle values outside its set gracefully and falls back
		// to a default value when unknown.
		//
		// If this property is not present the client only supports
		// the completion items kinds from `Text` to `Reference` as defined in
		// the initial version of the protocol.
		ValueSet *[]CompletionItemKind `json:"valueSet,omitempty"`
	} `json:"completionItemKind,omitempty"`

	// Defines how the client handles whitespace and indentation
	// when accepting a completion item that uses multi line
	// text in either `insertText` or `textEdit`.
	//
	// @since 3.17.0
	InsertTextMode *InsertTextMode `json:"insertTextMode,omitempty"`

	// The client supports to send additional context information for a
	// `textDocumentcompletion` request.
	ContextSupport *bool `json:"contextSupport,omitempty"`
}

/**
 * Text document specific client capabilities.
 */
type TextDocumentClientCapabilities struct {

	// Defines which synchronization capabilities the client supports.
	Synchronization *TextDocumentSyncClientCapabilities `json:"synchronization,omitempty"`

	// Capabilities specific to the `textDocumentcompletion`
	Completion *CompletionClientCapabilities `json:"completion,omitempty"`

	// Capabilities specific to the `textDocumenthover`
	Hover *HoverClientCapabilities `json:"hover,omitempty"`

	// Capabilities specific to the `textDocumentsignatureHelp`
	SignatureHelp *SignatureHelpClientCapabilities `json:"signatureHelp,omitempty"`

	// Capabilities specific to the `textDocumentdeclaration`
	//
	// @since 3.14.0
	Declaration *DeclarationClientCapabilities `json:"declaration,omitempty"`

	// Capabilities specific to the `textDocumentdefinition`
	Definition *DefinitionClientCapabilities `json:"definition,omitempty"`

	// Capabilities specific to the `textDocumenttypeDefinition`
	//
	// @since 3.6.0
	TypeDefinition *TypeDefinitionClientCapabilities `json:"typeDefinition,omitempty"`

	// Capabilities specific to the `textDocumentimplementation`
	//
	// @since 3.6.0
	Implementation *ImplementationClientCapabilities `json:"implementation,omitempty"`

	// Capabilities specific to the `textDocumentreferences`
	References *ReferenceClientCapabilities `json:"references,omitempty"`

	// Capabilities specific to the `textDocumentdocumentHighlight`
	DocumentHighlight *DocumentHighlightClientCapabilities `json:"documentHighlight,omitempty"`

	// Capabilities specific to the `textDocumentdocumentSymbol`
	DocumentSymbol *DocumentSymbolClientCapabilities `json:"documentSymbol,omitempty"`

	// Capabilities specific to the `textDocumentcodeAction`
	CodeAction *CodeActionClientCapabilities `json:"codeAction,omitempty"`

	// Capabilities specific to the `textDocumentcodeLens`
	CodeLens *CodeLensClientCapabilities `json:"codeLens,omitempty"`

	// Capabilities specific to the `textDocumentdocumentLink`
	DocumentLink *DocumentLinkClientCapabilities `json:"documentLink,omitempty"`

	// Capabilities specific to the `textDocumentdocumentColor`
	ColorProvider *DocumentColorClientCapabilities `json:"colorProvider,omitempty"`

	// Capabilities specific to the `textDocumentformatting`
	Formatting *DocumentFormattingClientCapabilities `json:"formatting,omitempty"`

	// Capabilities specific to the `textDocumentrangeFormatting`
	RangeFormatting *DocumentRangeFormattingClientCapabilities `json:"rangeFormatting,omitempty"`

	// Capabilities specific to the `textDocumentonTypeFormatting`
	OnTypeFormatting *DocumentOnTypeFormattingClientCapabilities `json:"onTypeFormatting,omitempty"`

	// Capabilities specific to the `textDocumentrename`
	Rename *RenameClientCapabilities `json:"rename,omitempty"`

	// Capabilities specific to `textDocumentfoldingRange` request.
	//
	// @since 3.10.0
	FoldingRange *FoldingRangeClientCapabilities `json:"foldingRange,omitempty"`

	// Capabilities specific to `textDocumentselectionRange` request.
	//
	// @since 3.15.0
	SelectionRange *SelectionRangeClientCapabilities `json:"selectionRange,omitempty"`

	// Capabilities specific to `textDocumentpublishDiagnostics` notification.
	PublishDiagnostics *PublishDiagnosticsClientCapabilities `json:"publishDiagnostics,omitempty"`

	// Capabilities specific to the various call hierarchy request.
	//
	// @since 3.16.0
	CallHierarchy *CallHierarchyClientCapabilities `json:"callHierarchy,omitempty"`

	// Capabilities specific to the various semantic token request.
	//
	// @since 3.16.0
	SemanticTokens *SemanticTokensClientCapabilities `json:"semanticTokens,omitempty"`

	// Capabilities specific to the linked editing range request.
	//
	// @since 3.16.0
	LinkedEditingRange *LinkedEditingRangeClientCapabilities `json:"linkedEditingRange,omitempty"`

	// Client capabilities specific to the moniker request.
	//
	// @since 3.16.0
	Moniker *MonikerClientCapabilities `json:"moniker,omitempty"`

	// Capabilities specific to the various type hierarchy requests.
	//
	// @since 3.17.0 - proposed state
	TypeHierarchy *TypeHierarchyClientCapabilities `json:"typeHierarchy,omitempty"`

	// Capabilities specific to the `textDocumentinlineValues` request.
	//
	// @since 3.17.0 - proposed state
	InlineValues *InlineValuesClientCapabilities `json:"inlineValues,omitempty"`
}

type WindowClientCapabilities struct {

	// Whether client supports handling progress notifications. If set
	// servers are allowed to report in `workDoneProgress` property in the
	// request specific server capabilities.
	//
	// @since 3.15.0
	WorkDoneProgress *bool `json:"workDoneProgress,omitempty"`

	// Capabilities specific to the showMessage request.
	//
	// @since 3.16.0
	ShowMessage *ShowMessageRequestClientCapabilities `json:"showMessage,omitempty"`

	// Capabilities specific to the showDocument request.
	//
	// @since 3.16.0
	ShowDocument *ShowDocumentClientCapabilities `json:"showDocument,omitempty"`
}

/**
 * Client capabilities specific to regular expressions.
 *
 * @since 3.16.0
 */
type RegularExpressionsClientCapabilities struct {

	// The engine's name.
	Engine string `json:"engine,omitempty"`

	// The engine's version.
	Version *string `json:"version,omitempty"`
}

/**
 * Client capabilities specific to the used markdown parser.
 *
 * @since 3.16.0
 */
type MarkdownClientCapabilities struct {

	// The name of the parser.
	Parser string `json:"parser,omitempty"`

	// The version of the parser.
	Version *string `json:"version,omitempty"`
}

/**
 * General client capabilities.
 *
 * @since 3.16.0
 */
type GeneralClientCapabilities struct {

	// Client capability that signals how the client
	// handles stale requests (e.g. a request
	// for which the client will not process the response
	// anymore since the information is outdated).
	//
	// @since 3.17.0
	StaleRequestSupport interface{} `json:"staleRequestSupport,omitempty"` // cancel, retryOnContentModified,

	// Client capabilities specific to regular expressions.
	//
	// @since 3.16.0
	RegularExpressions *RegularExpressionsClientCapabilities `json:"regularExpressions,omitempty"`

	// Client capabilities specific to the client's markdown parser.
	//
	// @since 3.16.0
	Markdown *MarkdownClientCapabilities `json:"markdown,omitempty"`
}

/**
 * Defines the capabilities provided by the client.
 */
type _ClientCapabilities struct {

	// Workspace specific client capabilities.
	Workspace *WorkspaceClientCapabilities `json:"workspace,omitempty"`

	// Text document specific client capabilities.
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`

	// Window specific client capabilities.
	Window *WindowClientCapabilities `json:"window,omitempty"`

	// General client capabilities.
	//
	// @since 3.16.0
	General *GeneralClientCapabilities `json:"general,omitempty"`

	// Experimental client capabilities.
	Experimental interface{} `json:"experimental,omitempty"`
}

/**
 * Static registration options to be returned in the initialize
 * request.
 */
type StaticRegistrationOptions struct {

	// The id used to register the request. The id can be used to deregister
	// the request again. See also Registration#id.
	Id *string `json:"id,omitempty"`
}

/**
 * General text document registration options.
 */
type TextDocumentRegistrationOptions struct {

	// A document selector to identify the scope of the registration. If set to null
	// the document selector provided on the client side will be used.
	DocumentSelector interface{} `json:"documentSelector,omitempty"` // DocumentSelector, null,
}

/**
 * Save options.
 */
type SaveOptions struct {

	// The client is supposed to include the content on save.
	IncludeText *bool `json:"includeText,omitempty"`
}

type WorkDoneProgressOptions struct {
	WorkDoneProgress *bool `json:"workDoneProgress,omitempty"`
}

/**
 * The result returned from an initialize request.
 */
type InitializeResult struct {

	// The capabilities the language server provides.
	Capabilities ServerCapabilities `json:"capabilities,omitempty"`

	// Information about the server.
	//
	// @since 3.15.0
	ServerInfo *struct {
		Name    string  `json:"name,omitempty"`
		Version *string `json:"version,omitempty"`
	} `json:"serverInfo,omitempty"` // name, version,

	// Custom initialization results.
	Custom interface{} `json:"custom,omitempty"`
}

/**
 * The initialize parameters
 */
type _InitializeParams struct {
	WorkDoneProgressParams

	// The process Id of the parent process that started
	// the server.
	ProcessId interface{} `json:"processId,omitempty"` // int, null,

	// Information about the client
	//
	// @since 3.15.0
	ClientInfo interface{} `json:"clientInfo,omitempty"` // name, version,

	// The locale the client is currently showing the user interface
	// in. This must not necessarily be the locale of the operating
	// system.
	//
	// Uses IETF language tags as the value's syntax
	// (See https:en.wikipedia.orgwikiIETF_language_tag)
	//
	// @since 3.16.0
	Locale *string `json:"locale,omitempty"`

	// The rootPath of the workspace. Is null
	// if no folder is open.
	//
	// @deprecated in favour of rootUri.
	RootPath interface{} `json:"rootPath,omitempty"` // string, null,

	// The rootUri of the workspace. Is null if no
	// folder is open. If both `rootPath` and `rootUri` are set
	// `rootUri` wins.
	//
	// @deprecated in favour of workspaceFolders.
	RootUri interface{} `json:"rootUri,omitempty"` // DocumentUri, null,

	// The capabilities provided by the client (editor or tool)
	Capabilities ClientCapabilities `json:"capabilities,omitempty"`

	// User provided initialization options.
	InitializationOptions interface{} `json:"initializationOptions,omitempty"`

	// The initial trace setting. If omitted trace is disabled ('off').
	Trace interface{} `json:"trace,omitempty"` // interface{} // 'off', interface{} // 'messages', interface{} // 'compact', interface{} // 'verbose',
}

/**
 * The data type of the ResponseError if the
 * initialize request fails.
 */
type InitializeError struct {

	// Indicates whether the client execute the following retry logic:
	// (1) show the message provided by the ResponseError to the user
	// (2) user selects retry or cancel
	// (3) if user selected retry the initialize method is sent again.
	Retry bool `json:"retry,omitempty"`
}

// ---- Configuration notification ----
type DidChangeConfigurationClientCapabilities struct {

	// Did change configuration notification supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

type DidChangeConfigurationRegistrationOptions struct {
	Section interface{} `json:"section,omitempty"` // string, []string,
}

/**
 * The parameters of a change configuration notification.
 */
type DidChangeConfigurationParams struct {

	// The actual changed settings
	Settings interface{} `json:"settings,omitempty"`
}

/**
 * The parameters of a notification message.
 */
type ShowMessageParams struct {

	// The message type. See {@link MessageType}
	Type MessageType `json:"type,omitempty"`

	// The actual message
	Message string `json:"message,omitempty"`
}

/**
 * Show message request client capabilities
 */
type ShowMessageRequestClientCapabilities struct {

	// Capabilities specific to the `MessageActionItem` type.
	MessageActionItem *struct {

		// Whether the client supports additional attributes which
		// are preserved and send back to the server in the
		// request's response.
		AdditionalPropertiesSupport *bool `json:"additionalPropertiesSupport,omitempty"`
	} `json:"messageActionItem,omitempty"`
}

type MessageActionItem struct {

	// A short title like 'Retry', 'Open Log' etc.
	Title string `json:"title,omitempty"`

	// Additional attributes that the client preserves and
	// sends back to the server. This depends on the client
	// capability window.messageActionItem.additionalPropertiesSupport
	Key interface{} `json:"key,omitempty"` // string, bool, int, interface{},
}

type ShowMessageRequestParams struct {

	// The message type. See {@link MessageType}
	Type MessageType `json:"type,omitempty"`

	// The actual message
	Message string `json:"message,omitempty"`

	// The message action items to present.
	Actions *[]MessageActionItem `json:"actions,omitempty"`
}

/**
 * The logs message parameters.
 */
type LogMessageParams struct {

	// The message type. See {@link MessageType}
	Type MessageType `json:"type,omitempty"`

	// The actual message
	Message string `json:"message,omitempty"`
}

// ---- Text document notifications ----
type TextDocumentSyncClientCapabilities struct {

	// Whether text document synchronization supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// The client supports sending will save notifications.
	WillSave *bool `json:"willSave,omitempty"`

	// The client supports sending a will save request and
	// waits for a response providing text edits which will
	// be applied to the document before it is saved.
	WillSaveWaitUntil *bool `json:"willSaveWaitUntil,omitempty"`

	// The client supports did save notifications.
	DidSave *bool `json:"didSave,omitempty"`
}

type TextDocumentSyncOptions struct {

	// Open and close notifications are sent to the server. If omitted open close notification should not
	// be sent.
	OpenClose *bool `json:"openClose,omitempty"`

	// Change notifications are sent to the server. See TextDocumentSyncKind.None, TextDocumentSyncKind.Full
	// and TextDocumentSyncKind.Incremental. If omitted it defaults to TextDocumentSyncKind.None.
	Change *TextDocumentSyncKind `json:"change,omitempty"`

	// If present will save notifications are sent to the server. If omitted the notification should not be
	// sent.
	WillSave *bool `json:"willSave,omitempty"`

	// If present will save wait until requests are sent to the server. If omitted the request should not be
	// sent.
	WillSaveWaitUntil *bool `json:"willSaveWaitUntil,omitempty"`

	// If present save notifications are sent to the server. If omitted the notification should not be
	// sent.
	Save interface{} `json:"save,omitempty"` // bool, SaveOptions,
}

/**
 * The parameters send in a open text document notification
 */
type DidOpenTextDocumentParams struct {

	// The document that was opened.
	TextDocument TextDocumentItem `json:"textDocument,omitempty"`
}

/**
 * The change text document notification's parameters.
 */
type DidChangeTextDocumentParams struct {

	// The document that did change. The version number points
	// to the version after all provided content changes have
	// been applied.
	TextDocument VersionedTextDocumentIdentifier `json:"textDocument,omitempty"`

	// The actual content changes. The content changes describe single state changes
	// to the document. So if there are two content changes c1 (at array index 0) and
	// c2 (at array index 1) for a document in state S then c1 moves the document from
	// S to S' and c2 from S' to S''. So c1 is computed on the state S and c2 is computed
	// on the state S'.
	//
	// To mirror the content of a document using change events use the following approach:
	// - start with the same initial content
	// - apply the 'textDocumentdidChange' notifications in the order you receive them.
	// - apply the `TextDocumentContentChangeEvent`s in a single notification in the order
	// you receive them.
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges,omitempty"`
}

/**
 * An event describing a change to a text document. If range and rangeLength are omitted
 * the new text is considered to be the full content of the document.
 */
type TextDocumentContentChangeEvent struct {

	// The range of the document that changed.
	Range Range `json:"range,omitempty"`

	// The optional length of the range that got replaced.
	//
	// @deprecated use range instead.
	RangeLength *uint `json:"rangeLength,omitempty"`

	// The new text for the provided range.
	Text interface{} `json:"text,omitempty"` // string, {"text": string}
}

/**
 * Describe options to be used when registered for text document change events.
 */
type TextDocumentChangeRegistrationOptions struct {
	TextDocumentRegistrationOptions

	// How documents are synced to the server.
	SyncKind TextDocumentSyncKind `json:"syncKind,omitempty"`
}

/**
 * The parameters send in a close text document notification
 */
type DidCloseTextDocumentParams struct {

	// The document that was closed.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`
}

/**
 * The parameters send in a save text document notification
 */
type DidSaveTextDocumentParams struct {

	// The document that was closed.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// Optional the content when saved. Depends on the includeText value
	// when the save notification was requested.
	Text *string `json:"text,omitempty"`
}

/**
 * Save registration options.
 */
type TextDocumentSaveRegistrationOptions struct {
	TextDocumentRegistrationOptions
	SaveOptions
}

/**
 * The parameters send in a will save text document notification.
 */
type WillSaveTextDocumentParams struct {

	// The document that will be saved.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The 'TextDocumentSaveReason'.
	Reason TextDocumentSaveReason `json:"reason,omitempty"`
}

// ---- File eventing ----
type DidChangeWatchedFilesClientCapabilities struct {

	// Did change watched files notification supports dynamic registration. Please note
	// that the current protocol doesn't support static configuration for file changes
	// from the server side.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * The watched files change notification's parameters.
 */
type DidChangeWatchedFilesParams struct {

	// The actual file events.
	Changes []FileEvent `json:"changes,omitempty"`
}

/**
 * An event describing a file change.
 */
type FileEvent struct {

	// The file's uri.
	Uri DocumentUri `json:"uri,omitempty"`

	// The change type.
	Type FileChangeType `json:"type,omitempty"`
}

/**
 * Describe options to be used when registered for text document change events.
 */
type DidChangeWatchedFilesRegistrationOptions struct {

	// The watchers to register.
	Watchers []FileSystemWatcher `json:"watchers,omitempty"`
}

type FileSystemWatcher struct {

	// The  glob pattern to watch. Glob patterns can have the following syntax:
	// - `` to match one or more characters in a path segment
	// - `?` to match on one character in a path segment
	// - `` to match any number of path segments, including none
	// - `{}` to group conditions (e.g. `​.{ts,js}` matches all TypeScript and JavaScript files)
	// - `[]` to declare a range of characters to match in a path segment (e.g., `example.[0-9]` to match on `example.0`, `example.1`, …)
	// - `[!...]` to negate a range of characters to match in a path segment (e.g., `example.[!0-9]` to match on `example.a`, `example.b`, but not `example.0`)
	GlobPattern string `json:"globPattern,omitempty"`

	// The kind of events of interest. If omitted it defaults
	// to WatchKind.Create | WatchKind.Change | WatchKind.Delete
	// which is 7.
	Kind *uint `json:"kind,omitempty"`
}

/**
 * The publish diagnostic client capabilities.
 */
type PublishDiagnosticsClientCapabilities struct {

	// Whether the clients accepts diagnostics with related information.
	RelatedInformation *bool `json:"relatedInformation,omitempty"`

	// Client supports the tag property to provide meta data about a diagnostic.
	// Clients supporting tags have to handle unknown tags gracefully.
	//
	// @since 3.15.0
	TagSupport *struct {

		// The tags supported by the client.
		ValueSet []DiagnosticTag `json:"valueSet,omitempty"`
	} `json:"tagSupport,omitempty"`

	// Whether the client interprets the version property of the
	// `textDocumentpublishDiagnostics` notification`s parameter.
	//
	// @since 3.15.0
	VersionSupport *bool `json:"versionSupport,omitempty"`

	// Client supports a codeDescription property
	//
	// @since 3.16.0
	CodeDescriptionSupport *bool `json:"codeDescriptionSupport,omitempty"`

	// Whether code action supports the `data` property which is
	// preserved between a `textDocumentpublishDiagnostics` and
	// `textDocumentcodeAction` request.
	//
	// @since 3.16.0
	DataSupport *bool `json:"dataSupport,omitempty"`
}

/**
 * The publish diagnostic notification's parameters.
 */
type PublishDiagnosticsParams struct {

	// The URI for which diagnostic information is reported.
	Uri DocumentUri `json:"uri,omitempty"`

	// Optional the version number of the document the diagnostics are published for.
	//
	// @since 3.15.0
	Version *int `json:"version,omitempty"`

	// An array of diagnostic information items.
	Diagnostics []Diagnostic `json:"diagnostics"`
}

/**
 * Contains additional information about the context in which a completion request is triggered.
 */
type CompletionContext struct {

	// How the completion was triggered.
	TriggerKind CompletionTriggerKind `json:"triggerKind,omitempty"`

	// The trigger character (a single character) that has trigger code complete.
	// Is undefined if `triggerKind !== CompletionTriggerKind.TriggerCharacter`
	TriggerCharacter *string `json:"triggerCharacter,omitempty"`
}

/**
 * Completion parameters
 */
type CompletionParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
	PartialResultParams

	// The completion context. This is only available it the client specifies
	// to send this using the client capability `textDocument.completion.contextSupport === true`
	Context *CompletionContext `json:"context,omitempty"`
}

/**
 * Completion options.
 */
type CompletionOptions struct {
	WorkDoneProgressOptions

	// Most tools trigger completion request automatically without explicitly requesting
	// it using a keyboard shortcut (e.g. Ctrl+Space). Typically they do so when the user
	// starts to type an identifier. For example if the user types `c` in a JavaScript file
	// code complete will automatically pop up present `console` besides others as a
	// completion item. Characters that make up identifiers don't need to be listed here.
	//
	// If code complete should automatically be trigger on characters not being valid inside
	// an identifier (for example `.` in JavaScript) list them in `triggerCharacters`.
	TriggerCharacters *[]string `json:"triggerCharacters,omitempty"`

	// The list of all possible characters that commit a completion. This field can be used
	// if clients don't support individual commit characters per completion item. See
	// `ClientCapabilities.textDocument.completion.completionItem.commitCharactersSupport`
	//
	// If a server provides both `allCommitCharacters` and commit characters on an individual
	// completion item the ones on the completion item win.
	//
	// @since 3.2.0
	AllCommitCharacters *[]string `json:"allCommitCharacters,omitempty"`

	// The server provides support to resolve additional
	// information for a completion item.
	ResolveProvider *bool `json:"resolveProvider,omitempty"`

	// The server supports the following `CompletionItem` specific
	// capabilities.
	//
	// @since 3.17.0 - proposed state
	CompletionItem *struct {

		// The server has support for completion item label
		// details (see also `CompletionItemLabelDetails`) when
		// receiving a completion item in a resolve call.
		//
		// @since 3.17.0 - proposed state
		LabelDetailsSupport *bool `json:"labelDetailsSupport,omitempty"`
	} `json:"completionItem,omitempty"`
}

/**
 * Registration options for a [CompletionRequest](#CompletionRequest).
 */
type CompletionRegistrationOptions struct {
	TextDocumentRegistrationOptions
	CompletionOptions
}

// ---- Hover Support -------------------------------
type HoverClientCapabilities struct {

	// Whether hover supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// Client supports the follow content formats for the content
	// property. The order describes the preferred format of the client.
	ContentFormat *[]MarkupKind `json:"contentFormat,omitempty"`
}

/**
 * Hover options.
 */
type HoverOptions struct {
	WorkDoneProgressOptions
}

/**
 * Parameters for a [HoverRequest](#HoverRequest).
 */
type HoverParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
}

/**
 * Registration options for a [HoverRequest](#HoverRequest).
 */
type HoverRegistrationOptions struct {
	TextDocumentRegistrationOptions
	HoverOptions
}

/**
 * Client Capabilities for a [SignatureHelpRequest](#SignatureHelpRequest).
 */
type SignatureHelpClientCapabilities struct {

	// Whether signature help supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// The client supports the following `SignatureInformation`
	// specific properties.
	SignatureInformation interface{} `json:"signatureInformation,omitempty"` // documentationFormat, parameterInformation, activeParameterSupport,

	// The client supports to send additional context information for a
	// `textDocumentsignatureHelp` request. A client that opts into
	// contextSupport will also support the `retriggerCharacters` on
	// `SignatureHelpOptions`.
	//
	// @since 3.15.0
	ContextSupport *bool `json:"contextSupport,omitempty"`
}

/**
 * Server Capabilities for a [SignatureHelpRequest](#SignatureHelpRequest).
 */
type SignatureHelpOptions struct {
	WorkDoneProgressOptions

	// List of characters that trigger signature help.
	TriggerCharacters *[]string `json:"triggerCharacters,omitempty"`

	// List of characters that re-trigger signature help.
	//
	// These trigger characters are only active when signature help is already showing. All trigger characters
	// are also counted as re-trigger characters.
	//
	// @since 3.15.0
	RetriggerCharacters *[]string `json:"retriggerCharacters,omitempty"`
}

/**
 * Additional information about the context in which a signature help request was triggered.
 *
 * @since 3.15.0
 */
type SignatureHelpContext struct {

	// Action that caused signature help to be triggered.
	TriggerKind SignatureHelpTriggerKind `json:"triggerKind,omitempty"`

	// Character that caused signature help to be triggered.
	//
	// This is undefined when `triggerKind !== SignatureHelpTriggerKind.TriggerCharacter`
	TriggerCharacter *string `json:"triggerCharacter,omitempty"`

	// `true` if signature help was already showing when it was triggered.
	//
	// Retrigger occurs when the signature help is already active and can be caused by actions such as
	// typing a trigger character, a cursor move, or document content changes.
	IsRetrigger bool `json:"isRetrigger,omitempty"`

	// The currently active `SignatureHelp`.
	//
	// The `activeSignatureHelp` has its `SignatureHelp.activeSignature` field updated based on
	// the user navigating through available signatures.
	ActiveSignatureHelp *SignatureHelp `json:"activeSignatureHelp,omitempty"`
}

/**
 * Parameters for a [SignatureHelpRequest](#SignatureHelpRequest).
 */
type SignatureHelpParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams

	// The signature help context. This is only available if the client specifies
	// to send this using the client capability `textDocument.signatureHelp.contextSupport === true`
	//
	// @since 3.15.0
	Context *SignatureHelpContext `json:"context,omitempty"`
}

/**
 * Registration options for a [SignatureHelpRequest](#SignatureHelpRequest).
 */
type SignatureHelpRegistrationOptions struct {
	TextDocumentRegistrationOptions
	SignatureHelpOptions
}

/**
 * Client Capabilities for a [DefinitionRequest](#DefinitionRequest).
 */
type DefinitionClientCapabilities struct {

	// Whether definition supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// The client supports additional metadata in the form of definition links.
	//
	// @since 3.14.0
	LinkSupport *bool `json:"linkSupport,omitempty"`
}

/**
 * Server Capabilities for a [DefinitionRequest](#DefinitionRequest).
 */
type DefinitionOptions struct {
	WorkDoneProgressOptions
}

/**
 * Parameters for a [DefinitionRequest](#DefinitionRequest).
 */
type DefinitionParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
	PartialResultParams
}

/**
 * Registration options for a [DefinitionRequest](#DefinitionRequest).
 */
type DefinitionRegistrationOptions struct {
	TextDocumentRegistrationOptions
	DefinitionOptions
}

/**
 * Client Capabilities for a [ReferencesRequest](#ReferencesRequest).
 */
type ReferenceClientCapabilities struct {

	// Whether references supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * Parameters for a [ReferencesRequest](#ReferencesRequest).
 */
type ReferenceParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
	PartialResultParams

	Context ReferenceContext `json:"context,omitempty"`
}

/**
 * Reference options.
 */
type ReferenceOptions struct {
	WorkDoneProgressOptions
}

/**
 * Registration options for a [ReferencesRequest](#ReferencesRequest).
 */
type ReferenceRegistrationOptions struct {
	TextDocumentRegistrationOptions
	ReferenceOptions
}

/**
 * Client Capabilities for a [DocumentHighlightRequest](#DocumentHighlightRequest).
 */
type DocumentHighlightClientCapabilities struct {

	// Whether document highlight supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * Parameters for a [DocumentHighlightRequest](#DocumentHighlightRequest).
 */
type DocumentHighlightParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
	PartialResultParams
}

/**
 * Provider options for a [DocumentHighlightRequest](#DocumentHighlightRequest).
 */
type DocumentHighlightOptions struct {
	WorkDoneProgressOptions
}

/**
 * Registration options for a [DocumentHighlightRequest](#DocumentHighlightRequest).
 */
type DocumentHighlightRegistrationOptions struct {
	TextDocumentRegistrationOptions
	DocumentHighlightOptions
}

/**
 * Client Capabilities for a [DocumentSymbolRequest](#DocumentSymbolRequest).
 */
type DocumentSymbolClientCapabilities struct {

	// Whether document symbol supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// Specific capabilities for the `SymbolKind`.
	SymbolKind *struct {

		// The symbol kind values the client supports. When this
		// property exists the client also guarantees that it will
		// handle values outside its set gracefully and falls back
		// to a default value when unknown.
		//
		// If this property is not present the client only supports
		// the symbol kinds from `File` to `Array` as defined in
		// the initial version of the protocol.
		ValueSet *[]SymbolKind `json:"valueSet,omitempty"`
	} `json:"symbolKind,omitempty"`

	// The client support hierarchical document symbols.
	HierarchicalDocumentSymbolSupport *bool `json:"hierarchicalDocumentSymbolSupport,omitempty"`

	// The client supports tags on `SymbolInformation`. Tags are supported on
	// `DocumentSymbol` if `hierarchicalDocumentSymbolSupport` is set to true.
	// Clients supporting tags have to handle unknown tags gracefully.
	//
	// @since 3.16.0
	TagSupport *struct {

		// The tags supported by the client.
		ValueSet []SymbolTag `json:"valueSet,omitempty"`
	} `json:"tagSupport,omitempty"`

	// The client supports an additional label presented in the UI when
	// registering a document symbol provider.
	//
	// @since 3.16.0
	LabelSupport *bool `json:"labelSupport,omitempty"`
}

/**
 * Parameters for a [DocumentSymbolRequest](#DocumentSymbolRequest).
 */
type DocumentSymbolParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The text document.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`
}

/**
 * Provider options for a [DocumentSymbolRequest](#DocumentSymbolRequest).
 */
type DocumentSymbolOptions struct {
	WorkDoneProgressOptions

	// A human-readable string that is shown when multiple outlines trees
	// are shown for the same document.
	//
	// @since 3.16.0
	Label *string `json:"label,omitempty"`
}

/**
 * Registration options for a [DocumentSymbolRequest](#DocumentSymbolRequest).
 */
type DocumentSymbolRegistrationOptions struct {
	TextDocumentRegistrationOptions
	DocumentSymbolOptions
}

/**
 * The Client Capabilities of a [CodeActionRequest](#CodeActionRequest).
 */
type CodeActionClientCapabilities struct {

	// Whether code action supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// The client support code action literals of type `CodeAction` as a valid
	// response of the `textDocumentcodeAction` request. If the property is not
	// set the request can only return `Command` literals.
	//
	// @since 3.8.0
	CodeActionLiteralSupport *struct {

		// The code action kind is support with the following value
		// set.
		CodeActionKind struct {

			// The code action kind values the client supports. When this
			// property exists the client also guarantees that it will
			// handle values outside its set gracefully and falls back
			// to a default value when unknown.
			ValueSet []CodeActionKind `json:"valueSet,omitempty"`
		} `json:"codeActionKind,omitempty"`
	} `json:"codeActionLiteralSupport,omitempty"`

	// Whether code action supports the `isPreferred` property.
	//
	// @since 3.15.0
	IsPreferredSupport *bool `json:"isPreferredSupport,omitempty"`

	// Whether code action supports the `disabled` property.
	//
	// @since 3.16.0
	DisabledSupport *bool `json:"disabledSupport,omitempty"`

	// Whether code action supports the `data` property which is
	// preserved between a `textDocumentcodeAction` and a
	// `codeActionresolve` request.
	//
	// @since 3.16.0
	DataSupport *bool `json:"dataSupport,omitempty"`

	// Whether the client support resolving additional code action
	// properties via a separate `codeActionresolve` request.
	//
	// @since 3.16.0
	ResolveSupport *struct {

		// The properties that a client can resolve lazily.
		Properties []string `json:"properties,omitempty"`
	} `json:"resolveSupport,omitempty"`

	// Whether th client honors the change annotations in
	// text edits and resource operations returned via the
	// `CodeAction#edit` property by for example presenting
	// the workspace edit in the user interface and asking
	// for confirmation.
	//
	// @since 3.16.0
	HonorsChangeAnnotations *bool `json:"honorsChangeAnnotations,omitempty"`
}

/**
 * The parameters of a [CodeActionRequest](#CodeActionRequest).
 */
type CodeActionParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The document in which the command was invoked.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The range for which the command was invoked.
	Range Range `json:"range,omitempty"`

	// Context carrying additional information.
	Context CodeActionContext `json:"context,omitempty"`
}

/**
 * Provider options for a [CodeActionRequest](#CodeActionRequest).
 */
type CodeActionOptions struct {
	WorkDoneProgressOptions

	// CodeActionKinds that this server may return.
	//
	// The list of kinds may be generic, such as `CodeActionKind.Refactor`, or the server
	// may list out every specific kind they provide.
	CodeActionKinds *[]CodeActionKind `json:"codeActionKinds,omitempty"`

	// The server provides support to resolve additional
	// information for a code action.
	//
	// @since 3.16.0
	ResolveProvider *bool `json:"resolveProvider,omitempty"`
}

/**
 * Registration options for a [CodeActionRequest](#CodeActionRequest).
 */
type CodeActionRegistrationOptions struct {
	TextDocumentRegistrationOptions
	CodeActionOptions
}

/**
 * Client capabilities for a [WorkspaceSymbolRequest](#WorkspaceSymbolRequest).
 */
type WorkspaceSymbolClientCapabilities struct {

	// Symbol request supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// Specific capabilities for the `SymbolKind` in the `workspacesymbol` request.
	SymbolKind *struct {

		// The symbol kind values the client supports. When this
		// property exists the client also guarantees that it will
		// handle values outside its set gracefully and falls back
		// to a default value when unknown.
		//
		// If this property is not present the client only supports
		// the symbol kinds from `File` to `Array` as defined in
		// the initial version of the protocol.
		ValueSet *[]SymbolKind `json:"valueSet,omitempty"`
	} `json:"symbolKind,omitempty"`

	// The client supports tags on `SymbolInformation`.
	// Clients supporting tags have to handle unknown tags gracefully.
	//
	// @since 3.16.0
	TagSupport *struct {

		// The tags supported by the client.
		ValueSet []SymbolTag `json:"valueSet,omitempty"`
	} `json:"tagSupport,omitempty"`
}

/**
 * The parameters of a [WorkspaceSymbolRequest](#WorkspaceSymbolRequest).
 */
type WorkspaceSymbolParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// A query string to filter symbols by. Clients may send an empty
	// string here to request all symbols.
	Query string `json:"query,omitempty"`
}

/**
 * Server capabilities for a [WorkspaceSymbolRequest](#WorkspaceSymbolRequest).
 */
type WorkspaceSymbolOptions struct {
	WorkDoneProgressOptions
}

/**
 * Registration options for a [WorkspaceSymbolRequest](#WorkspaceSymbolRequest).
 */
type WorkspaceSymbolRegistrationOptions struct {
	WorkspaceSymbolOptions
}

/**
 * The client capabilities  of a [CodeLensRequest](#CodeLensRequest).
 */
type CodeLensClientCapabilities struct {

	// Whether code lens supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * @since 3.16.0
 */
type CodeLensWorkspaceClientCapabilities struct {

	// Whether the client implementation supports a refresh request sent from the
	// server to the client.
	//
	// Note that this event is global and will force the client to refresh all
	// code lenses currently shown. It should be used with absolute care and is
	// useful for situation where a server for example detect a project wide
	// change that requires such a calculation.
	RefreshSupport *bool `json:"refreshSupport,omitempty"`
}

/**
 * The parameters of a [CodeLensRequest](#CodeLensRequest).
 */
type CodeLensParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The document to request code lens for.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`
}

/**
 * Code Lens provider options of a [CodeLensRequest](#CodeLensRequest).
 */
type CodeLensOptions struct {
	WorkDoneProgressOptions

	// Code lens has a resolve provider as well.
	ResolveProvider *bool `json:"resolveProvider,omitempty"`
}

/**
 * Registration options for a [CodeLensRequest](#CodeLensRequest).
 */
type CodeLensRegistrationOptions struct {
	TextDocumentRegistrationOptions
	CodeLensOptions
}

/**
 * The client capabilities of a [DocumentLinkRequest](#DocumentLinkRequest).
 */
type DocumentLinkClientCapabilities struct {

	// Whether document link supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// Whether the client support the `tooltip` property on `DocumentLink`.
	//
	// @since 3.15.0
	TooltipSupport *bool `json:"tooltipSupport,omitempty"`
}

/**
 * The parameters of a [DocumentLinkRequest](#DocumentLinkRequest).
 */
type DocumentLinkParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The document to provide document links for.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`
}

/**
 * Provider options for a [DocumentLinkRequest](#DocumentLinkRequest).
 */
type DocumentLinkOptions struct {
	WorkDoneProgressOptions

	// Document links have a resolve provider as well.
	ResolveProvider *bool `json:"resolveProvider,omitempty"`
}

/**
 * Registration options for a [DocumentLinkRequest](#DocumentLinkRequest).
 */
type DocumentLinkRegistrationOptions struct {
	TextDocumentRegistrationOptions
	DocumentLinkOptions
}

/**
 * Client capabilities of a [DocumentFormattingRequest](#DocumentFormattingRequest).
 */
type DocumentFormattingClientCapabilities struct {

	// Whether formatting supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * The parameters of a [DocumentFormattingRequest](#DocumentFormattingRequest).
 */
type DocumentFormattingParams struct {
	WorkDoneProgressParams

	// The document to format.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The format options
	Options FormattingOptions `json:"options,omitempty"`
}

/**
 * Provider options for a [DocumentFormattingRequest](#DocumentFormattingRequest).
 */
type DocumentFormattingOptions struct {
	WorkDoneProgressOptions
}

/**
 * Registration options for a [DocumentFormattingRequest](#DocumentFormattingRequest).
 */
type DocumentFormattingRegistrationOptions struct {
	TextDocumentRegistrationOptions
	DocumentFormattingOptions
}

/**
 * Client capabilities of a [DocumentRangeFormattingRequest](#DocumentRangeFormattingRequest).
 */
type DocumentRangeFormattingClientCapabilities struct {

	// Whether range formatting supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * The parameters of a [DocumentRangeFormattingRequest](#DocumentRangeFormattingRequest).
 */
type DocumentRangeFormattingParams struct {
	WorkDoneProgressParams

	// The document to format.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The range to format
	Range Range `json:"range,omitempty"`

	// The format options
	Options FormattingOptions `json:"options,omitempty"`
}

/**
 * Provider options for a [DocumentRangeFormattingRequest](#DocumentRangeFormattingRequest).
 */
type DocumentRangeFormattingOptions struct {
	WorkDoneProgressOptions
}

/**
 * Registration options for a [DocumentRangeFormattingRequest](#DocumentRangeFormattingRequest).
 */
type DocumentRangeFormattingRegistrationOptions struct {
	TextDocumentRegistrationOptions
	DocumentRangeFormattingOptions
}

/**
 * Client capabilities of a [DocumentOnTypeFormattingRequest](#DocumentOnTypeFormattingRequest).
 */
type DocumentOnTypeFormattingClientCapabilities struct {

	// Whether on type formatting supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * The parameters of a [DocumentOnTypeFormattingRequest](#DocumentOnTypeFormattingRequest).
 */
type DocumentOnTypeFormattingParams struct {

	// The document to format.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The position at which this request was send.
	Position Position `json:"position,omitempty"`

	// The character that has been typed.
	Ch string `json:"ch,omitempty"`

	// The format options.
	Options FormattingOptions `json:"options,omitempty"`
}

/**
 * Provider options for a [DocumentOnTypeFormattingRequest](#DocumentOnTypeFormattingRequest).
 */
type DocumentOnTypeFormattingOptions struct {

	// A character on which formatting should be triggered, like `}`.
	FirstTriggerCharacter string `json:"firstTriggerCharacter,omitempty"`

	// More trigger characters.
	MoreTriggerCharacter *[]string `json:"moreTriggerCharacter,omitempty"`
}

/**
 * Registration options for a [DocumentOnTypeFormattingRequest](#DocumentOnTypeFormattingRequest).
 */
type DocumentOnTypeFormattingRegistrationOptions struct {
	TextDocumentRegistrationOptions
	DocumentOnTypeFormattingOptions
}

type RenameClientCapabilities struct {

	// Whether rename supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// Client supports testing for validity of rename operations
	// before execution.
	//
	// @since 3.12.0
	PrepareSupport *bool `json:"prepareSupport,omitempty"`

	// Client supports the default behavior result.
	//
	// The value indicates the default behavior used by the
	// client.
	//
	// @since 3.16.0
	PrepareSupportDefaultBehavior *PrepareSupportDefaultBehavior `json:"prepareSupportDefaultBehavior,omitempty"`

	// Whether th client honors the change annotations in
	// text edits and resource operations returned via the
	// rename request's workspace edit by for example presenting
	// the workspace edit in the user interface and asking
	// for confirmation.
	//
	// @since 3.16.0
	HonorsChangeAnnotations *bool `json:"honorsChangeAnnotations,omitempty"`
}

/**
 * The parameters of a [RenameRequest](#RenameRequest).
 */
type RenameParams struct {
	WorkDoneProgressParams

	// The document to rename.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The position at which this request was sent.
	Position Position `json:"position,omitempty"`

	// The new name of the symbol. If the given name is not valid the
	// request must return a [ResponseError](#ResponseError) with an
	// appropriate message set.
	NewName string `json:"newName,omitempty"`
}

/**
 * Provider options for a [RenameRequest](#RenameRequest).
 */
type RenameOptions struct {
	WorkDoneProgressOptions

	// Renames should be checked and tested before being executed.
	//
	// @since version 3.12.0
	PrepareProvider *bool `json:"prepareProvider,omitempty"`
}

/**
 * Registration options for a [RenameRequest](#RenameRequest).
 */
type RenameRegistrationOptions struct {
	TextDocumentRegistrationOptions
	RenameOptions
}

type PrepareRenameParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
}

/**
 * The client capabilities of a [ExecuteCommandRequest](#ExecuteCommandRequest).
 */
type ExecuteCommandClientCapabilities struct {

	// Execute command supports dynamic registration.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * The parameters of a [ExecuteCommandRequest](#ExecuteCommandRequest).
 */
type ExecuteCommandParams struct {
	WorkDoneProgressParams

	// The identifier of the actual command handler.
	Command string `json:"command,omitempty"`

	// Arguments that the command should be invoked with.
	Arguments *[]interface{} `json:"arguments,omitempty"`
}

/**
 * The server capabilities of a [ExecuteCommandRequest](#ExecuteCommandRequest).
 */
type ExecuteCommandOptions struct {
	WorkDoneProgressOptions

	// The commands to be executed on the server
	Commands []string `json:"commands,omitempty"`
}

/**
 * Registration options for a [ExecuteCommandRequest](#ExecuteCommandRequest).
 */
type ExecuteCommandRegistrationOptions struct {
	ExecuteCommandOptions
}

// ---- Apply Edit request ----------------------------------------
type WorkspaceEditClientCapabilities struct {

	// The client supports versioned document changes in `WorkspaceEdit`s
	DocumentChanges *bool `json:"documentChanges,omitempty"`

	// The resource operations the client supports. Clients should at least
	// support 'create', 'rename' and 'delete' files and folders.
	//
	// @since 3.13.0
	ResourceOperations *[]ResourceOperationKind `json:"resourceOperations,omitempty"`

	// The failure handling strategy of a client if applying the workspace edit
	// fails.
	//
	// @since 3.13.0
	FailureHandling *FailureHandlingKind `json:"failureHandling,omitempty"`

	// Whether the client normalizes line endings to the client specific
	// setting.
	// If set to `true` the client will normalize line ending characters
	// in a workspace edit containing to the client specific new line
	// character.
	//
	// @since 3.16.0
	NormalizesLineEndings *bool `json:"normalizesLineEndings,omitempty"`

	// Whether the client in general supports change annotations on text edits,
	// create file, rename file and delete file changes.
	//
	// @since 3.16.0
	ChangeAnnotationSupport *struct {

		// Whether the client groups edits with equal labels into tree nodes,
		// for instance all edits labelled with "Changes in Strings" would
		// be a tree node.
		GroupsOnLabel *bool `json:"groupsOnLabel,omitempty"`
	} `json:"changeAnnotationSupport,omitempty"`
}

/**
 * The parameters passed via a apply workspace edit request.
 */
type ApplyWorkspaceEditParams struct {

	// An optional label of the workspace edit. This label is
	// presented in the user interface for example on an undo
	// stack to undo the workspace edit.
	Label *string `json:"label,omitempty"`

	// The edits to apply.
	Edit WorkspaceEdit `json:"edit,omitempty"`
}

/**
 * The result returned from the apply workspace edit request.
 *
 * @since 3.17 renamed from ApplyWorkspaceEditResponse
 */
type ApplyWorkspaceEditResult struct {

	// Indicates whether the edit was applied or not.
	Applied bool `json:"applied,omitempty"`

	// An optional textual description for why the edit was not applied.
	// This may be used by the server for diagnostic logging or to provide
	// a suitable error for a request that triggered the edit.
	FailureReason *string `json:"failureReason,omitempty"`

	// Depending on the client's failure handling strategy `failedChange` might
	// contain the index of the change that failed. This property is only available
	// if the client signals a `failureHandlingStrategy` in its client capabilities.
	FailedChange *uint `json:"failedChange,omitempty"`
}

/**
 * The `client/registerCapability` request is sent from the server to the client to register a new capability
 * handler on the client side.
 */
type RegistrationRequest string

const (
	RegistrationRequestType RegistrationRequest = "new ProtocolRequestType<RegistrationParams, void, never, void, void>('client/registerCapability')"
)

/**
 * The `client/unregisterCapability` request is sent from the server to the client to unregister a previously registered capability
 * handler on the client side.
 */
type UnregistrationRequest string

const (
	UnregistrationRequestType UnregistrationRequest = "new ProtocolRequestType<UnregistrationParams, void, never, void, void>('client/unregisterCapability')"
)

type ResourceOperationKind string

const (
	/**
	 * Supports creating new files and folders.
	 */
	ResourceOperationKindCreate ResourceOperationKind = "create"
	/**
	 * Supports renaming existing files and folders.
	 */
	ResourceOperationKindRename ResourceOperationKind = "rename"
	/**
	 * Supports deleting existing files and folders.
	 */
	ResourceOperationKindDelete ResourceOperationKind = "delete"
)

type FailureHandlingKind string

const (
	/**
	 * Applying the workspace change is simply aborted if one of the changes provided
	 * fails. All operations executed before the failing operation stay executed.
	 */
	FailureHandlingKindAbort FailureHandlingKind = "abort"
	/**
	 * All operations are executed transactional. That means they either all
	 * succeed or no changes at all are applied to the workspace.
	 */
	FailureHandlingKindTransactional FailureHandlingKind = "transactional"
	/**
	 * If the workspace edit contains only textual file changes they are executed transactional.
	 * If resource changes (create, rename or delete file) are part of the change the failure
	 * handling strategy is abort.
	 */
	FailureHandlingKindTextOnlyTransactional FailureHandlingKind = "textOnlyTransactional"
	/**
	 * The client tries to undo the operations already executed. But there is no
	 * guarantee that this is succeeding.
	 */
	FailureHandlingKindUndo FailureHandlingKind = "undo"
)

/**
 * The initialize request is sent from the client to the server.
 * It is sent once as the request after starting up the server.
 * The requests parameter is of type [InitializeParams](#InitializeParams)
 * the response if of type [InitializeResult](#InitializeResult) of a Thenable that
 * resolves to such.
 */
type InitializeRequest string

const (
	InitializeRequestType InitializeRequest = "new ProtocolRequestType<InitializeParams & WorkDoneProgressParams, InitializeResult, never, InitializeError, void>('initialize')"
)

/**
 * Known error codes for an `InitializeError`;
 */
type InitializeErrorCode int

var initializeErrorStringMap = map[InitializeErrorCode]string{
	InitializeErrorUnknownProtocolVersion: "unknownProtocolVersion",
}

func (i InitializeErrorCode) String() string {
	if s, ok := initializeErrorStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * If the protocol version provided by the client can't be handled by the server.
	 * @deprecated This initialize error got replaced by client capabilities. There is
	 * no version handshake in version 3.0x
	 */
	InitializeErrorUnknownProtocolVersion InitializeErrorCode = 1
)

/**
 * The initialized notification is sent from the client to the
 * server after the client is fully initialized and the server
 * is allowed to send requests from the server to the client.
 */
type InitializedNotification string

const (
	InitializedNotificationType InitializedNotification = "new ProtocolNotificationType<InitializedParams, void>('initialized')"
)

/**
 * A shutdown request is sent from the client to the server.
 * It is sent once when the client decides to shutdown the
 * server. The only notification that is sent after a shutdown request
 * is the exit event.
 */
type ShutdownRequest string

const (
	ShutdownRequestType ShutdownRequest = "new ProtocolRequestType0<void, never, void, void>('shutdown')"
)

/**
 * The exit event is sent from the client to the server to
 * ask the server to exit its process.
 */
type ExitNotification string

const (
	ExitNotificationType ExitNotification = "new ProtocolNotificationType0<void>('exit')"
)

/**
 * The configuration change notification is sent from the client to the server
 * when the client's configuration has changed. The notification contains
 * the changed configuration as defined by the language client.
 */
type DidChangeConfigurationNotification string

const (
	DidChangeConfigurationNotificationType DidChangeConfigurationNotification = "new ProtocolNotificationType<DidChangeConfigurationParams, DidChangeConfigurationRegistrationOptions>('workspace/didChangeConfiguration')"
)

/**
 * The message type
 */
type MessageType int

var messageTypeStringMap = map[MessageType]string{
	MessageTypeError:   "Error",
	MessageTypeWarning: "Warning",
	MessageTypeInfo:    "Info",
	MessageTypeLog:     "Log",
}

func (i MessageType) String() string {
	if s, ok := messageTypeStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * An error message.
	 */
	MessageTypeError MessageType = 1
	/**
	 * A warning message.
	 */
	MessageTypeWarning MessageType = 2
	/**
	 * An information message.
	 */
	MessageTypeInfo MessageType = 3
	/**
	 * A logs message.
	 */
	MessageTypeLog MessageType = 4
)

/**
 * The show message notification is sent from a server to a client to ask
 * the client to display a particular message in the user interface.
 */
type ShowMessageNotification string

const (
	ShowMessageNotificationType ShowMessageNotification = "new ProtocolNotificationType<ShowMessageParams, void>('window/showMessage')"
)

/**
 * The show message request is sent from the server to the client to show a message
 * and a set of options actions to the user.
 */
type ShowMessageRequest string

const (
	ShowMessageRequestType ShowMessageRequest = "new ProtocolRequestType<ShowMessageRequestParams, MessageActionItem | null, never, void, void>('window/showMessageRequest')"
)

/**
 * The logs message notification is sent from the server to the client to ask
 * the client to logs a particular message.
 */
type LogMessageNotification string

const (
	LogMessageNotificationType LogMessageNotification = "new ProtocolNotificationType<LogMessageParams, void>('window/logMessage')"
)

/**
 * The telemetry event notification is sent from the server to the client to ask
 * the client to logs telemetry data.
 */
type TelemetryEventNotification string

const (
	TelemetryEventNotificationType TelemetryEventNotification = "new ProtocolNotificationType<any, void>('telemetry/event')"
)

/**
 * Defines how the host (editor) should sync
 * document changes to the language server.
 */
type TextDocumentSyncKind int

var textDocumentSyncKindStringMap = map[TextDocumentSyncKind]string{
	TextDocumentSyncKindNone:        "None",
	TextDocumentSyncKindFull:        "Full",
	TextDocumentSyncKindIncremental: "Incremental",
}

func (i TextDocumentSyncKind) String() string {
	if s, ok := textDocumentSyncKindStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * Documents should not be synced at all.
	 */
	TextDocumentSyncKindNone TextDocumentSyncKind = 0
	/**
	 * Documents are synced by always sending the full content
	 * of the document.
	 */
	TextDocumentSyncKindFull TextDocumentSyncKind = 1
	/**
	 * Documents are synced by sending the full content on open.
	 * After that only incremental updates to the document are
	 * send.
	 */
	TextDocumentSyncKindIncremental TextDocumentSyncKind = 2
)

/**
 * The document open notification is sent from the client to the server to signal
 * newly opened text documents. The document's truth is now managed by the client
 * and the server must not try to read the document's truth using the document's
 * uri. Open in this sense means it is managed by the client. It doesn't necessarily
 * mean that its content is presented in an editor. An open notification must not
 * be sent more than once without a corresponding close notification send before.
 * This means open and close notification must be balanced and the max open count
 * is one.
 */
type DidOpenTextDocumentNotification string

const (
	DidOpenTextDocumentNotificationMethod DidOpenTextDocumentNotification = "textDocument/didOpen"

	DidOpenTextDocumentNotificationType DidOpenTextDocumentNotification = "new ProtocolNotificationType<DidOpenTextDocumentParams, TextDocumentRegistrationOptions>(method)"
)

/**
 * The document change notification is sent from the client to the server to signal
 * changes to a text document.
 */
type DidChangeTextDocumentNotification string

const (
	DidChangeTextDocumentNotificationMethod DidChangeTextDocumentNotification = "textDocument/didChange"

	DidChangeTextDocumentNotificationType DidChangeTextDocumentNotification = "new ProtocolNotificationType<DidChangeTextDocumentParams, TextDocumentChangeRegistrationOptions>(method)"
)

/**
 * The document close notification is sent from the client to the server when
 * the document got closed in the client. The document's truth now exists where
 * the document's uri points to (e.g. if the document's uri is a file uri the
 * truth now exists on disk). As with the open notification the close notification
 * is about managing the document's content. Receiving a close notification
 * doesn't mean that the document was open in an editor before. A close
 * notification requires a previous open notification to be sent.
 */
type DidCloseTextDocumentNotification string

const (
	DidCloseTextDocumentNotificationMethod DidCloseTextDocumentNotification = "textDocument/didClose"

	DidCloseTextDocumentNotificationType DidCloseTextDocumentNotification = "new ProtocolNotificationType<DidCloseTextDocumentParams, TextDocumentRegistrationOptions>(method)"
)

/**
 * The document save notification is sent from the client to the server when
 * the document got saved in the client.
 */
type DidSaveTextDocumentNotification string

const (
	DidSaveTextDocumentNotificationMethod DidSaveTextDocumentNotification = "textDocument/didSave"

	DidSaveTextDocumentNotificationType DidSaveTextDocumentNotification = "new ProtocolNotificationType<DidSaveTextDocumentParams, TextDocumentSaveRegistrationOptions>(method)"
)

/**
 * Represents reasons why a text document is saved.
 */
type TextDocumentSaveReason int

var textDocumentSaveReasonStringMap = map[TextDocumentSaveReason]string{
	TextDocumentSaveReasonManual:     "Manual",
	TextDocumentSaveReasonAfterDelay: "AfterDelay",
	TextDocumentSaveReasonFocusOut:   "FocusOut",
}

func (i TextDocumentSaveReason) String() string {
	if s, ok := textDocumentSaveReasonStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * Manually triggered, e.g. by the user pressing save, by starting debugging,
	 * or by an API call.
	 */
	TextDocumentSaveReasonManual TextDocumentSaveReason = 1
	/**
	 * Automatic after a delay.
	 */
	TextDocumentSaveReasonAfterDelay TextDocumentSaveReason = 2
	/**
	 * When the editor lost focus.
	 */
	TextDocumentSaveReasonFocusOut TextDocumentSaveReason = 3
)

/**
 * A document will save notification is sent from the client to the server before
 * the document is actually saved.
 */
type WillSaveTextDocumentNotification string

const (
	WillSaveTextDocumentNotificationMethod WillSaveTextDocumentNotification = "textDocument/willSave"

	WillSaveTextDocumentNotificationType WillSaveTextDocumentNotification = "new ProtocolNotificationType<WillSaveTextDocumentParams, TextDocumentRegistrationOptions>(method)"
)

/**
 * A document will save request is sent from the client to the server before
 * the document is actually saved. The request can return an array of TextEdits
 * which will be applied to the text document before it is saved. Please note that
 * clients might drop results if computing the text edits took too long or if a
 * server constantly fails on this request. This is done to keep the save fast and
 * reliable.
 */
type WillSaveTextDocumentWaitUntilRequest string

const (
	WillSaveTextDocumentWaitUntilRequestMethod WillSaveTextDocumentWaitUntilRequest = "textDocument/willSaveWaitUntil"

	WillSaveTextDocumentWaitUntilRequestType WillSaveTextDocumentWaitUntilRequest = "new ProtocolRequestType<WillSaveTextDocumentParams, TextEdit[] | null, never, void, TextDocumentRegistrationOptions>(method)"
)

/**
 * The watched files notification is sent from the client to the server when
 * the client detects changes to file watched by the language client.
 */
type DidChangeWatchedFilesNotification string

const (
	DidChangeWatchedFilesNotificationType DidChangeWatchedFilesNotification = "new ProtocolNotificationType<DidChangeWatchedFilesParams, DidChangeWatchedFilesRegistrationOptions>('workspace/didChangeWatchedFiles')"
)

/**
 * The file event type
 */
type FileChangeType int

var fileChangeTypeStringMap = map[FileChangeType]string{
	FileChangeTypeCreated: "Created",
	FileChangeTypeChanged: "Changed",
	FileChangeTypeDeleted: "Deleted",
}

func (i FileChangeType) String() string {
	if s, ok := fileChangeTypeStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * The file got created.
	 */
	FileChangeTypeCreated FileChangeType = 1
	/**
	 * The file got changed.
	 */
	FileChangeTypeChanged FileChangeType = 2
	/**
	 * The file got deleted.
	 */
	FileChangeTypeDeleted FileChangeType = 3
)

type WatchKind int

var watchKindStringMap = map[WatchKind]string{
	WatchKindCreate: "Create",
	WatchKindChange: "Change",
	WatchKindDelete: "Delete",
}

func (i WatchKind) String() string {
	if s, ok := watchKindStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * Interested in create events.
	 */
	WatchKindCreate WatchKind = 1
	/**
	 * Interested in change events
	 */
	WatchKindChange WatchKind = 2
	/**
	 * Interested in delete events
	 */
	WatchKindDelete WatchKind = 4
)

/**
 * Diagnostics notification are sent from the server to the client to signal
 * results of validation runs.
 */
type PublishDiagnosticsNotification string

const (
	PublishDiagnosticsNotificationType PublishDiagnosticsNotification = "new ProtocolNotificationType<PublishDiagnosticsParams, void>('textDocument/publishDiagnostics')"
)

/**
 * How a completion was triggered
 */
type CompletionTriggerKind int

var completionTriggerKindStringMap = map[CompletionTriggerKind]string{
	CompletionTriggerKindInvoked:                         "Invoked",
	CompletionTriggerKindTriggerCharacter:                "TriggerCharacter",
	CompletionTriggerKindTriggerForIncompleteCompletions: "TriggerForIncompleteCompletions",
}

func (i CompletionTriggerKind) String() string {
	if s, ok := completionTriggerKindStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * Completion was triggered by typing an identifier (24x7 code
	 * complete), manual invocation (e.g Ctrl+Space) or via API.
	 */
	CompletionTriggerKindInvoked CompletionTriggerKind = 1
	/**
	 * Completion was triggered by a trigger character specified by
	 * the `triggerCharacters` properties of the `CompletionRegistrationOptions`.
	 */
	CompletionTriggerKindTriggerCharacter CompletionTriggerKind = 2
	/**
	 * Completion was re-triggered as current completion list is incomplete
	 */
	CompletionTriggerKindTriggerForIncompleteCompletions CompletionTriggerKind = 3
)

/**
 * Request to request completion at a given text document position. The request's
 * parameter is of type [TextDocumentPosition](#TextDocumentPosition) the response
 * is of type [CompletionItem[]](#CompletionItem) or [CompletionList](#CompletionList)
 * or a Thenable that resolves to such.
 *
 * The request can delay the computation of the [`detail`](#CompletionItem.detail)
 * and [`documentation`](#CompletionItem.documentation) properties to the `completionItem/resolve`
 * request. However, properties that are needed for the initial sorting and filtering, like `sortText`,
 * `filterText`, `insertText`, and `textEdit`, must not be changed during resolve.
 */
type CompletionRequest string

const (
	CompletionRequestMethod CompletionRequest = "textDocument/completion"

	CompletionRequestType CompletionRequest = "new ProtocolRequestType<CompletionParams, CompletionItem[] | CompletionList | null, CompletionItem[], void, CompletionRegistrationOptions>(method)"
)

/**
 * Request to resolve additional information for a given completion item.The request's
 * parameter is of type [CompletionItem](#CompletionItem) the response
 * is of type [CompletionItem](#CompletionItem) or a Thenable that resolves to such.
 */
type CompletionResolveRequest string

const (
	CompletionResolveRequestMethod CompletionResolveRequest = "completionItem/resolve"

	CompletionResolveRequestType CompletionResolveRequest = "new ProtocolRequestType<CompletionItem, CompletionItem, never, void, void>(method)"
)

/**
 * Request to request hover information at a given text document position. The request's
 * parameter is of type [TextDocumentPosition](#TextDocumentPosition) the response is of
 * type [Hover](#Hover) or a Thenable that resolves to such.
 */
type HoverRequest string

const (
	HoverRequestMethod HoverRequest = "textDocument/hover"

	HoverRequestType HoverRequest = "new ProtocolRequestType<HoverParams, Hover | null, never, void, HoverRegistrationOptions>(method)"
)

/**
 * How a signature help was triggered.
 *
 * @since 3.15.0
 */
type SignatureHelpTriggerKind int

var signatureHelpTriggerKindStringMap = map[SignatureHelpTriggerKind]string{
	SignatureHelpTriggerKindInvoked:          "Invoked",
	SignatureHelpTriggerKindTriggerCharacter: "TriggerCharacter",
	SignatureHelpTriggerKindContentChange:    "ContentChange",
}

func (i SignatureHelpTriggerKind) String() string {
	if s, ok := signatureHelpTriggerKindStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * Signature help was invoked manually by the user or by a command.
	 */
	SignatureHelpTriggerKindInvoked SignatureHelpTriggerKind = 1
	/**
	 * Signature help was triggered by a trigger character.
	 */
	SignatureHelpTriggerKindTriggerCharacter SignatureHelpTriggerKind = 2
	/**
	 * Signature help was triggered by the cursor moving or by the document content changing.
	 */
	SignatureHelpTriggerKindContentChange SignatureHelpTriggerKind = 3
)

type SignatureHelpRequest string

const (
	SignatureHelpRequestMethod SignatureHelpRequest = "textDocument/signatureHelp"

	SignatureHelpRequestType SignatureHelpRequest = "new ProtocolRequestType<SignatureHelpParams, SignatureHelp | null, never, void, SignatureHelpRegistrationOptions>(method)"
)

/**
 * A request to resolve the definition location of a symbol at a given text
 * document position. The request's parameter is of type [TextDocumentPosition]
 * (#TextDocumentPosition) the response is of either type [Definition](#Definition)
 * or a typed array of [DefinitionLink](#DefinitionLink) or a Thenable that resolves
 * to such.
 */
type DefinitionRequest string

const (
	DefinitionRequestMethod DefinitionRequest = "textDocument/definition"

	DefinitionRequestType DefinitionRequest = "new ProtocolRequestType<DefinitionParams, Definition | DefinitionLink[] | null, Location[] | DefinitionLink[], void, DefinitionRegistrationOptions>(method)"
)

/**
 * A request to resolve project-wide references for the symbol denoted
 * by the given text document position. The request's parameter is of
 * type [ReferenceParams](#ReferenceParams) the response is of type
 * [Location[]](#Location) or a Thenable that resolves to such.
 */
type ReferencesRequest string

const (
	ReferencesRequestMethod ReferencesRequest = "textDocument/references"

	ReferencesRequestType ReferencesRequest = "new ProtocolRequestType<ReferenceParams, Location[] | null, Location[], void, ReferenceRegistrationOptions>(method)"
)

/**
 * Request to resolve a [DocumentHighlight](#DocumentHighlight) for a given
 * text document position. The request's parameter is of type [TextDocumentPosition]
 * (#TextDocumentPosition) the request response is of type [DocumentHighlight[]]
 * (#DocumentHighlight) or a Thenable that resolves to such.
 */
type DocumentHighlightRequest string

const (
	DocumentHighlightRequestMethod DocumentHighlightRequest = "textDocument/documentHighlight"

	DocumentHighlightRequestType DocumentHighlightRequest = "new ProtocolRequestType<DocumentHighlightParams, DocumentHighlight[] | null, DocumentHighlight[], void, DocumentHighlightRegistrationOptions>(method)"
)

/**
 * A request to list all symbols found in a given text document. The request's
 * parameter is of type [TextDocumentIdentifier](#TextDocumentIdentifier) the
 * response is of type [SymbolInformation[]](#SymbolInformation) or a Thenable
 * that resolves to such.
 */
type DocumentSymbolRequest string

const (
	DocumentSymbolRequestMethod DocumentSymbolRequest = "textDocument/documentSymbol"

	DocumentSymbolRequestType DocumentSymbolRequest = "new ProtocolRequestType<DocumentSymbolParams, SymbolInformation[] | DocumentSymbol[] | null, SymbolInformation[] | DocumentSymbol[], void, DocumentSymbolRegistrationOptions>(method)"
)

/**
 * A request to provide commands for the given text document and range.
 */
type CodeActionRequest string

const (
	CodeActionRequestMethod CodeActionRequest = "textDocument/codeAction"

	CodeActionRequestType CodeActionRequest = "new ProtocolRequestType<CodeActionParams, (Command | CodeAction)[] | null, (Command | CodeAction)[], void, CodeActionRegistrationOptions>(method)"
)

/**
 * Request to resolve additional information for a given code action.The request's
 * parameter is of type [CodeAction](#CodeAction) the response
 * is of type [CodeAction](#CodeAction) or a Thenable that resolves to such.
 */
type CodeActionResolveRequest string

const (
	CodeActionResolveRequestMethod CodeActionResolveRequest = "codeAction/resolve"

	CodeActionResolveRequestType CodeActionResolveRequest = "new ProtocolRequestType<CodeAction, CodeAction, never, void, void>(method)"
)

/**
 * A request to list project-wide symbols matching the query string given
 * by the [WorkspaceSymbolParams](#WorkspaceSymbolParams). The response is
 * of type [SymbolInformation[]](#SymbolInformation) or a Thenable that
 * resolves to such.
 */
type WorkspaceSymbolRequest string

const (
	WorkspaceSymbolRequestMethod WorkspaceSymbolRequest = "workspace/symbol"

	WorkspaceSymbolRequestType WorkspaceSymbolRequest = "new ProtocolRequestType<WorkspaceSymbolParams, SymbolInformation[] | null, SymbolInformation[], void, WorkspaceSymbolRegistrationOptions>(method)"
)

/**
 * A request to provide code lens for the given text document.
 */
type CodeLensRequest string

const (
	CodeLensRequestMethod CodeLensRequest = "textDocument/codeLens"

	CodeLensRequestType CodeLensRequest = "new ProtocolRequestType<CodeLensParams, CodeLens[] | null, CodeLens[], void, CodeLensRegistrationOptions>(method)"
)

/**
 * A request to resolve a command for a given code lens.
 */
type CodeLensResolveRequest string

const (
	CodeLensResolveRequestMethod CodeLensResolveRequest = "codeLens/resolve"

	CodeLensResolveRequestType CodeLensResolveRequest = "new ProtocolRequestType<CodeLens, CodeLens, never, void, void>(method)"
)

/**
 * A request to refresh all code actions
 *
 * @since 3.16.0
 */
type CodeLensRefreshRequest string

const (
	CodeLensRefreshRequestMethod CodeLensRefreshRequest = "`workspace/codeLens/refresh`"

	CodeLensRefreshRequestType CodeLensRefreshRequest = "new ProtocolRequestType0<void, void, void, void>(method)"
)

/**
 * A request to provide document links
 */
type DocumentLinkRequest string

const (
	DocumentLinkRequestMethod DocumentLinkRequest = "textDocument/documentLink"

	DocumentLinkRequestType DocumentLinkRequest = "new ProtocolRequestType<DocumentLinkParams, DocumentLink[] | null, DocumentLink[], void, DocumentLinkRegistrationOptions>(method)"
)

/**
 * Request to resolve additional information for a given document link. The request's
 * parameter is of type [DocumentLink](#DocumentLink) the response
 * is of type [DocumentLink](#DocumentLink) or a Thenable that resolves to such.
 */
type DocumentLinkResolveRequest string

const (
	DocumentLinkResolveRequestMethod DocumentLinkResolveRequest = "documentLink/resolve"

	DocumentLinkResolveRequestType DocumentLinkResolveRequest = "new ProtocolRequestType<DocumentLink, DocumentLink, never, void, void>(method)"
)

/**
 * A request to to format a whole document.
 */
type DocumentFormattingRequest string

const (
	DocumentFormattingRequestMethod DocumentFormattingRequest = "textDocument/formatting"

	DocumentFormattingRequestType DocumentFormattingRequest = "new ProtocolRequestType<DocumentFormattingParams, TextEdit[] | null, never, void, DocumentFormattingRegistrationOptions>(method)"
)

/**
 * A request to to format a range in a document.
 */
type DocumentRangeFormattingRequest string

const (
	DocumentRangeFormattingRequestMethod DocumentRangeFormattingRequest = "textDocument/rangeFormatting"

	DocumentRangeFormattingRequestType DocumentRangeFormattingRequest = "new ProtocolRequestType<DocumentRangeFormattingParams, TextEdit[] | null, never, void, DocumentRangeFormattingRegistrationOptions>(method)"
)

/**
 * A request to format a document on type.
 */
type DocumentOnTypeFormattingRequest string

const (
	DocumentOnTypeFormattingRequestMethod DocumentOnTypeFormattingRequest = "textDocument/onTypeFormatting"

	DocumentOnTypeFormattingRequestType DocumentOnTypeFormattingRequest = "new ProtocolRequestType<DocumentOnTypeFormattingParams, TextEdit[] | null, never, void, DocumentOnTypeFormattingRegistrationOptions>(method)"
)

// ---- Rename ----------------------------------------------
type PrepareSupportDefaultBehavior int

var prepareSupportDefaultBehaviorStringMap = map[PrepareSupportDefaultBehavior]string{
	PrepareSupportDefaultBehaviorIdentifier: "Identifier",
}

func (i PrepareSupportDefaultBehavior) String() string {
	if s, ok := prepareSupportDefaultBehaviorStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * The client's default behavior is to select the identifier
	 * according the to language's syntax rule.
	 */
	PrepareSupportDefaultBehaviorIdentifier PrepareSupportDefaultBehavior = 1
)

/**
 * A request to rename a symbol.
 */
type RenameRequest string

const (
	RenameRequestMethod RenameRequest = "textDocument/rename"

	RenameRequestType RenameRequest = "new ProtocolRequestType<RenameParams, WorkspaceEdit | null, never, void, RenameRegistrationOptions>(method)"
)

/**
 * A request to test and perform the setup necessary for a rename.
 *
 * @since 3.16 - support for default behavior
 */
type PrepareRenameRequest string

const (
	PrepareRenameRequestMethod PrepareRenameRequest = "textDocument/prepareRename"

	PrepareRenameRequestType PrepareRenameRequest = "new ProtocolRequestType<PrepareRenameParams, Range | { range: Range, placeholder: string } | { defaultBehavior: boolean } | null, never, void, void>(method)"
)

/**
 * A request send from the client to the server to execute a command. The request might return
 * a workspace edit which the client will apply to the workspace.
 */
type ExecuteCommandRequest string

const (
	ExecuteCommandRequestType ExecuteCommandRequest = "new ProtocolRequestType<ExecuteCommandParams, any | null, never, void, ExecuteCommandRegistrationOptions>('workspace/executeCommand')"
)

/**
 * A request sent from the server to the client to modified certain resources.
 */
type ApplyWorkspaceEditRequest string

const (
	ApplyWorkspaceEditRequestType ApplyWorkspaceEditRequest = "new ProtocolRequestType<ApplyWorkspaceEditParams, ApplyWorkspaceEditResult, never, void, void>('workspace/applyEdit')"
)
