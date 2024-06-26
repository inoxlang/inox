package text

import (
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/ast"

	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
)

const (
	MODULE_IMPORTS_NOT_ALLOWED_IN_INCLUDABLE_FILES = "module imports are not allowed in includable files"

	//global constant declarations
	VAR_CONST_NOT_DECLARED_IF_YOU_MEANT_TO_DECLARE_CONSTANTS_GLOBAL_CONST_DECLS_ONLY_SUPPORTED_AT_THE_START_OF_THE_MODULE = //
	"variable 'const' is not declared, if you meant to declare constants note that a single global constant declaration section at the start of the module is supported for now"

	CALLED_NOT_ALLOWED_INSIDE_GLOBAL_CONST_DECLS                         = "this callee is not allowed inside global constant declarations"
	CALL_EXPRS_NOT_ALLOWED_INSIDE_GLOBAL_CONST_DECLS_OF_INCLUDABLE_FILES = "call expressions are not allowed inside the global constant declarations of includable files"

	//Manifest

	NO_SPREAD_IN_MANIFEST            = "objects & lists in the manifest cannot contain spread elements"
	ELEMENTS_NOT_ALLOWED_IN_MANIFEST = "elements (valus without a key) are not allowed in the manifest object"

	//Kind section

	KIND_SECTION_SHOULD_BE_A_STRING_LITERAL                      = "the '" + inoxconsts.MANIFEST_KIND_SECTION_NAME + "' section of the manifest should have a string value (string literal)"
	INVALID_KIND_SECTION_EMBEDDED_MOD_KINDS_NOT_ALLOWED          = "invalid '" + inoxconsts.MANIFEST_KIND_SECTION_NAME + "' section: embedded module kinds are not allowed"
	THE_UNSPECIFIED_MOD_KIND_NAME_CANNOT_BE_USED_IN_THE_MANIFEST = "the '" + inoxconsts.UNSPECIFIED_MODULE_KIND_NAME +
		"' module kind name cannot be used in the manifest"
	MOD_KIND_NOT_EQUAL_TO_KIND_DETERMINED_DURING_PARSING = "module kind not equal to the kind determined during parsing, " +
		"the kind inferred from the filename may be conflicting with the kind specified in the manifest"
	MOD_KIND_SPECIFIED_IN_MANIFEST_SHOULD_BE_SPEC_OR_SHOULD_BE_OMITTED = "the module is inferred as a specification module, " +
		"so the kind specified in the manifest should be '" + inoxconsts.SPEC_MODULE_KIND_NAME + "' or should be omitted"

	//Permissions section

	PERMS_SECTION_SHOULD_BE_AN_OBJECT     = "the '" + inoxconsts.MANIFEST_PERMS_SECTION_NAME + "' section of the manifest should be an object"
	ELEMENTS_NOT_ALLOWED_IN_PERMS_SECTION = "elements are not allowed in the 'permissions' section"

	//Limits section

	LIMITS_SECTION_SHOULD_BE_AN_OBJECT = "the '" + inoxconsts.MANIFEST_LIMITS_SECTION_NAME + "' section of the manifest should be an object"

	//Env section

	ENV_SECTION_SHOULD_BE_AN_OBJECT_PATTERN                = "the '" + inoxconsts.MANIFEST_ENV_SECTION_NAME + "' section of the manifest should be an object pattern literal"
	ENV_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS = "the '" + inoxconsts.MANIFEST_ENV_SECTION_NAME + "' section is not available in embedded module manifests"

	//Parameters section

	PARAMS_SECTION_SHOULD_BE_AN_OBJECT                        = "the '" + inoxconsts.MANIFEST_PARAMS_SECTION_NAME + "' section of the manifest should be an object literal"
	PARAMS_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS = "the '" + inoxconsts.MANIFEST_PARAMS_SECTION_NAME + "' section is not available in embedded module manifests"

	FORBIDDEN_NODE_TYPE_IN_INCLUDABLE_CHUNK_IMPORTED_BY_PREINIT = "forbidden node type in includable file imported by preinit"

	//Permissions

	NO_PERM_DESCRIBED_BY_THIS_TYPE_OF_VALUE         = "there is no permission described by this type of value"
	NO_PERM_DESCRIBED_BY_STRINGS                    = "there is no permission described by strings"
	MAYBE_YOU_MEANT_TO_WRITE_A_PATH_LITERAL         = "maybe you meant to write a path literal such as /dir/ or /data.json (always unquoted)"
	MAYBE_YOU_MEANT_TO_WRITE_A_PATH_PATTERN_LITERAL = "maybe you meant to write a path pattern literal such as %/... or %/*.json (always unquoted)"
	MAYBE_YOU_MEANT_TO_WRITE_A_URL_LITERAL          = "maybe you meant to write a url literal such as https://example.com/ (always unquoted)"
	MAYBE_YOU_MEANT_TO_WRITE_A_URL_PATTERN_LITERAL  = "maybe you meant to write a url pattern literal such as %https://example.com/... (always unquoted)"

	//preinit-files section
	PREINIT_FILES_SECTION_SHOULD_BE_AN_OBJECT                        = "the '" + inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME + "' section of the manifest should be an object literal"
	PREINIT_FILES__FILE_CONFIG_SHOULD_BE_AN_OBJECT                   = "the description of each file in the '" + inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME + "' section of the manifest should be an object literal"
	PREINIT_FILES__FILE_CONFIG_PATH_SHOULD_BE_ABS_PATH               = "the ." + inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME + " of each file in the '" + inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME + "' section (manifest) should be an absolute path"
	PREINIT_FILES_SECTION_NOT_AVAILABLE_IN_EMBEDDED_MODULE_MANIFESTS = "the '" + inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME + "' section is not available in embedded module manifests"

	HOST_DEFS_SECTION_SHOULD_BE_A_DICT = "the '" + inoxconsts.MANIFEST_HOST_DEFINITIONS_SECTION_NAME + "' section of the manifest should be a dictionary with host keys"
	HOST_SCHEME_NOT_SUPPORTED          = "the host's scheme is not supported"

	//includable file
	AN_INCLUDABLE_FILE_CAN_ONLY_CONTAIN_DEFINITIONS = "an includable file should only contain definitions (functions, patterns, ...) and inclusion imports"

	INVALID_RATE     = "invalid rate"
	INVALID_QUANTITY = "invalid quantity"

	//spawn expression
	INVALID_SPAWN_EXPR_EXPR_SHOULD_BE_ONE_OF                             = "invalid spawn expression: the expression should be a simple function call or an embedded module (that can be global)"
	INVALID_SPAWN_GLOBALS_SHOULD_BE                                      = "invalid spawn expression: the description of globals should be a key list literal or an object literal with no implicit-key properties nor spread elements"
	INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED = "invalid spawn expression: only object literals with no spread elements nor implicit-key properties are supported for meta's value"

	INVALID_ASSIGNMENT_ANONYMOUS_VAR_CANNOT_BE_ASSIGNED                         = "invalid assignment: anonymous variable '$' cannot be assigned"
	INVALID_ASSIGNMENT_EQUAL_ONLY_SUPPORTED_ASSIGNMENT_OPERATOR_FOR_SLICE_EXPRS = "invalid assignment: '=' is the only supported assignment operators for slice expressions"

	INVALID_FN_DECL_SHOULD_BE_TOP_LEVEL_STMT             = "invalid function declaration: a function declaration should be a top level statement in a module (embedded or not)"
	CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT = "continue statements are only allowed in the body of a 'for' or 'walk' statement"
	BREAK_STMTS_ONLY_ALLOWED_LOCATION                    = "break statements are only allowed in the body of a switch or match statement, or of a 'for' or 'walk' statement"

	YIELD_STMTS_ONLY_ALLOWED_IN_BODY_FOR_WALK_EXPR       = "yield statements are only allowed in the body of a 'for' or 'walk' expression"
	PRUNE_STMTS_ARE_ONLY_ALLOWED_IN_WALK_STMTS_AND_EXPRS = "prune statement are only allowed in 'walk' statements and expressions"

	SELF_ACCESSIBILITY_EXPLANATION = "'self' is only accessible within " +
		"extension methods, metaproperty initialization blocks"
	CANNOT_CHECK_OBJECT_PROP_WITHOUT_PARENT     = "checking an ObjectProperty node requires the parent ObjectLiteral node"
	CANNOT_CHECK_OBJECT_METAPROP_WITHOUT_PARENT = "checking an ObjectMetaProperty node requires the parent ObjectLiteral node"
	OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS       = "object-like literals cannot have metaproperty keys, metaproperty keys have a (single) starting underscore '_' and a (single) trailing underscore"
	CANNOT_CHECK_MANIFEST_WITHOUT_PARENT        = "checking a Manifest node requires the parent node"

	//object literal
	ELEMENTS_NOT_ALLOWED_IF_EMPTY_PROP_NAME = "elements are not allowed if the empty property name is present"
	EMPTY_PROP_NAME_NOT_ALLOWED_IF_ELEMENTS = "the empty property name is not allowed if there are elements (values without a key)"

	//object pattern literals
	UNEXPECTED_OTHER_PROPS_EXPR_OTHERPROPS_NO_IS_PRESENT = "unexpected otherprops expression: no other properties are allowed since otherprops(no) is present"

	INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS      = "invalid mapping entry key: only simple value literals and pattern identifiers are supported"
	ONLY_GLOBALS_ARE_ACCESSIBLE_FROM_RIGHT_SIDE_OF_MAPPING_ENTRIES = "only globals are accessible from the right side of mapping entries"

	MISPLACED_RUNTIME_TYPECHECK_EXPRESSION                         = "misplaced runtime typecheck expression: for now runtime typechecks are only supported as arguments in function calls (ex: map ~$ .title)"
	MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY = "misplaced compute expression: compute expressions are only allowed on the right side of a dynamic Mapping entry"
	MISPLACE_COYIELD_STATEMENT_ONLY_ALLOWED_IN_EMBEDDED_MODULES    = "misplaced coyield statement: coyield statements are only allowed in embedded modules"
	MISPLACED_INCLUSION_IMPORT_STATEMENT_TOP_LEVEL_STMT            = "misplaced inclusion import statement: it should be located at the module's top level or a the top level of the preinit block"
	MISPLACED_MOD_IMPORT_STATEMENT_TOP_LEVEL_STMT                  = "misplaced module import statement: it should be located at the top level"

	MISPLACED_PATTERN_DEF_NOT_TOP_LEVEL_STMT         = "misplaced pattern definition: it should be located at the top level"
	MISPLACED_PATTERN_DEF_AFTER_FN_DECL_OR_REF_TO_FN = "misplaced pattern definition: definitions are not allowed after a function declaration, or after a reference to a function that is declared further below"

	MISPLACED_PATTERN_NS_DEF_NOT_TOP_LEVEL_STMT         = "misplaced pattern namespace definition: it should be located at the top level"
	MISPLACED_PATTERN_NS_DEF_AFTER_FN_DECL_OR_REF_TO_FN = "misplaced pattern namespace definition: definitions are not allowed after a function declaration, or after a reference to a function that is declared further below"

	MISPLACED_READONLY_PATTERN_EXPRESSION                 = "misplaced readonly pattern expression: they are only allowed as the type of function parameters"
	MISPLACED_EXTEND_STATEMENT_TOP_LEVEL_STMT             = "misplaced extend statement: it should be located at the top level"
	MISPLACED_GLOBAL_VAR_DECLS_TOP_LEVEL_STMT             = "misplaced global variable declaration(s): declarations are only allowed at the top level"
	MISPLACED_GLOBAL_VAR_DECLS_AFTER_FN_DECL_OR_REF_TO_FN = "misplaced global variable declaration(s): declarations are not allowed after a function declaration, or after a reference to a function that is declared further below"

	GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED = "global variables and constants cannot be re-assigned"

	INVALID_MEM_HOST_ONLY_VALID_VALUE = "invalid mem:// host, only valid value is " + inoxconsts.MEM_HOSTNAME
	CREDENTIALS_NOT_ALLOWED_IN_URLS   = "credentials are not allowed in URLs"

	LOWER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND   = "the lower bound of an integer range literal should be smaller than the upper bound"
	LOWER_BOUND_OF_FLOAT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND = "the lower bound of a float range literal should be smaller than the upper bound"

	//visibility
	INVALID_VISIB_INIT_BLOCK_SHOULD_CONT_OBJ   = "invalid visibility initialization block: block should only contain an object literal"
	INVALID_VISIB_DESC_SHOULDNT_HAVE_METAPROPS = "invalid visibility initialization description: object should not have metaproperties"
	INVALID_VISIB_DESC_SHOULDNT_HAVE_ELEMENTS  = "invalid visibility initialization description: object should not have elements (values without a key)"
	VAL_SHOULD_BE_KEYLIST_LIT                  = "value should be a key list literal"
	VAL_SHOULD_BE_DICT_LIT                     = "value should be a dictionary literal"
	INVALID_VISIBILITY_DESC_KEY                = "invalid key for visibility description"

	VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES = "variables are not allowed in the extended pattern and " +
		"in the extension object's properties"

	//test suites & cases
	TEST_CASES_NOT_ALLOWED_IF_SUBSUITES_ARE_PRESENT     = "test cases are not allowed if sub suites are presents"
	TEST_CASE_STMTS_NOT_ALLOWED_OUTSIDE_OF_TEST_SUITES  = "test case statements are not allowed outside of test suites"
	TEST_SUITE_STMTS_NOT_ALLOWED_INSIDE_TEST_CASE_STMTS = "test suite statements are not allowed in test case statements"

	//return statements
	MISPLACED_RETURN_STATEMENT = "misplaced return statement"

	//markup patterns
	ONLY_X_ARE_SUPPORTED_AS_PATTERNS_FOR_MARKUP_PATTERN_ATTRIBUTES = //
	"only pattern-names (e.g. int), pattern namespace members (e.g. mynamespace.int), variables (e.g. $varname) and member expressions " +
		"(e.g. $varname.property) are supported as patterns for markup pattern attributes"

	ONLY_X_ARE_SUPPORTED_IN_MARKUP_PATTERN_INTERPOLATIONS = //
	"only pattern-names (e.g. int), pattern namespace members (e.g. mynamespace.int), variables (e.g. $varname) and member expressions are " +
		"(e.g. $varname.property) are supported in markup pattern interpolations"
)

