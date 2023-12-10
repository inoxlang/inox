package defines

type WorkspaceDocumentDiagnosticReport interface{} // WorkspaceFullDocumentDiagnosticReport | WorkspaceUnchangedDocumentDiagnosticReport;

/**
 * @since 3.17.0 - proposed state
 */
type DiagnosticClientCapabilities struct {

	// Whether implementation supports dynamic registration. If this is set to `true`
	// the client supports the new `(TextDocumentRegistrationOptions & StaticRegistrationOptions)`
	// return value for the corresponding server capability as well.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// Whether the clients supports related documents for document diagnostic pulls.
	RelatedDocumentSupport *bool `json:"relatedDocumentSupport,omitempty"`
}

/**
 * Diagnostic options.
 *
 * @since 3.17.0 - proposed state
 */
type DiagnosticOptions struct {
	WorkDoneProgressOptions

	// An optional identifier under which the diagnostics are
	// managed by the client.
	Identifier *string `json:"identifier,omitempty"`

	// Whether the language has inter file dependencies meaning that
	// editing code in one file can result in a different diagnostic
	// set in another file. Inter file dependencies are common for
	// most programming languages and typically uncommon for linters.
	InterFileDependencies bool `json:"interFileDependencies,omitempty"`

	// The server provides support for workspace diagnostics as well.
	WorkspaceDiagnostics bool `json:"workspaceDiagnostics,omitempty"`
}

/**
 * Diagnostic registration options.
 *
 * @since 3.17.0 - proposed state
 */
type DiagnosticRegistrationOptions struct {
	TextDocumentRegistrationOptions
	DiagnosticOptions
	StaticRegistrationOptions
}

/**
 * Cancellation data returned from a diagnostic request.
 *
 * @since 3.17.0 - proposed state
 */
type DiagnosticServerCancellationData struct {
	RetriggerRequest bool `json:"retriggerRequest,omitempty"`
}

/**
 * Parameters of the document diagnostic request.
 *
 * @since 3.17.0 - proposed state
 */
type DocumentDiagnosticParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The text document.
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`

	// The additional identifier  provided during registration.
	Identifier *string `json:"identifier,omitempty"`

	// The result id of a previous response if provided.
	PreviousResultId *string `json:"previousResultId,omitempty"`
}

/**
 * Parameters of the workspace diagnostic request.
 *
 * @since 3.17.0 - proposed state
 */
type WorkspaceDiagnosticParams struct {
	WorkDoneProgressParams
	PartialResultParams

	// The additional identifier provided during registration.
	Identifier *string `json:"identifier,omitempty"`

	// The currently known diagnostic reports with their
	// previous result ids.
	PreviousResultIds []PreviousResultId `json:"previousResultIds,omitempty"`
}

type PreviousResultId struct {
	Uri   URI    `json:"uri,omitempty"`
	Value string `json:"value,omitempty"`
}

type FullDocumentDiagnosticReport struct {
	Kind     interface{}   `json:"kind,omitempty"` // DocumentDiagnosticReportKind.full
	ResultId *string       `json:"resultId,omitempty"`
	Items    []interface{} `json:"items,omitempty"`
}

/**
 * A full document diagnostic report for a workspace diagnostic result.
 *
 * @since 3.17.0 - proposed state
 */
type WorkspaceFullDocumentDiagnosticReport struct {
	FullDocumentDiagnosticReport

	// The URI for which diagnostic information is reported.
	Uri DocumentUri `json:"uri,omitempty"`

	// The version number for which the diagnostics are reported.
	// If the document is not marked as open `null` can be provided.
	Version interface{} `json:"version,omitempty"` // int, null,
}

type UnchangedDocumentDiagnosticReport struct {
	Kind     interface{} `json:"kind,omitempty"` //DocumentDiagnosticReportKind.unChanged
	ResultId string      `json:"resultId,omitempty"`
}

/**
 * An unchanged document diagnostic report for a workspace diagnostic result.
 *
 * @since 3.17.0 - proposed state
 */
type WorkspaceUnchangedDocumentDiagnosticReport struct {
	UnchangedDocumentDiagnosticReport

	// The URI for which diagnostic information is reported.
	Uri DocumentUri `json:"uri,omitempty"`

	// The version number for which the diagnostics are reported.
	// If the document is not marked as open `null` can be provided.
	Version interface{} `json:"version,omitempty"` // int, null,
}

/**
 * A workspace diagnostic report.
 *
 * @since 3.17.0 - proposed state
 */
type WorkspaceDiagnosticReport struct {
	Items []WorkspaceDocumentDiagnosticReport `json:"items,omitempty"`
}

/**
 * A partial result for a workspace diagnostic report.
 *
 * @since 3.17.0 - proposed state
 */
type WorkspaceDiagnosticReportPartialResult struct {
	Items []WorkspaceDocumentDiagnosticReport `json:"items,omitempty"`
}

/**
 * The document diagnostic report kinds.
 *
 * @since 3.17.0 - proposed state
 */
type DocumentDiagnosticReportKind string

const (
	/**
	 * A diagnostic report with a full
	 * set of problems.
	 */
	DocumentDiagnosticReportKindFull DocumentDiagnosticReportKind = "full"
	/**
	 * A report indicating that the last
	 * returned report is still accurate.
	 */
	DocumentDiagnosticReportKindUnChanged DocumentDiagnosticReportKind = "unChanged"
)
