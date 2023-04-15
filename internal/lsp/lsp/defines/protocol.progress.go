package defines

type WorkDoneProgressClientCapabilities struct {

	// Window specific client capabilities.
	Window *struct {

		// Whether client supports server initiated progress using the
		// `windowworkDoneProgresscreate` request.
		//
		// Since 3.15.0
		WorkDoneProgress *bool `json:"workDoneProgress,omitempty"`
	} `json:"window,omitempty"`
}

type WorkDoneProgressBegin struct {
	Kind interface{} `json:"kind,omitempty"` // 'begin'

	// Mandatory title of the progress operation. Used to briefly inform about
	// the kind of operation being performed.
	//
	// Examples: "Indexing" or "Linking dependencies".
	Title string `json:"title,omitempty"`

	// Controls if a cancel button should show to allow the user to cancel the
	// long running operation. Clients that don't support cancellation are allowed
	// to ignore the setting.
	Cancellable *bool `json:"cancellable,omitempty"`

	// Optional, more detailed associated progress message. Contains
	// complementary information to the `title`.
	//
	// Examples: "325 files", "projectsrcmodule2", "node_modulessome_dep".
	// If unset, the previous progress message (if any) is still valid.
	Message *string `json:"message,omitempty"`

	// Optional progress percentage to display (value 100 is considered 100%).
	// If not provided infinite progress is assumed and clients are allowed
	// to ignore the `percentage` value in subsequent in report notifications.
	//
	// The value should be steadily rising. Clients are free to ignore values
	// that are not following this rule. The value range is [0, 100].
	Percentage *uint `json:"percentage,omitempty"`
}

type WorkDoneProgressReport struct {
	Kind interface{} `json:"kind,omitempty"` // 'report'

	// Controls enablement state of a cancel button.
	//
	// Clients that don't support cancellation or don't support controlling the button's
	// enablement state are allowed to ignore the property.
	Cancellable *bool `json:"cancellable,omitempty"`

	// Optional, more detailed associated progress message. Contains
	// complementary information to the `title`.
	//
	// Examples: "325 files", "projectsrcmodule2", "node_modulessome_dep".
	// If unset, the previous progress message (if any) is still valid.
	Message *string `json:"message,omitempty"`

	// Optional progress percentage to display (value 100 is considered 100%).
	// If not provided infinite progress is assumed and clients are allowed
	// to ignore the `percentage` value in subsequent in report notifications.
	//
	// The value should be steadily rising. Clients are free to ignore values
	// that are not following this rule. The value range is [0, 100]
	Percentage *uint `json:"percentage,omitempty"`
}

type WorkDoneProgressEnd struct {
	Kind interface{} `json:"kind,omitempty"` // 'end'

	// Optional, a final message indicating to for example indicate the outcome
	// of the operation.
	Message *string `json:"message,omitempty"`
}

type WorkDoneProgressCreateParams struct {

	// The token to be used to report progress.
	Token ProgressToken `json:"token,omitempty"`
}

type WorkDoneProgressCancelParams struct {

	// The token to be used to report progress.
	Token ProgressToken `json:"token,omitempty"`
}
