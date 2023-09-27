package core

import (
	"fmt"
	"strings"

	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
)

const (
	MODULE_IMPORTS_NOT_ALLOWED_IN_INCLUDED_CHUNK = "modules imports are not allowed in included chunks"

	//manifest
	NO_SPREAD_IN_MANIFEST                           = "objects & lists in the manifest cannot contain spread elements"
	IMPLICIT_KEY_PROPS_NOT_ALLOWED_IN_MANIFEST      = "implicit key properties are not allowed in the manifest object"
	PERMS_SECTION_SHOULD_BE_AN_OBJECT               = "the '" + MANIFEST_PERMS_SECTION_NAME + "' section of the manifest should be an object"
	IMPLICIT_KEY_PROPS_NOT_ALLOWED_IN_PERMS_SECTION = "implicit key properties are not allowed in the 'permissions' section"

	LIMITS_SECTION_SHOULD_BE_AN_OBJECT                        = "the '" + MANIFEST_LIMITS_SECTION_NAME + "' section of the manifest should be an object"
	ENV_SECTION_SHOULD_BE_AN_OBJECT_PATTERN                   = "the '" + MANIFEST_ENV_SECTION_NAME + "' section of the manifest should be an object pattern literal"
	ENV_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS    = "the '" + MANIFEST_ENV_SECTION_NAME + "' section is not available in embedded module manifests"
	PARAMS_SECTION_SHOULD_BE_AN_OBJECT                        = "the '" + MANIFEST_PARAMS_SECTION_NAME + "' section of the manifest should be an object literal"
	PARAMS_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS = "the '" + MANIFEST_PARAMS_SECTION_NAME + "' section is not available in embedded module manifests"

	//permissions
	NO_PERM_DESCRIBED_BY_THIS_TYPE_OF_VALUE         = "there is no permission described by this type of value"
	NO_PERM_DESCRIBED_BY_STRINGS                    = "there is no permission described by strings"
	MAYBE_YOU_MEANT_TO_WRITE_A_PATH_LITERAL         = "maybe you meant to write a path literal such as /dir/ or /data.json (always unquoted)"
	MAYBE_YOU_MEANT_TO_WRITE_A_PATH_PATTERN_LITERAL = "maybe you meant to write a path pattern literal such as %/... or %/*.json (always unquoted)"
	MAYBE_YOU_MEANT_TO_WRITE_A_URL_LITERAL          = "maybe you meant to write a url literal such as https://example.com/ (always unquoted)"
	MAYBE_YOU_MEANT_TO_WRITE_A_URL_PATTERN_LITERAL  = "maybe you meant to write a url pattern literal such as %https://example.com/... (always unquoted)"

	//preinit-files section
	PREINIT_FILES_SECTION_SHOULD_BE_AN_OBJECT                        = "the '" + MANIFEST_PREINIT_FILES_SECTION_NAME + "' section of the manifest should be an object literal"
	PREINIT_FILES__FILE_CONFIG_SHOULD_BE_AN_OBJECT                   = "the description of each file in the '" + MANIFEST_PREINIT_FILES_SECTION_NAME + "' section of the manifest should be an object literal"
	PREINIT_FILES__FILE_CONFIG_PATH_SHOULD_BE_ABS_PATH               = "the ." + MANIFEST_PREINIT_FILE__PATH_PROP_NAME + " of each file in the '" + MANIFEST_PREINIT_FILES_SECTION_NAME + "' section (manifest) should be an absolute path"
	PREINIT_FILES_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS = "the '" + MANIFEST_PREINIT_FILES_SECTION_NAME + "' section is not available in embedded module manifests"

	//databases section
	DATABASES_SECTION_SHOULD_BE_AN_OBJECT_OR_ABS_PATH            = "the '" + MANIFEST_DATABASES_SECTION_NAME + "' section of the manifest should be an object literal or an absolute path literal"
	DATABASES__DB_CONFIG_SHOULD_BE_AN_OBJECT                     = "the description of each database in the '" + MANIFEST_DATABASES_SECTION_NAME + "' section of the manifest should be an object literal"
	DATABASES__DB_RESOURCE_SHOULD_BE_HOST_OR_URL                 = "the ." + MANIFEST_DATABASE__RESOURCE_PROP_NAME + " of each database in the '" + MANIFEST_DATABASES_SECTION_NAME + "' section (manifest) should be a Host or a URL"
	DATABASES__DB_EXPECTED_SCHEMA_UPDATE_SHOULD_BE_BOOL_LIT      = "the ." + MANIFEST_DATABASE__EXPECTED_SCHEMA_UPDATE_PROP_NAME + " of each database in the '" + MANIFEST_DATABASES_SECTION_NAME + "' section (manifest) should be a boolean literal (the property is optional)"
	DATABASES_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS = "the '" + MANIFEST_DATABASES_SECTION_NAME + "' section is not available in embedded module manifests"
	DATABASES__DB_RESOLUTION_DATA_ONLY_PATHS_SUPPORTED           = "paths are the only supported values for ." + MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME + " in a database description"

	HOST_RESOL_SECTION_SHOULD_BE_A_DICT = "the '" + MANIFEST_HOST_RESOLUTION_SECTION_NAME + "' section of the manifest should be a dictionary with host keys"
	HOST_SCHEME_NOT_SUPPORTED           = "the host's scheme is not supported"

	INVALID_RATE     = "invalid rate"
	INVALID_QUANTITY = "invalid quantity"

	//spawn expression
	INVALID_SPAWN_EXPR_EXPR_SHOULD_BE_ONE_OF                             = "invalid spawn expression: the expression should be a simple function call or an embedded module (that can be global)"
	INVALID_SPAWN_GLOBALS_SHOULD_BE                                      = "invalid spawn expression: the description of globals should be a key list literal or an object literal with no implicit-key properties nor spread elements"
	INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED = "invalid spawn expression: only object literals with no spread elements nor implicit-key properties are supported for meta's value"

	INVALID_ASSIGNMENT_ANONYMOUS_VAR_CANNOT_BE_ASSIGNED                         = "invalid assignment: anonymous variable '$' cannot be assigned"
	INVALID_ASSIGNMENT_EQUAL_ONLY_SUPPORTED_ASSIGNMENT_OPERATOR_FOR_SLICE_EXPRS = "invalid assignment: '=' is the only supported assignment operators for slice expressions"

	INVALID_FN_DECL_SHOULD_BE_TOP_LEVEL_STMT                       = "invalid function declaration: a function declaration should be a top level statement in a module (embedded or not)"
	INVALID_BREAK_OR_CONTINUE_STMT_SHOULD_BE_IN_A_FOR_OR_WALK_STMT = "invalid break/continue statement: should be in a for or walk statement"
	INVALID_PRUNE_STMT_SHOULD_BE_IN_WALK_STMT                      = "invalid prune statement: should be in a walk statement"
	SELF_ACCESSIBILITY_EXPLANATION                                 = "'self' is only accessible within functions that are object properties, metaproperty initialization blocks and in lifetime jobs"
	CANNOT_CHECK_OBJECT_PROP_WITHOUT_PARENT                        = "checking an ObjectProperty node requires the parent ObjectLiteral node"
	CANNOT_CHECK_OBJECT_METAPROP_WITHOUT_PARENT                    = "checking an ObjectMetaProperty node requires the parent ObjectLiteral node"
	OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS                          = "(object | object pattern | record) literals cannot have metaproperty keys, metaproperty keys have a starting & a trailing underscore '_'"
	CANNOT_CHECK_MANIFEST_WITHOUT_PARENT                           = "checking a Manifest node requires the parent node"

	MISPLACED_SENDVAL_EXPR                 = "sendval expressions are only usable within functions that are object properties, metaproperty initialization blocks and in lifetime jobs"
	MISPLACED_RECEPTION_HANDLER_EXPRESSION = "misplaced reception handler expression is misplaced, it should be an implicit key property of an object literal"

	INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS      = "invalid mapping entry key: only simple value literals and pattern identifiers are supported"
	ONLY_GLOBALS_ARE_ACCESSIBLE_FROM_RIGHT_SIDE_OF_MAPPING_ENTRIES = "only globals are accessible from the right side of mapping entries"

	MISPLACED_RUNTIME_TYPECHECK_EXPRESSION                         = "misplaced runtime typecheck expression: for now runtime typechecks are only supported as arguments in function calls (ex: map ~$ .title)"
	MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY = "misplaced compute expression: compute expressions are only allowed on the right side of a dynamic Mapping entry"
	MISPLACE_YIELD_STATEMENT_ONLY_ALLOWED_IN_EMBEDDED_MODULES      = "misplaced yield statement: yield statements are only allowed in embedded modules"
	MISPLACED_INCLUSION_IMPORT_STATEMENT_TOP_LEVEL_STMT            = "misplaced inclusion import statement: it should be located at the top level"
	MISPLACED_MOD_IMPORT_STATEMENT_TOP_LEVEL_STMT                  = "misplaced module import statement: it should be located at the top level"
	MISPLACED_PATTERN_DEF_STATEMENT_TOP_LEVEL_STMT                 = "misplaced pattern definition statement: it should be located at the top level"
	MISPLACED_PATTERN_NS_DEF_STATEMENT_TOP_LEVEL_STMT              = "misplaced pattern namespace definition statement: it should be located at the top level"
	MISPLACED_HOST_ALIAS_DEF_STATEMENT_TOP_LEVEL_STMT              = "misplaced host alias definition statement: it should be located at the top level"
	MISPLACED_READONLY_PATTERN_EXPRESSION                          = "misplaced readonly pattern expression: they are only allowed as the type of function parameters"
	MISPLACED_EXTEND_STATEMENT_TOP_LEVEL_STMT                      = "misplaced extend statement: it should be located at the top level"

	INVALID_MEM_HOST_ONLY_VALID_VALUE                                 = "invalid mem:// host, only valid value is " + MEM_HOSTNAME
	LOWER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND   = "the lower bound of an integer range literal should be smaller than the upper bound"
	LOWER_BOUND_OF_FLOAT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND = "the lower bound of a float range literal should be smaller than the upper bound"

	//lifetime job
	MISSING_LIFETIMEJOB_SUBJECT_PATTERN_NOT_AN_IMPLICIT_OBJ_PROP = "missing subject pattern of lifetime job: subject can only be ommitted for lifetime jobs that are implicit object properties"

	//visibility
	INVALID_VISIB_INIT_BLOCK_SHOULD_CONT_OBJ       = "invalid visibility initialization block: block should only contain an object literal"
	INVALID_VISIB_DESC_SHOULDNT_HAVE_METAPROPS     = "invalid visibility initialization description: object should not have metaproperties"
	INVALID_VISIB_DESC_SHOULDNT_HAVE_IMPLICIT_KEYS = "invalid visibility initialization description: object should not have implicit keys"
	VAL_SHOULD_BE_KEYLIST_LIT                      = "value should be a key list literal"
	VAL_SHOULD_BE_DICT_LIT                         = "value should be a dictionary literal"
	INVALID_VISIBILITY_DESC_KEY                    = "invalid key for visibility description"

	OPTIONAL_DYN_MEMB_EXPR_NOT_SUPPORTED_YET = "optional dynamic member expression are not supported yet"
)

