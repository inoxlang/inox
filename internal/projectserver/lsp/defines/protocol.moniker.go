package defines

/**
 * Moniker definition to match LSIF 0.5 moniker definition.
 *
 * @since 3.16.0
 */
type Moniker struct {

	// The scheme of the moniker. For example tsc or .Net
	Scheme string `json:"scheme,omitempty"`

	// The identifier of the moniker. The value is opaque in LSIF however
	// schema owners are allowed to define the structure if they want.
	Identifier string `json:"identifier,omitempty"`

	// The scope in which the moniker is unique
	Unique UniquenessLevel `json:"unique,omitempty"`

	// The moniker kind if known.
	Kind *MonikerKind `json:"kind,omitempty"`
}

/**
 * Client capabilities specific to the moniker request.
 *
 * @since 3.16.0
 */
type MonikerClientCapabilities struct {

	// Whether moniker supports dynamic registration. If this is set to `true`
	// the client supports the new `MonikerRegistrationOptions` return value
	// for the corresponding server capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

type MonikerOptions struct {
	WorkDoneProgressOptions
}

type MonikerRegistrationOptions struct {
	TextDocumentRegistrationOptions
	MonikerOptions
}

type MonikerParams struct {
	TextDocumentPositionParams
	WorkDoneProgressParams
	PartialResultParams
}

/**
 * Moniker uniqueness level to define scope of the moniker.
 *
 * @since 3.16.0
 */
type UniquenessLevel string

const (
	/**
	 * The moniker is only unique inside a document
	 */
	UniquenessLevelDocument UniquenessLevel = "document"
	/**
	 * The moniker is unique inside a project for which a dump got created
	 */
	UniquenessLevelProject UniquenessLevel = "project"
	/**
	 * The moniker is unique inside the group to which a project belongs
	 */
	UniquenessLevelGroup UniquenessLevel = "group"
	/**
	 * The moniker is unique inside the moniker scheme.
	 */
	UniquenessLevelScheme UniquenessLevel = "scheme"
	/**
	 * The moniker is globally unique
	 */
	UniquenessLevelGlobal UniquenessLevel = "global"
)

/**
 * The moniker kind.
 *
 * @since 3.16.0
 */
type MonikerKind string

const (
	/**
	 * The moniker represent a symbol that is imported into a project
	 */
	MonikerKindImport MonikerKind = "import"
	/**
	 * The moniker represents a symbol that is exported from a project
	 */
	MonikerKindExport MonikerKind = "export"
	/**
	 * The moniker represents a symbol that is local to a project (e.g. a local
	 * variable of a function, a class not visible outside the project, ...)
	 */
	MonikerKindLocal MonikerKind = "local"
)

/**
 * A request to get the moniker of a symbol at a given text document position.
 * The request parameter is of type [TextDocumentPositionParams](#TextDocumentPositionParams).
 * The response is of type [Moniker[]](#Moniker[]) or `null`.
 */
type MonikerRequest string

const (
	MonikerRequestMethod MonikerRequest = "textDocument/moniker"

	MonikerRequestType MonikerRequest = "new ProtocolRequestType<MonikerParams, Moniker[] | null, Moniker[], void, MonikerRegistrationOptions>(method)"
)
