package codecompletion

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAJOR_PERM_KIND_TEXT = "major permission kind"
	MINOR_PERM_KIND_TEXT = "minor permission kind"
)

var (
	MANIFEST_SECTION_DEFAULT_VALUE_COMPLETIONS = map[string]string{
		inoxconsts.MANIFEST_ENV_SECTION_NAME:              "%{}",
		inoxconsts.MANIFEST_DATABASES_SECTION_NAME:        "{}",
		inoxconsts.MANIFEST_PARAMS_SECTION_NAME:           "{}",
		inoxconsts.MANIFEST_PERMS_SECTION_NAME:            "{}",
		inoxconsts.MANIFEST_LIMITS_SECTION_NAME:           "{}",
		inoxconsts.MANIFEST_HOST_DEFINITIONS_SECTION_NAME: ":{}",
		inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME:    "{}",
	}

	MANIFEST_SECTION_DOC = map[string]string{
		inoxconsts.MANIFEST_PARAMS_SECTION_NAME:    utils.MustGet(help.HelpFor("manifest/parameters-section", helpMessageConfig)),
		inoxconsts.MANIFEST_ENV_SECTION_NAME:       utils.MustGet(help.HelpFor("manifest/env-section", helpMessageConfig)),
		inoxconsts.MANIFEST_PERMS_SECTION_NAME:     utils.MustGet(help.HelpFor("manifest/permissions-section", helpMessageConfig)),
		inoxconsts.MANIFEST_DATABASES_SECTION_NAME: utils.MustGet(help.HelpFor("manifest/databases-section", helpMessageConfig)),
	}

	MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS = map[string]string{
		inoxconsts.MANIFEST_DATABASE__RESOURCE_PROP_NAME:               "ldb://main  # (example) local database named 'main'",
		inoxconsts.MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME:        "nil",
		inoxconsts.MANIFEST_DATABASE__EXPECTED_SCHEMA_UPDATE_PROP_NAME: "false  # should be set to true if the module performs a schema update (update_schema call)",
		inoxconsts.MANIFEST_DATABASE__ASSERT_SCHEMA_UPDATE_PROP_NAME:   "# object pattern to check the actual schema against",
	}

	MANIFEST_DB_DESC_DOC = map[string]string{
		inoxconsts.MANIFEST_DATABASE__RESOURCE_PROP_NAME:               utils.MustGet(help.HelpFor("manifest/databases-section/resource", helpMessageConfig)),
		inoxconsts.MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME:        utils.MustGet(help.HelpFor("manifest/databases-section/resolution-data", helpMessageConfig)),
		inoxconsts.MANIFEST_DATABASE__EXPECTED_SCHEMA_UPDATE_PROP_NAME: utils.MustGet(help.HelpFor("manifest/databases-section/expected-schema-update", helpMessageConfig)),
		inoxconsts.MANIFEST_DATABASE__ASSERT_SCHEMA_UPDATE_PROP_NAME:   utils.MustGet(help.HelpFor("manifest/databases-section/assert-schema", helpMessageConfig)),
	}

	MODULE_IMPORT_SECTION_DEFAULT_VALUE_COMPLETIONS = map[string]string{
		inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME:     "{}",
		inoxconsts.IMPORT_CONFIG__ARGUMENTS_PROPNAME: "{}",
	}

	MODULE_IMPORT_SECTION_DOC = map[string]string{
		inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME:     utils.MustGet(help.HelpFor("module-import-config/allow-section", helpMessageConfig)),
		inoxconsts.IMPORT_CONFIG__ARGUMENTS_PROPNAME: utils.MustGet(help.HelpFor("module-import-config/arguments-section", helpMessageConfig)),
	}

	MODULE_IMPORT_SECTION_LABEL_DETAILS = map[string]string{
		inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME:      "granted permissions",
		inoxconsts.IMPORT_CONFIG__ARGUMENTS_PROPNAME:  "module arguments",
		inoxconsts.IMPORT_CONFIG__VALIDATION_PROPNAME: "validation string (base64 encoded sha256 hash)",
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