func fmtNotValidPermissionKindName(name string) string {
	return fmt.Sprintf("'%s' is not a valid permission kind, valid permissions are %s", name, strings.Join(permkind.PERMISSION_KIND_NAMES, ", "))
}

func fmtUnknownSectionOfManifest(name string) string {
	return fmt.Sprintf("unknown section '%s' of manifest", name)
}

func fmtForbiddenNodeInPermListing(n parse.Node) string {
	return fmt.Sprintf("invalid permission listing: invalid node %T, only variables, simple values, objects, lists & dictionaries are allowed", n)
}

func fmtForbiddenNodeInLimitsSection(n parse.Node) string {
	return fmt.Sprintf(
		"invalid %s: invalid node %T, only variables and simple literals are allowed",
		MANIFEST_LIMITS_SECTION_NAME, n)
}

func fmtForbiddenNodeInEnvSection(n parse.Node) string {
	return fmt.Sprintf(
		"invalid %s section: invalid node %T, only variables, simple literals & named patterns are allowed",
		MANIFEST_ENV_SECTION_NAME, n)
}

func fmtForbiddenNodeInPreinitFilesSection(n parse.Node) string {
	return fmt.Sprintf(
		"invalid %s section: invalid node %T, only variables, simple literals & named patterns are allowed",
		MANIFEST_PREINIT_FILES_SECTION_NAME, n)
}

