package defines

/**
 * @since 3.16.0
 */
type SemanticTokensPartialResult struct {
	Data []uint `json:"data,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensDeltaPartialResult struct {
	Edits []SemanticTokensEdit `json:"edits,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensClientCapabilities struct {

	// Whether implementation supports dynamic registration. If this is set to `true`
	// the client supports the new `(TextDocumentRegistrationOptions & StaticRegistrationOptions)`
	// return value for the corresponding server capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// Which requests the client supports and might send to the server
	// depending on the server's capability. Please note that clients might not
	// show semantic tokens or degrade some of the user experience if a range
	// or full request is advertised by the client but not provided by the
	// server. If for example the client capability `requests.full` and
	// `request.range` are both set to true but the server only provides a
	// range provider the client might not render a minimap correctly or might
	// even decide to not show any semantic tokens at all.
	Requests interface{} `json:"requests,omitempty"` // range, full,

	// The token types that the client supports.
	TokenTypes []string `json:"tokenTypes,omitempty"`

	// The token modifiers that the client supports.
	TokenModifiers []string `json:"tokenModifiers,omitempty"`

	// The token formats the clients supports.
	Formats []TokenFormat `json:"formats,omitempty"`

	// Whether the client supports tokens that can overlap each other.
	OverlappingTokenSupport *bool `json:"overlappingTokenSupport,omitempty"`

	// Whether the client supports tokens that can span multiple lines.
	MultilineTokenSupport *bool `json:"multilineTokenSupport,omitempty"`

	// Whether the client allows the server to actively cancel a
	// semantic token request, e.g. supports returning
	// LSPErrorCodes.ServerCancelled. If a server does the client
	// needs to retrigger the request.
	//
	// @since 3.17.0
	ServerCancelSupport *bool `json:"serverCancelSupport,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensOptions struct {
	WorkDoneProgressOptions

	// The legend used by the server
	Legend SemanticTokensLegend `json:"legend,omitempty"`

	// Server supports providing semantic tokens for a specific range
	// of a document.
	Range *bool `json:"range,omitempty"`

	// Server supports providing semantic tokens for a full document.
	Full *bool `json:"full,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensRegistrationOptions struct {
	TextDocumentRegistrationOptions
	SemanticTokensOptions
	StaticRegistrationOptions
}

/**
 * @since 3.16.0
 */
type SemanticTokensParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The text document.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensDeltaParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The text document.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The result id of a previous response. The result Id can either point to a full response
	// or a delta response depending on what was received last.
	PreviousResultId string `json:"previousResultId,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensRangeParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The text document.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The range the semantic tokens are requested for.
	Range Range `json:"range,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensWorkspaceClientCapabilities struct {

	// Whether the client implementation supports a refresh request sent from
	// the server to the client.
	//
	// Note that this event is global and will force the client to refresh all
	// semantic tokens currently shown. It should be used with absolute care
	// and is useful for situation where a server for example detects a project
	// wide change that requires such a calculation.
	RefreshSupport *bool `json:"refreshSupport,omitempty"`
}

//------- 'textDocument/semanticTokens' -----
type TokenFormat string

const (
	TokenFormatRelative TokenFormat = "relative"
)

type SemanticTokensRegistrationType string

const (
	SemanticTokensRegistrationTypeMethod SemanticTokensRegistrationType = "textDocument/semanticTokens"

	SemanticTokensRegistrationTypeType SemanticTokensRegistrationType = "new RegistrationType<SemanticTokensRegistrationOptions>(method)"
)
