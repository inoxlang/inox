package inoxconsts

const (
	IMPLICIT_PROP_NAME = ""
	VISIBILITY_KEY     = "_visibility_"
	CONSTRAINTS_KEY    = "_constraints_"

	//Units

	LINE_COUNT_UNIT               = "ln"
	RUNE_COUNT_UNIT               = "rn"
	BYTE_COUNT_UNIT               = "B"
	SIMPLE_RATE_PER_SECOND_SUFFIX = "x/s"

	//Module kind names

	UNSPECIFIED_MODULE_KIND_NAME = "unspecified"
	SPEC_MODULE_KIND_NAME        = "spec"
	LTHREAD_MODULE_KIND_NAME     = "userlthread"
	TESTSUITE_MODULE_KIND_NAME   = "testsuite"
	TESTCASE_MODULE_KIND_NAME    = "testcase"
	APP_MODULE_KIND_NAME         = "application"

	//Module import

	IMPORT_CONFIG__ALLOW_PROPNAME      = "allow"
	IMPORT_CONFIG__ARGUMENTS_PROPNAME  = "arguments"
	IMPORT_CONFIG__VALIDATION_PROPNAME = "validation"
)

var (
	IMPORT_CONFIG_SECTION_NAMES = []string{
		IMPORT_CONFIG__ALLOW_PROPNAME, IMPORT_CONFIG__ARGUMENTS_PROPNAME, IMPORT_CONFIG__VALIDATION_PROPNAME,
	}
)