func fmtForbiddenNodeInDatabasesSection(n parse.Node) string {
	return fmt.Sprintf(
		"invalid %s section: invalid node %T, only variables, simple literals, path expressions & named patterns are allowed",
		MANIFEST_DATABASES_SECTION_NAME, n)
}

func fmtForbiddenNodeInHostResolutionSection(n parse.Node) string {
	return fmt.Sprintf(
		"invalid %s description: invalid node %T, only object literals, variables and simple literals are allowed",
		MANIFEST_HOST_RESOLUTION_SECTION_NAME, n)
}

func fmtForbiddenNodeInParametersSection(n parse.Node) string {
	return fmt.Sprintf("invalid %s description: forbidden node %T", MANIFEST_PARAMS_SECTION_NAME, n)
}

func fmtMissingPropInPreinitFileDescription(propName, name string) string {
	return fmt.Sprintf("missing .%s property in description of preinit file %s", propName, name)
}

func fmtMissingPropInDatabaseDescription(propName, name string) string {
	return fmt.Sprintf("missing .%s property in description of database %s", propName, name)
}

func fmtUnexpectedPropOfDatabaseDescription(name string) string {
	return fmt.Sprintf("unexpected property '%s' of database description", name)
}

func fmtFollowingNodeTypeNotAllowedInAssertions(n parse.Node) string {
	return fmt.Sprintf("following node type is not allowed in assertion: %T", n)
}

