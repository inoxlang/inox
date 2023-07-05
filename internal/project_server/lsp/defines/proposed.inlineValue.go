package defines

/**
 * Client capabilities specific to inline values.
 *
 * @since 3.17.0 - proposed state
 */
type InlineValuesClientCapabilities struct {

	// Whether implementation supports dynamic registration for inline value providers.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * Client workspace capabilities specific to inline values.
 *
 * @since 3.17.0 - proposed state
 */
type InlineValuesWorkspaceClientCapabilities struct {

	// Whether the client implementation supports a refresh request sent from the
	// server to the client.
	//
	// Note that this event is global and will force the client to refresh all
	// inline values currently shown. It should be used with absolute care and is
	// useful for situation where a server for example detect a project wide
	// change that requires such a calculation.
	RefreshSupport *bool `json:"refreshSupport,omitempty"`
}

/**
 * Inline values options used during static registration.
 *
 * @since 3.17.0 - proposed state
 */
type InlineValuesOptions struct {
	WorkDoneProgressOptions
}

/**
 * Inline value options used during static or dynamic registration.
 *
 * @since 3.17.0 - proposed state
 */
type InlineValuesRegistrationOptions struct {
	InlineValuesOptions
	TextDocumentRegistrationOptions
	StaticRegistrationOptions
}

/**
 * A parameter literal used in inline values requests.
 *
 * @since 3.17.0 - proposed state
 */
type InlineValuesParams struct {
	WorkDoneProgressParams

	// The text document.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The visible document range for which inline values should be computed.
	ViewPort Range `json:"viewPort,omitempty"`

	// Additional information about the context in which inline values were
	// requested.
	Context InlineValuesContext `json:"context,omitempty"`
}
