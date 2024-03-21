package inoxconsts

const (
	GO_MOD_ID                       = "github.com/inoxlang/inox"
	RELATIVE_MAIN_INOX_MOD_PKG_PATH = "app"
	TRANSPILED_APP_BINARY_NAME      = "mod"

	MAIN_INOX_MOD_PKG_ID            = GO_MOD_ID + "/app"
	TRANSPILED_MOD_EXECUTION_FN     = "Execute"
	PRIMARY_TRANSPILED_MOD_FILENAME = "mod.go" //file present in each Go package corresponding to an Inox module
)