var (
	A_LIMITED_NUMBER_OF_BUILTINS_ARE_ALLOWED_TO_BE_CALLED_IN_GLOBAL_CONST_DECLS = //
	"a limited number of builtins are allowed to be called in global constant declarations: " + strings.Join(globalnames.USABLE_GLOBALS_IN_PREINIT, " ")
)

func FmtNotValidPermissionKindName(name string) string {
	return fmt.Sprintf("'%s' is not a valid permission kind, valid permissions are %s", name, strings.Join(permbase.PERMISSION_KIND_NAMES, ", "))
}

func FmtUnknownSectionOfManifest(name string) string {
	return fmt.Sprintf("unknown section '%s' of manifest", name)
}

func FmtForbiddenNodeInPermListing(n ast.Node) string {
	return fmt.Sprintf("invalid permission listing: invalid node %T, only variables, simple values, objects, lists & dictionaries are allowed", n)
}

func FmtForbiddenNodeInLimitsSection(n ast.Node) string {
	return fmt.Sprintf(
		"invalid %s: invalid node %T, only variables and simple literals are allowed",
		inoxconsts.MANIFEST_LIMITS_SECTION_NAME, n)
}

func FmtForbiddenNodeInEnvSection(n ast.Node) string {
	return fmt.Sprintf(
		"invalid %s section: invalid node %T, only variables, simple literals & named patterns are allowed",
		inoxconsts.MANIFEST_ENV_SECTION_NAME, n)
}

