package compl

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

const (
	MAJOR_PERM_KIND_TEXT = "major permission kind"
	MINOR_PERM_KIND_TEXT = "minor permission kind"
)

var (
	MANIFEST_SECTION_DEFAULT_VALUE_COMPLETIONS = map[string]string{
		core.MANIFEST_ENV_SECTION_NAME:             "%{}",
		core.MANIFEST_DATABASES_SECTION_NAME:       "{}",
		core.MANIFEST_PARAMS_SECTION_NAME:          "{}",
		core.MANIFEST_PERMS_SECTION_NAME:           "{}",
		core.MANIFEST_LIMITS_SECTION_NAME:          "{}",
		core.MANIFEST_HOST_RESOLUTION_SECTION_NAME: ":{}",
		core.MANIFEST_PREINIT_FILES_SECTION_NAME:   "{}",
	}

	LTHREAD_META_SECTION_HELP = map[string]string{
		symbolic.LTHREAD_META_ALLOW_SECTION:   "granted permissions",
		symbolic.LTHREAD_META_GLOBALS_SECTION: "globals of embedded module",
		symbolic.LTHREAD_META_GROUP_SECTION:   "group the lthread will be added to",
	}

	LTHREAD_META_SECTION_DEFAULT_VALUE_COMPLETIONS = map[string]string{
		symbolic.LTHREAD_META_ALLOW_SECTION:   "{}",
		symbolic.LTHREAD_META_GLOBALS_SECTION: "{}",
	}
)
