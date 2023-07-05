package defines

/**
 * Client capabilities for the show document request.
 *
 * @since 3.16.0
 */
type ShowDocumentClientCapabilities struct {

	// The client has support for the show document
	// request.
	Support bool `json:"support,omitempty"`
}

/**
 * Params to show a document.
 *
 * @since 3.16.0
 */
type ShowDocumentParams struct {

	// The document uri to show.
	Uri URI `json:"uri,omitempty"`

	// Indicates to show the resource in an external program.
	// To show for example `https:code.visualstudio.com`
	// in the default WEB browser set `external` to `true`.
	External *bool `json:"external,omitempty"`

	// An optional property to indicate whether the editor
	// showing the document should take focus or not.
	// Clients might ignore this property if an external
	// program in started.
	TakeFocus *bool `json:"takeFocus,omitempty"`

	// An optional selection range if the document is a text
	// document. Clients might ignore the property if an
	// external program is started or the file is not a text
	// file.
	Selection *Range `json:"selection,omitempty"`
}

/**
 * The result of an show document request.
 *
 * @since 3.16.0
 */
type ShowDocumentResult struct {

	// A boolean indicating if the show was successful.
	Success bool `json:"success,omitempty"`
}