func fmtNonSupportedUnit(unit string) string {
	return fmt.Sprintf("non supported unit: %s", unit)
}

func fmtObjLitExplicityDeclaresPropWithImplicitKey(k string) string {
	return fmt.Sprintf("An object literal explictly declares a property with key '%s' but has the same implicit key", k)
}

func fmtRecLitExplicityDeclaresPropWithImplicitKey(k string) string {
	return fmt.Sprintf("A record literal explictly declares a property with key '%s' but has the same implicit key", k)
}

func fmtValuesOfRecordLiteralsShouldBeImmutablePropHasMutable(k string) string {
	return fmt.Sprintf("invalid value for key '%s', values of a record should be immutable", k)
}

func fmtValuesOfTupleLiteralsShouldBeImmutableElemIsMutable(i int) string {
	return fmt.Sprintf("invalid value for element at index %d, values of a tuple should be immutable", i)
}

func fmtDuplicateKey(k string) string {
	return fmt.Sprintf("duplicate key '%s'", k)
}

func fmtDuplicateDictKey(k string) string {
	return fmt.Sprintf("duplicate dictionary key '%s'", k)
}

func fmtInvalidImportStmtAlreadyDeclaredGlobal(name string) string {
	return fmt.Sprintf("invalid import statement: global '%s' is already declared", name)
}

func fmtInvalidConstDeclGlobalAlreadyDeclared(name string) string {
	return fmt.Sprintf("invalid constant declaration: '%s' is already declared", name)
}

func fmtInvalidLocalVarDeclAlreadyDeclared(name string) string {
	return fmt.Sprintf("invalid local variable declaration: '%s' is already declared", name)
}

