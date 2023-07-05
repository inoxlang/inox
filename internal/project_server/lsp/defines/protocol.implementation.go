package defines

/**
 * @since 3.6.0
 */
type ImplementationClientCapabilities struct {

	// Whether implementation supports dynamic registration. If this is set to `true`
	// the client supports the new `ImplementationRegistrationOptions` return value
	// for the corresponding server capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// The client supports additional metadata in the form of definition links.
	//
	// @since 3.14.0
	LinkSupport *bool `json:"linkSupport,omitempty"`
}

type ImplementationOptions struct {
	WorkDoneProgressOptions
}

type ImplementationRegistrationOptions struct {
	TextDocumentRegistrationOptions
	ImplementationOptions
	StaticRegistrationOptions
}

type ImplementationParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
	PartialResultParams
}
