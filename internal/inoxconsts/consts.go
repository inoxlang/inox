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

	//Ports reserved for development.

	DEV_PORT_0 string = "8080"
	DEV_PORT_1 string = "8081"
)

func IsDevPort(s string) bool {
	return s == DEV_PORT_0 || s == DEV_PORT_1
}
