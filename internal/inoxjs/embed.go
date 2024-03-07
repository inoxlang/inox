package inoxjs

import _ "embed"

const (
	INOX_COMPONENT_LIB_NAME   = "inox-component"
	PREACT_SIGNALS_LIB_NAME   = "preact-signals"
	SURREAL_LIB_NAME          = "surreal"
	CSS_INLINE_SCOPE_LIB_NAME = "css-inline-scope"
)

var (
	//go:embed inox-component.js
	INOX_COMPONENT_JS string

	//go:embed preact-signals.js
	PREACT_SIGNALS_JS string

	//go:embed surreal.js
	SURREAL_JS string

	//go:embed css-inline-scope.js
	CSS_INLINE_SCOPE string

	LIBRARIES = map[string]string{
		INOX_COMPONENT_LIB_NAME:   INOX_COMPONENT_JS,
		PREACT_SIGNALS_LIB_NAME:   PREACT_SIGNALS_JS,
		SURREAL_LIB_NAME:          SURREAL_JS,
		CSS_INLINE_SCOPE_LIB_NAME: CSS_INLINE_SCOPE,
	}
)