func fmtInvalidGlobalVarAssignmentNameIsFuncName(name string) string {
	return fmt.Sprintf("invalid global variable assignment: '%s' is a declared function's name", name)
}

func fmtInvalidGlobalVarAssignmentNameIsConstant(name string) string {
	return fmt.Sprintf("invalid global variable assignment: '%s' is a constant", name)
}

func fmtInvalidGlobalVarAssignmentVarDoesNotExist(name string) string {
	return fmt.Sprintf("invalid global variable assignment: '%s' does not exist", name)
}

func fmtInvalidVariableAssignmentVarDoesNotExist(name string) string {
	return fmt.Sprintf("invalid variable assignment: '%s' does not exist", name)
}

func fmtInvalidMemberAssignmentCannotModifyMetaProperty(name string) string {
	return fmt.Sprintf("invalid member assignment: cannot modify metaproperty '%s'", name)
}

func fmtCannotShadowVariable(name string) string {
	return fmt.Sprintf("cannot shadow variable '%s', use another name instead", name)
}

func fmtCannotShadowGlobalVariable(name string) string {
	return fmt.Sprintf("cannot shadow global variable '%s', use another name instead", name)
}

func fmtCannotShadowLocalVariable(name string) string {
	return fmt.Sprintf("cannot shadow local variable '%s', use another name instead", name)
}

func fmtParameterCannotShadowGlobalVariable(name string) string {
	return fmt.Sprintf("a parameter cannot shadow global variable '%s', use another name instead", name)
}

func fmtInvalidFnDeclAlreadyDeclared(name string) string {
	return fmt.Sprintf("invalid function declaration: %s is already declared", name)
}

func fmtInvalidFnDeclGlobVarExist(name string) string {
	return fmt.Sprintf("invalid function declaration: a global variable named '%s' exists", name)
}

func fmtHostAliasAlreadyDeclared(name string) string {
	return fmt.Sprintf("host alias @%s is already declared", name)
}

func fmtPatternAlreadyDeclared(name string) string {
	return fmt.Sprintf("pattern %%%s is already declared", name)
}

func fmtPatternNamespaceAlreadyDeclared(name string) string {
	return fmt.Sprintf("pattern namespace %%%s is already declared", name)
}

func fmtCannotPassGlobalThatIsNotDeclaredToLThread(name string) string {
	return fmt.Sprintf("cannot pass global variable '%s' to lthread, '%s' is not declared", name, name)
}

func fmtCannotPassGlobalToFunction(name string) string {
	return fmt.Sprintf("cannot pass global variable '%s' to function.", name)
}

func fmtNameIsTooLong(name string) string {
	return fmt.Sprintf("name '%s' is too long", name)
}

func fmtVarIsNotDeclared(name string) string {
	return fmt.Sprintf("variable '%s' is not declared", name)
}

func fmtLocalVarIsNotDeclared(name string) string {
	return fmt.Sprintf("local variable '%s' is not declared", name)
}

func fmtGlobalVarIsNotDeclared(name string) string {
	return fmt.Sprintf("global variable '%s' is not declared", name)
}

func fmtPatternIsNotDeclared(name string) string {
	return fmt.Sprintf("pattern %%%s is not declared", name)
}

func fmtPatternNamespaceIsNotDeclared(name string) string {
	return fmt.Sprintf("pattern namespace %%%s is not declared", name)
}

func fmtObjectDoesNotHaveProp(name string) string {
	return fmt.Sprintf("object dos not have a .%s property", name)
}

func fmtOnlyAbsPathsAreAcceptedInPerms(v string) string {
	return fmt.Sprintf("only absolute paths are accepted in permissions: %s", v)
}

func fmtOnlyAbsPathPatternsAreAcceptedInPerms(v string) string {
	return fmt.Sprintf("only absolute path patterns are accepted in permissions: %s", v)
}

func fmtCannotInferPermission(kind string, name string) string {
	return fmt.Sprintf("cannot infer '%s' permission '%s", kind, name)
}
