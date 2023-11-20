package compl

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/help"
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

	MANIFEST_SECTION_DOC = map[string]string{
		core.MANIFEST_PARAMS_SECTION_NAME:    utils.MustGet(help.HelpFor("manifest/parameters-section", helpMessageConfig)),
		core.MANIFEST_ENV_SECTION_NAME:       utils.MustGet(help.HelpFor("manifest/env-section", helpMessageConfig)),
		core.MANIFEST_PERMS_SECTION_NAME:     utils.MustGet(help.HelpFor("manifest/permissions-section", helpMessageConfig)),
		core.MANIFEST_DATABASES_SECTION_NAME: utils.MustGet(help.HelpFor("manifest/databases-section", helpMessageConfig)),
	}

	MODULE_IMPORT_SECTION_DEFAULT_VALUE_COMPLETIONS = map[string]string{
		core.IMPORT_CONFIG__ALLOW_PROPNAME:     "{}",
		core.IMPORT_CONFIG__ARGUMENTS_PROPNAME: "{}",
	}

	MODULE_IMPORT_SECTION_DOC = map[string]string{
		core.IMPORT_CONFIG__ALLOW_PROPNAME:     utils.MustGet(help.HelpFor("module-import-config/allow-section", helpMessageConfig)),
		core.IMPORT_CONFIG__ARGUMENTS_PROPNAME: utils.MustGet(help.HelpFor("module-import-config/arguments-section", helpMessageConfig)),
	}

	MODULE_IMPORT_SECTION_LABEL_DETAILS = map[string]string{
		core.IMPORT_CONFIG__ALLOW_PROPNAME:      "granted permissions",
		core.IMPORT_CONFIG__ARGUMENTS_PROPNAME:  "module arguments",
		core.IMPORT_CONFIG__VALIDATION_PROPNAME: "validation string (base64 encoded sha256 hash)",
	}

	LTHREAD_META_SECTION_LABEL_DETAILS = map[string]string{
		symbolic.LTHREAD_META_ALLOW_SECTION:   "granted permissions",
		symbolic.LTHREAD_META_GLOBALS_SECTION: "globals of embedded module",
		symbolic.LTHREAD_META_GROUP_SECTION:   "group the lthread will be added to",
	}

	LTHREAD_META_SECTION_DOC = map[string]string{
		symbolic.LTHREAD_META_ALLOW_SECTION:   utils.MustGet(help.HelpFor("lthreads/allow-section", helpMessageConfig)),
		symbolic.LTHREAD_META_GLOBALS_SECTION: utils.MustGet(help.HelpFor("lthreads/globals-section", helpMessageConfig)),
	}

	LTHREAD_META_SECTION_DEFAULT_VALUE_COMPLETIONS = map[string]string{
		symbolic.LTHREAD_META_ALLOW_SECTION:   "{}",
		symbolic.LTHREAD_META_GLOBALS_SECTION: "{}",
	}

	helpMessageConfig = help.HelpMessageConfig{
		Format: help.MarkdownFormat,
	}
)
