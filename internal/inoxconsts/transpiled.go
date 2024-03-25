package inoxconsts

const (
	GO_MOD_ID                       = "github.com/inoxlang/inox"
	RELATIVE_MAIN_INOX_MOD_PKG_PATH = "app"
	TRANSPILED_APP_BINARY_NAME      = "mod"

	MAIN_INOX_MOD_PKG_ID            = GO_MOD_ID + "/app"
	TRANSPILED_MOD_EXECUTION_FN     = "Execute"
	TRANSPILED_MOD_PRIMARY_FILENAME = "_mod_.go" //file present in each Go package corresponding to an Inox module
)
