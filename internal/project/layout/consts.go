package layout

const (
	STATIC_JS_DIRNAME      = "js"
	INOXJS_FILENAME        = "inox.js"
	INOX_JS_FILENAME       = "inox.js"
	HYPERSCRIPTJS_FILENAME = "hyperscript.js"
	HTMX_JS_FILENAME       = "htmx.js"

	STATIC_STYLES_DIRNAME = "styles"
	TAILWIND_FILENAME     = "tailwind.css"
	MAIN_CSS_FILENAME     = "main.css"
	TAILWIND_IMPORT       = "/* Tailwind */\n\n@import \"" + TAILWIND_FILENAME + "\";"

	TAILWIND_CSS_STYLESHEET_EXPLANATION = "/* This file is generated automatically by scanning the codebase for Tailwind class names. */"
	HYPERSCRIPT_JS_EXPLANATION          = "/* This file is generated automatically by scanning the codebase for used Hyperscript features. */"
	HTMX_JS_EXPLANATION                 = "/* This file is generated automatically by scanning the codebase for used HTMX extensions. */"
	INOX_JS_EXPLANATION                 = "/* This file is generated automatically by scanning the codebase for used librairies among: Surreal, CSS Scope Inline, Preact Signals, .... */"
)