func FmtForbiddenNodeInPreinitFilesSection(n ast.Node) string {
	return fmt.Sprintf(
		"invalid %s section: invalid node %T, only variables, simple literals & named patterns are allowed",
		inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME, n)
}

func FmtForbiddenNodeInHostDefinitionsSection(n ast.Node) string {
	return fmt.Sprintf(
		"invalid %s description: invalid node %T, only object literals, variables and simple literals are allowed",
		inoxconsts.MANIFEST_HOST_DEFINITIONS_SECTION_NAME, n)
}

func FmtForbiddenNodeInParametersSection(n ast.Node) string {
	return fmt.Sprintf("invalid %s description: forbidden node %T", inoxconsts.MANIFEST_PARAMS_SECTION_NAME, n)
}

func FmtMissingPropInPreinitFileDescription(propName, name string) string {
	return fmt.Sprintf("missing .%s property in description of preinit file %s", propName, name)
}

func FmtMissingPropInDatabaseDescription(propName, name string) string {
	return fmt.Sprintf("missing .%s property in description of database %s", propName, name)
}

func FmtUnexpectedPropOfDatabaseDescription(name string) string {
	return fmt.Sprintf("unexpected property '%s' of database description", name)
}

func FmtUnexpectedPropOfInvocationDescription(name string) string {
	return fmt.Sprintf("unexpected property '%s' of invocation description", name)
}

