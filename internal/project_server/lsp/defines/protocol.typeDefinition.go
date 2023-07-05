package defines

/**
 * Since 3.6.0
 */
type TypeDefinitionClientCapabilities struct {

	// Whether implementation supports dynamic registration. If this is set to `true`
	// the client supports the new `TypeDefinitionRegistrationOptions` return value
	// for the corresponding server capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// The client supports additional metadata in the form of definition links.
	//
	// Since 3.14.0
	LinkSupport *bool `json:"linkSupport,omitempty"`
}

type TypeDefinitionOptions struct {
	WorkDoneProgressOptions
}

type TypeDefinitionRegistrationOptions struct {
	TextDocumentRegistrationOptions
	TypeDefinitionOptions
	StaticRegistrationOptions
}

type TypeDefinitionParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
	PartialResultParams
}
