package defines

/**
 * Client capabilities for the linked editing range request.
 *
 * @since 3.16.0
 */
type LinkedEditingRangeClientCapabilities struct {

	// Whether implementation supports dynamic registration. If this is set to `true`
	// the client supports the new `(TextDocumentRegistrationOptions & StaticRegistrationOptions)`
	// return value for the corresponding server capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

type LinkedEditingRangeParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
}

type LinkedEditingRangeOptions struct {
	WorkDoneProgressOptions
}

type LinkedEditingRangeRegistrationOptions struct {
	TextDocumentRegistrationOptions
	LinkedEditingRangeOptions
	StaticRegistrationOptions
}

/**
 * The result of a linked editing range request.
 *
 * @since 3.16.0
 */
type LinkedEditingRanges struct {

	// A list of ranges that can be edited together. The ranges must have
	// identical length and contain identical text content. The ranges cannot overlap.
	Ranges []Range `json:"ranges,omitempty"`

	// An optional word pattern (regular expression) that describes valid contents for
	// the given ranges. If no pattern is provided, the client configuration's word
	// pattern will be used.
	WordPattern *string `json:"wordPattern,omitempty"`
}