func FmtFollowingNodeTypeNotAllowedInAssertions(n ast.Node) string {
	return fmt.Sprintf("following node type is not allowed in assertion: %T", n)
}

func FmtFollowingNodeTypeNotAllowedInMarkupPatterns(n ast.Node) string {
	return fmt.Sprintf("following node type is not allowed in markup patterns: %T", n)
}

func FmtFollowingNodeTypeNotAllowedInGlobalConstantDeclarations(n ast.Node) string {
	return fmt.Sprintf("following node type is not allowed in global constant declarations: %T", n)
}

func FmtNonSupportedUnit(unit string) string {
	return fmt.Sprintf("non supported unit: %s", unit)
}

func FmtValuesOfRecordLiteralsShouldBeImmutablePropHasMutable(k string) string {
	return fmt.Sprintf("invalid value for key '%s', values of a record should be immutable", k)
}

func FmtValuesOfTupleLiteralsShouldBeImmutableElemIsMutable(i int) string {
	return fmt.Sprintf("invalid value for element at index %d, values of a tuple should be immutable", i)
}

func FmtDuplicateKey(k string) string {
	return fmt.Sprintf("duplicate key '%s'", k)
}

func FmtDuplicateFieldName(k string) string {
	return fmt.Sprintf("duplicate field name '%s'", k)
}

