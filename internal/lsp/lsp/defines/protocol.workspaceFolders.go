package defines

type WorkspaceFoldersInitializeParams struct {

	// The actual configured workspace folders.
	WorkspaceFolders interface{} `json:"workspaceFolders,omitempty"` // []WorkspaceFolder, null,
}

type WorkspaceFoldersClientCapabilities struct {

	// The workspace client capabilities
	Workspace *struct {

		// The client has support for workspace folders
		//
		// @since 3.6.0
		WorkspaceFolders *bool `json:"workspaceFolders,omitempty"`
	} `json:"workspace,omitempty"`
}

type WorkspaceFoldersServerCapabilities struct {

	// The workspace server capabilities
	Workspace *struct {
		WorkspaceFolders interface{} `json:"workspaceFolders,omitempty"` // supported, changeNotifications,
	} `json:"workspace,omitempty"`
}

type WorkspaceFolder struct {

	// The associated URI for this workspace folder.
	Uri string `json:"uri,omitempty"`

	// The name of the workspace folder. Used to refer to this
	// workspace folder in the user interface.
	Name string `json:"name,omitempty"`
}

/**
 * The parameters of a `workspace/didChangeWorkspaceFolders` notification.
 */
type DidChangeWorkspaceFoldersParams struct {

	// The actual workspace folder change event.
	Event WorkspaceFoldersChangeEvent `json:"event,omitempty"`
}

/**
 * The workspace folder change event.
 */
type WorkspaceFoldersChangeEvent struct {

	// The array of added workspace folders
	Added []WorkspaceFolder `json:"added,omitempty"`

	// The array of the removed workspace folders
	Removed []WorkspaceFolder `json:"removed,omitempty"`
}
