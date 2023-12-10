package defines

/**
 * @since 3.17.0 - proposed state
 */
type TypeHierarchyClientCapabilities struct {

	// Whether implementation supports dynamic registration. If this is set to `true`
	// the client supports the new `(TextDocumentRegistrationOptions & StaticRegistrationOptions)`
	// return value for the corresponding server capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

/**
 * Type hierarchy options used during static registration.
 *
 * @since 3.17.0 - proposed state
 */
type TypeHierarchyOptions struct {
	WorkDoneProgressOptions
}

/**
 * Type hierarchy options used during static or dynamic registration.
 *
 * @since 3.17.0 - proposed state
 */
type TypeHierarchyRegistrationOptions struct {
	TextDocumentRegistrationOptions
	TypeHierarchyOptions
	StaticRegistrationOptions
}

/**
 * The parameter of a `textDocument/prepareTypeHierarchy` request.
 *
 * @since 3.17.0 - proposed state
 */
type TypeHierarchyPrepareParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
}

/**
 * The parameter of a `typeHierarchy/supertypes` request.
 *
 * @since 3.17.0 - proposed state
 */
type TypeHierarchySupertypesParams struct {
	WorkDoneProgressParams
	PartialResultParams

	Item TypeHierarchyItem `json:"item,omitempty"`
}

/**
 * The parameter of a `typeHierarchy/subtypes` request.
 *
 * @since 3.17.0 - proposed state
 */
type TypeHierarchySubtypesParams struct {
	WorkDoneProgressParams
	PartialResultParams

	Item TypeHierarchyItem `json:"item,omitempty"`
}