func FmtDuplicateDictKey(k string) string {
	return fmt.Sprintf("duplicate dictionary key '%s'", k)
}

func FmtInvalidImportStmtAlreadyDeclaredGlobal(name string) string {
	return fmt.Sprintf("invalid import statement: global '%s' is already declared", name)
}

func FmtInvalidConstDeclGlobalAlreadyDeclared(name string) string {
	return fmt.Sprintf("invalid constant declaration: '%s' is already declared", name)
}

func FmtInvalidLocalVarDeclAlreadyDeclared(name string) string {
	return fmt.Sprintf("invalid local variable declaration: '%s' is already declared", name)
}

func FmtVarIsAlreadyDeclared(name string) string {
	return fmt.Sprintf("variable '%s' is already declared", name)
}

func FmtInvalidGlobalVarDeclAlreadyDeclared(name string) string {
	return fmt.Sprintf("invalid global variable declaration: '%s' is already declared", name)
}

func FmtInvalidAssignmentNameIsFuncName(name string) string {
	return fmt.Sprintf("invalid assignment: '%s' is a declared function's name", name)
}

func FmtInvalidVariableAssignmentVarDoesNotExist(name string) string {
	return fmt.Sprintf("invalid variable assignment: '%s' does not exist", name)
}

func FmtInvalidMemberAssignmentCannotModifyMetaProperty(name string) string {
	return fmt.Sprintf("invalid member assignment: cannot modify metaproperty '%s'", name)
}

func FmtCannotShadowVariable(name string) string {
	return fmt.Sprintf("cannot shadow variable '%s', use another name instead", name)
}

func FmtCannotShadowGlobalVariable(name string) string {
	return fmt.Sprintf("cannot shadow global variable '%s', use another name instead", name)
}

