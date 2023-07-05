package defines

// ---- capabilities
type SelectionRangeClientCapabilities struct {

	// Whether implementation supports dynamic registration for selection range providers. If this is set to `true`
	// the client supports the new `SelectionRangeRegistrationOptions` return value for the corresponding server
	// capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

type SelectionRangeOptions struct {
	WorkDoneProgressOptions
}

type SelectionRangeRegistrationOptions struct {
	SelectionRangeOptions
	TextDocumentRegistrationOptions
	StaticRegistrationOptions
}

/**
 * A parameter literal used in selection range requests.
 */
type SelectionRangeParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The text document.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The positions inside the text document.
	Positions []Position `json:"positions,omitempty"`
}
