package inoxconsts

const (
	// -------- sections --------

	//section names
	MANIFEST_KIND_SECTION_NAME             = "kind"
	MANIFEST_ENV_SECTION_NAME              = "env"
	MANIFEST_PARAMS_SECTION_NAME           = "parameters"
	MANIFEST_PERMS_SECTION_NAME            = "permissions"
	MANIFEST_LIMITS_SECTION_NAME           = "limits"
	MANIFEST_HOST_DEFINITIONS_SECTION_NAME = "host-definitions"
	MANIFEST_PREINIT_FILES_SECTION_NAME    = "preinit-files"

	//preinit-files section
	MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME = "pattern"
	MANIFEST_PREINIT_FILE__PATH_PROP_NAME    = "path"

	//parameters
	MANIFEST_PARAM__PATTERN_PROPNAME                  = "pattern"
	MANIFEST_PARAM__DESCRIPTION_PROPNAME              = "description"
	MANIFEST_POSITIONAL_PARAM__REST_PROPNAME          = "rest"
	MANIFEST_NON_POSITIONAL_PARAM__NAME_PROPNAME      = "name"
	MANIFEST_NON_POSITIONAL_PARAM__DEFAULT_PROPNAME   = "default"
	MANIFEST_NON_POSITIONAL_PARAM__CHAR_NAME_PROPNAME = "char-name"
)

var (
	MANIFEST_SECTION_NAMES = []string{
		MANIFEST_KIND_SECTION_NAME, MANIFEST_ENV_SECTION_NAME, MANIFEST_PARAMS_SECTION_NAME,
		MANIFEST_PERMS_SECTION_NAME, MANIFEST_LIMITS_SECTION_NAME,
		MANIFEST_HOST_DEFINITIONS_SECTION_NAME, MANIFEST_PREINIT_FILES_SECTION_NAME,
	}
)