func FmtCannotShadowGlobalConstant(name string) string {
	return fmt.Sprintf("cannot shadow global constant '%s', use another name instead", name)
}

func FmtCannotShadowLocalVariable(name string) string {
	return fmt.Sprintf("cannot shadow local variable '%s', use another name instead", name)
}

func FmtParameterCannotShadowGlobalVariable(name string) string {
	return fmt.Sprintf("a parameter cannot shadow global variable '%s', use another name instead", name)
}

func FmtParameterAlreadyDeclared(name string) string {
	return fmt.Sprintf("parameter '%s' is already declared", name)
}

func FmtInvalidFnDeclAlreadyDeclared(name string) string {
	return fmt.Sprintf("invalid function declaration: %s is already declared", name)
}

func FmtInvalidOrMisplacedFnDeclShouldBeAfterCapturedVarDeclaration(name string) string {
	return fmt.Sprintf("invalid or misplaced function declaration: the function should be declared after the declaration of the local variable '%s'", name)
}

func FmtVarIsAlreadyCaptured(name string) string {
	return fmt.Sprintf("variable '%s' is already captured", name)
}

func FmtInvalidFnDeclGlobVarExist(name string) string {
	return fmt.Sprintf("invalid function declaration: a global variable named '%s' exists", name)
}

func FmtMisplacedFnDeclGlobVarExist(name string) string {
	return fmt.Sprintf("misplaced function declaration: a global variable named '%s' exists", name)
}

func FmtAnXFieldOrMethodIsAlreadyDefined(name string) string {
	return fmt.Sprintf("a field or method named '%s' is already defined ", name)
}

func FmtPatternAlreadyDeclared(name string) string {
	return fmt.Sprintf("pattern %%%s is already declared", name)
}

func FmtPatternNamespaceAlreadyDeclared(name string) string {
	return fmt.Sprintf("pattern namespace %%%s is already declared", name)
}

func FmtPatternNamespaceDoesNotHaveMember(namespace, member string) string {
	return fmt.Sprintf("pattern namespace %%%s. does not have a member '%s'", namespace, member)
}

func FmtCannotPassGlobalThatIsNotDeclaredToLThread(name string) string {
	return fmt.Sprintf("cannot pass global variable '%s' to lthread, '%s' is not declared", name, name)
}

func FmtCannotPassGlobalToFunction(name string) string {
	return fmt.Sprintf("cannot pass global variable '%s' to function.", name)
}

func FmtNameIsTooLong(name string) string {
	return fmt.Sprintf("name '%s' is too long", name)
}

func FmtVarIsNotDeclared(name string) string {
	return fmt.Sprintf("variable '%s' is not declared", name)
}

func FmtLocalVarIsNotDeclared(name string) string {
	return fmt.Sprintf("local variable '%s' is not declared", name)
}

func FmtGlobalVarIsNotDeclared(name string) string {
	return fmt.Sprintf("global variable '%s' is not declared", name)
}

func FmtPatternIsNotDeclared(name string) string {
	return fmt.Sprintf("pattern %%%s is not declared", name)
}

func FmtPatternNamespaceIsNotDeclared(name string) string {
	return fmt.Sprintf("pattern namespace %%%s is not declared", name)
}

func FmtObjectDoesNotHaveProp(name string) string {
	return fmt.Sprintf("object dos not have a .%s property", name)
}

func FmtOnlyAbsPathsAreAcceptedInPerms(v string) string {
	return fmt.Sprintf("only absolute paths are accepted in permissions: %s", v)
}

func FmtOnlyAbsPathPatternsAreAcceptedInPerms(v string) string {
	return fmt.Sprintf("only absolute path patterns are accepted in permissions: %s", v)
}

func FmtCannotInferPermission(kind string, name string) string {
	return fmt.Sprintf("cannot infer '%s' permission '%s", kind, name)
}

func FmtTheXSectionIsNotAllowedForTheCurrentModuleKind(sectionName string, moduleKind fmt.Stringer) string {
	return fmt.Sprintf("the %q section is not allowed for the current module kind (%s)", sectionName, moduleKind)
}
