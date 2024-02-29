package inoxconsts

const (
	INOXLANG_FILE_EXTENSION   = ".ix"
	INOXLANG_SPEC_FILE_SUFFIX = ".spec.ix"
	DEV_DIR_NAME              = ".dev"

	FS_DIR_IN_IMG_ZIP          = "fs"
	FS_DIR_SLASH_IN_IMG_ZIP    = FS_DIR_IN_IMG_ZIP + "/"
	IMAGE_INFO_FILE_IN_IMG_ZIP = "inox-image.json"

	LDB_SCHEME_NAME string = "ldb"
	ODB_SCHEME_NAME string = "odb"

	IMPLICIT_PROP_NAME = ""

	//Project server.
	DEFAULT_PROJECT_SERVER_PORT                             = "8305"
	DEFAULT_PROJECT_SERVER_PORT_INT                         = 8305
	DEFAULT_DENO_CONTROL_SERVER_PORT_FOR_PROJECT_SERVER     = "8306"
	DEFAULT_DENO_CONTROL_SERVER_PORT_INT_FOR_PROJECT_SERVER = 8306

	//Ports reserved for development.

	DEV_PORT_0 string = "8080"
	DEV_PORT_1 string = "8081"

	DEV_SESSION_KEY_HEADER = "X-Dev-Session-Key"

	//Hyperscript

	HYPERSCRIPT_ATTRIBUTE_NAME = "_"
	HYPERSCRIPT_SCRIPT_MARKER  = "h" //<script h> elements are transpiled to <script type="text/hyperscript">
)

func IsDevPort(s string) bool {
	return s == DEV_PORT_0 || s == DEV_PORT_1
}
