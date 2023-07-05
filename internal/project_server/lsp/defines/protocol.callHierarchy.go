package defines

/**
 * @since 3.16.0
 */
type CallHierarchyClientCapabilities struct {

	// Whether implementation supports dynamic registration. If this is set to `true`
	// the client supports the new `(TextDocumentRegistrationOptions & StaticRegistrationOptions)`
	// return value for the corresponding server capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * Call hierarchy options used during static registration.
 *
 * @since 3.16.0
 */
type CallHierarchyOptions struct {
	WorkDoneProgressOptions
}

/**
 * Call hierarchy options used during static or dynamic registration.
 *
 * @since 3.16.0
 */
type CallHierarchyRegistrationOptions struct {
	TextDocumentRegistrationOptions
	CallHierarchyOptions
	StaticRegistrationOptions
}

/**
 * The parameter of a `textDocument/prepareCallHierarchy` request.
 *
 * @since 3.16.0
 */
type CallHierarchyPrepareParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
}

/**
 * The parameter of a `callHierarchy/incomingCalls` request.
 *
 * @since 3.16.0
 */
type CallHierarchyIncomingCallsParams struct {
	WorkDoneProgressParams
	PartialResultParams

	Item CallHierarchyItem `json:"item,omitempty"`
}

/**
 * The parameter of a `callHierarchy/outgoingCalls` request.
 *
 * @since 3.16.0
 */
type CallHierarchyOutgoingCallsParams struct {
	WorkDoneProgressParams
	PartialResultParams

	Item CallHierarchyItem `json:"item,omitempty"`
}
