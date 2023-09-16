package compl

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/help_ns"
	"github.com/inoxlang/inox/internal/utils"
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

	MANIFEST_SECTION_DOC = map[string]string{}

	LTHREAD_META_SECTION_LABEL_DETAILS = map[string]string{
		symbolic.LTHREAD_META_ALLOW_SECTION:   "granted permissions",
		symbolic.LTHREAD_META_GLOBALS_SECTION: "globals of embedded module",
		symbolic.LTHREAD_META_GROUP_SECTION:   "group the lthread will be added to",
	}

	LTHREAD_META_SECTION_DOC = map[string]string{
		symbolic.LTHREAD_META_ALLOW_SECTION:   utils.MustGet(help_ns.HelpFor("lthreads/allow-section", helpMessageConfig)),
		symbolic.LTHREAD_META_GLOBALS_SECTION: utils.MustGet(help_ns.HelpFor("lthreads/globals-section", helpMessageConfig)),
	}

	LTHREAD_META_SECTION_DEFAULT_VALUE_COMPLETIONS = map[string]string{
		symbolic.LTHREAD_META_ALLOW_SECTION:   "{}",
		symbolic.LTHREAD_META_GLOBALS_SECTION: "{}",
	}

	helpMessageConfig = help_ns.HelpMessageConfig{
		Format: help_ns.MarkdownFormat,
	}
)
