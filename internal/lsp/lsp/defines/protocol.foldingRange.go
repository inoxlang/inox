package defines

// ---- capabilities
type FoldingRangeClientCapabilities struct {

	// Whether implementation supports dynamic registration for folding range providers. If this is set to `true`
	// the client supports the new `FoldingRangeRegistrationOptions` return value for the corresponding server
	// capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// The maximum number of folding ranges that the client prefers to receive per document. The value serves as a
	// hint, servers are free to follow the limit.
	RangeLimit *uint `json:"rangeLimit,omitempty"`

	// If set, the client signals that it only supports folding complete lines. If set, client will
	// ignore specified `startCharacter` and `endCharacter` properties in a FoldingRange.
	LineFoldingOnly *bool `json:"lineFoldingOnly,omitempty"`
}

type FoldingRangeOptions struct {
	WorkDoneProgressOptions
}

type FoldingRangeRegistrationOptions struct {
	TextDocumentRegistrationOptions
	FoldingRangeOptions
	StaticRegistrationOptions
}

/**
 * Parameters for a [FoldingRangeRequest](#FoldingRangeRequest).
 */
type FoldingRangeParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The text document.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`
}
