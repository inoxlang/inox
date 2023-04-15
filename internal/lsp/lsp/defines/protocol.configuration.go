package defines

//---- Get Configuration request ----
type ConfigurationClientCapabilities struct {

	// The workspace client capabilities
	Workspace *struct {

		// The client supports `workspaceconfiguration` requests.
		//
		// @since 3.6.0
		Configuration *bool `json:"configuration,omitempty"`
	} `json:"workspace,omitempty"`
}

type ConfigurationItem struct {

	// The scope to get the configuration section for.
	ScopeUri *string `json:"scopeUri,omitempty"`

	// The configuration section asked for.
	Section *string `json:"section,omitempty"`
}

/**
 * The parameters of a configuration request.
 */
type ConfigurationParams struct {
	Items []ConfigurationItem `json:"items,omitempty"`
}
