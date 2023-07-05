package defines

/**
 * Options for notifications/requests for user operations on files.
 *
 * @since 3.16.0
 */
type FileOperationOptions struct {

	// The server is interested in didCreateFiles notifications.
	DidCreate *FileOperationRegistrationOptions `json:"didCreate,omitempty"`

	// The server is interested in willCreateFiles requests.
	WillCreate *FileOperationRegistrationOptions `json:"willCreate,omitempty"`

	// The server is interested in didRenameFiles notifications.
	DidRename *FileOperationRegistrationOptions `json:"didRename,omitempty"`

	// The server is interested in willRenameFiles requests.
	WillRename *FileOperationRegistrationOptions `json:"willRename,omitempty"`

	// The server is interested in didDeleteFiles file notifications.
	DidDelete *FileOperationRegistrationOptions `json:"didDelete,omitempty"`

	// The server is interested in willDeleteFiles file requests.
	WillDelete *FileOperationRegistrationOptions `json:"willDelete,omitempty"`
}

/**
 * The options to register for file operations.
 *
 * @since 3.16.0
 */
type FileOperationRegistrationOptions struct {

	// The actual filters.
	Filters []FileOperationFilter `json:"filters,omitempty"`
}

/**
 * Matching options for the file operation pattern.
 *
 * @since 3.16.0
 */
type FileOperationPatternOptions struct {

	// The pattern should be matched ignoring casing.
	IgnoreCase *bool `json:"ignoreCase,omitempty"`
}

/**
 * A pattern to describe in which file operation requests or notifications
 * the server is interested in.
 *
 * @since 3.16.0
 */
type FileOperationPattern struct {

	// The glob pattern to match. Glob patterns can have the following syntax:
	// - `` to match one or more characters in a path segment
	// - `?` to match on one character in a path segment
	// - `` to match any number of path segments, including none
	// - `{}` to group sub patterns into an OR expression. (e.g. `​.{ts,js}` matches all TypeScript and JavaScript files)
	// - `[]` to declare a range of characters to match in a path segment (e.g., `example.[0-9]` to match on `example.0`, `example.1`, …)
	// - `[!...]` to negate a range of characters to match in a path segment (e.g., `example.[!0-9]` to match on `example.a`, `example.b`, but not `example.0`)
	Glob string `json:"glob,omitempty"`

	// Whether to match files or folders with this pattern.
	//
	// Matches both if undefined.
	Matches *FileOperationPatternKind `json:"matches,omitempty"`

	// Additional options used during matching.
	Options *FileOperationPatternOptions `json:"options,omitempty"`
}

/**
 * A filter to describe in which file operation requests or notifications
 * the server is interested in.
 *
 * @since 3.16.0
 */
type FileOperationFilter struct {

	// A Uri like `file` or `untitled`.
	Scheme *string `json:"scheme,omitempty"`

	// The actual file operation pattern.
	Pattern FileOperationPattern `json:"pattern,omitempty"`
}

/**
 * Capabilities relating to events from file operations by the user in the client.
 *
 * These events do not come from the file system, they come from user operations
 * like renaming a file in the UI.
 *
 * @since 3.16.0
 */
type FileOperationClientCapabilities struct {

	// Whether the client supports dynamic registration for file requestsnotifications.
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`

	// The client has support for sending didCreateFiles notifications.
	DidCreate *bool `json:"didCreate,omitempty"`

	// The client has support for willCreateFiles requests.
	WillCreate *bool `json:"willCreate,omitempty"`

	// The client has support for sending didRenameFiles notifications.
	DidRename *bool `json:"didRename,omitempty"`

	// The client has support for willRenameFiles requests.
	WillRename *bool `json:"willRename,omitempty"`

	// The client has support for sending didDeleteFiles notifications.
	DidDelete *bool `json:"didDelete,omitempty"`

	// The client has support for willDeleteFiles requests.
	WillDelete *bool `json:"willDelete,omitempty"`
}

/**
 * The parameters sent in file create requests/notifications.
 *
 * @since 3.16.0
 */
type CreateFilesParams struct {

	// An array of all filesfolders created in this operation.
	Files []FileCreate `json:"files,omitempty"`
}

/**
 * Represents information on a file/folder create.
 *
 * @since 3.16.0
 */
type FileCreate struct {

	// A file: URI for the location of the filefolder being created.
	Uri string `json:"uri,omitempty"`
}

/**
 * The parameters sent in file rename requests/notifications.
 *
 * @since 3.16.0
 */
type RenameFilesParams struct {

	// An array of all filesfolders renamed in this operation. When a folder is renamed, only
	// the folder will be included, and not its children.
	Files []FileRename `json:"files,omitempty"`
}

/**
 * Represents information on a file/folder rename.
 *
 * @since 3.16.0
 */
type FileRename struct {

	// A file: URI for the original location of the filefolder being renamed.
	OldUri string `json:"oldUri,omitempty"`

	// A file: URI for the new location of the filefolder being renamed.
	NewUri string `json:"newUri,omitempty"`
}

/**
 * The parameters sent in file delete requests/notifications.
 *
 * @since 3.16.0
 */
type DeleteFilesParams struct {

	// An array of all filesfolders deleted in this operation.
	Files []FileDelete `json:"files,omitempty"`
}

/**
 * Represents information on a file/folder delete.
 *
 * @since 3.16.0
 */
type FileDelete struct {

	// A file: URI for the location of the filefolder being deleted.
	Uri string `json:"uri,omitempty"`
}

/**
 * A pattern kind describing if a glob pattern matches a file a folder or
 * both.
 *
 * @since 3.16.0
 */
type FileOperationPatternKind string

const (
	/**
	 * The pattern matches a file only.
	 */
	FileOperationPatternKindFile FileOperationPatternKind = "file"
	/**
	 * The pattern matches a folder only.
	 */
	FileOperationPatternKindFolder FileOperationPatternKind = "folder"
)
