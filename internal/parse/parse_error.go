package parse

import (
	"fmt"
	"strconv"
	"unicode"

	"github.com/inoxlang/inox/internal/inoxconsts"
)

const (
	UnspecifiedParsingError ParsingErrorKind = iota
	UnterminatedMemberExpr
	UnterminatedDoubleColonExpr
	UnterminatedExtendStmt
	UnterminatedPatternDefinition
	UnterminatedPatternNamespaceDefinition
	UnterminatedStructDefinition
	MissingBlock
	MissingFnBody
	MissingEqualsSignInDeclaration
	MissingObjectPropertyValue
	MissingObjectPatternProperty
	ExtractionExpressionExpected
	InvalidNext
	//TODO: add more kinds
)

type ParsingError struct {
	Kind    ParsingErrorKind `json:"kind"`
	Message string           `json:"message"`
}

func (err ParsingError) Error() string {
	return err.Message
}

type ParsingErrorKind int

type ParsingErrorAggregation struct {
	Message        string                `json:"completeMessage"`
	Errors         []*ParsingError       `json:"errors"`
	ErrorPositions []SourcePositionRange `json:"errorPositions"`
}

func (err ParsingErrorAggregation) Error() string {
	return err.Message
}

const (
	KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS = "keywords should not be used in left hand side of assignment"
	KEYWORDS_SHOULD_NOT_BE_USED_AS_FN_NAMES       = "keywords should not be used as function names"
	KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES    = "keywords should not be used as parameter names"

	PREINIT_KEYWORD_SHOULD_BE_FOLLOWED_BY_A_BLOCK           = "preinit keyword should be followed by a block"
	INVALID_MANIFEST_DESC_VALUE                             = "invalid manifest description value, an object is expected"
	UNTERMINATED_IDENTIFIER_LIT                             = "unterminated identifier literal"
	IDENTIFIER_LITERAL_MUST_NO_END_WITH_A_HYPHEN            = "identifier literal must not end with '-'"
	UNTERMINATED_REGEX_LIT                                  = "unterminated regex literal"
	INVALID_REGEX_LIT                                       = "invalid regex literal"
	INVALID_STRING_INTERPOLATION_SHOULD_NOT_BE_EMPTY        = "string interpolation should not be empty"
	INVALID_STRING_INTERPOLATION_SHOULD_START_WITH_A_NAME   = "string interpolation should start with a name"
	NAME_IN_STR_INTERP_SHOULD_BE_FOLLOWED_BY_COLON_AND_EXPR = "name in string interpolation should be followed by a colon and an expression"
	INVALID_STR_INTERP                                      = "invalid string interpolation"
	STR_INTERP_LIMITED_CHARSET                              = "a string interpolation can only contain a limited set of characters"
	UNTERMINATED_STRING_INTERP                              = "unterminated string interpolation"
	UNTERMINATED_STRING_TEMPL_LIT                           = "unterminated string template literal"

	//path
	INVALID_PATH_INTERP = "invalid path interpolation"
	EMPTY_PATH_INTERP   = "empty path interpolation"

	PATH_INTERP_EXPLANATION                               = "a path interpolation can only contain a limited set of characters"
	CANNOT_MIX_PATH_INTER_PATH_NAMED_SEGMENT              = "cannot mix interpolation and named path segments"
	UNTERMINATED_PATH_INTERP                              = "unterminated path interpolation"
	UNTERMINATED_PATH_INTERP_MISSING_CLOSING_BRACE        = "unterminated path interpolation; missing closing brace"
	UNTERMINATED_QUOTED_PATH_LIT_MISSING_CLOSING_BACTICK  = "unterminated quoted path literal: missing closing backtick"
	UNTERMINATED_QUOTED_PATH_EXPR_MISSING_CLOSING_BACTICK = "unterminated quoted path expression: missing closing backtick"

	//path pattern
	ONLY_PATH_PATTERNS_CAN_CONTAIN_NAMED_SEGMENTS                 = "only path patterns can contain named segments"
	INVALID_PATH_PATT_NAMED_SEGMENTS                              = "invalid path pattern literal with named segments"
	INVALID_PATH_PATT_UNBALANCED_DELIMITERS                       = "invalid path pattern literal: unbalanced delimiters"
	UNTERMINATED_QUOTED_PATH_PATTERN_LIT_MISSING_CLOSING_BACTICK  = "unterminated quoted path pattern literal: missing closing backtick"
	UNTERMINATED_QUOTED_PATH_PATTERN_EXPR_MISSING_CLOSING_BACTICK = "unterminated quoted path pattern expression : missing closing backtick"
	QUOTED_PATH_PATTERN_EXPRS_ARE_NOT_SUPPORTED_YET               = "quoted path pattern expressions are not supported yet"

	INVALID_NAMED_SEGMENT_PATH_PATTERN_COLON_SHOULD_BE_FOLLOWED_BY_A_NAME    = "invalid named-segment path pattern: colon should be followed by a name"
	INVALID_NAMED_SEGMENT_PATH_PATTERN_COLON_NAME_SHOULD_NOT_START_WITH_DASH = "invalid named-segment path pattern: name should not start with '-'"
	INVALID_NAMED_SEGMENT_PATH_PATTERN_COLON_NAME_SHOULD_NOT_END_WITH_DASH   = "invalid named-segment path pattern: name should not end with '-'"
	QUOTED_NAMED_SEGMENT_PATH_PATTERNS_ARE_NOT_SUPPORTED_YET                 = "quoted named-segment path patterns are not supported yet"

	// URL query parameter
	INVALID_QUERY                                         = "invalid query"
	QUERY_PARAM_INTERP_EXPLANATION                        = "a query parameter interpolation should contain an identifier without spaces, example: $name, name"
	UNTERMINATED_QUERY_PARAM_INTERP                       = "unterminated query parameter interpolation"
	UNTERMINATED_QUERY_PARAM_INTERP_MISSING_CLOSING_BRACE = "unterminated query parameter interpolation: missing closing brace '}'"
	INVALID_QUERY_PARAM_INTERP                            = "invalid query parameter interpolation"
	EMPTY_QUERY_PARAM_INTERP                              = "empty query parameter interpolation"

	URL_PATTERN_SUBSEQUENT_DOT_EXPLANATION                                  = "URL patterns cannot contain more than 2 subsequents dots except /... at the end"
	URL_PATTERNS_CANNOT_END_WITH_SLASH_MORE_THAN_4_DOTS                     = "URL patterns cannot end with more than 3 subsequent dots preceded by a slash"
	INVALID_URL_OR_HOST_PATT_SCHEME_SHOULD_BE_FOLLOWED_BY_COLON_SLASH_SLASH = "invalid URL or Host pattern: scheme should be followed by '://'"
	UNTERMINATED_URL_OR_HOST_PATT_MISSING_HOST                              = "unterminated URL or Host pattern: missing host part after ://"
	INVALID_URL_PATT                                                        = "invalid URL pattern"
	UNTERMINATED_PATT                                                       = "unterminated pattern: '%'"

	INVALID_COMPLEX_PATTERN_ELEMENT = "invalid complex pattern element"

	//object pattern literal
	UNTERMINATED_OBJ_PATT_MISSING_CLOSING_BRACE = "unterminated object pattern literal, missing closing brace '}'"
	ONLY_IDENTS_AND_STRINGS_VALID_OBJ_PATT_KEYS = "Only identifiers and strings are valid object pattern keys"

	INVALID_PATT_UNION_ELEMENT_SEPARATOR_EXPLANATION         = "invalid pattern union: elements should be separated by '|'"
	INVALID_PATTERN_INVALID_OCCURENCE_COUNT                  = "invalid pattern: invalid exact ocurrence count"
	UNTERMINATED_DICT_MISSING_CLOSING_BRACE                  = "unterminated dictionary literal, missing closing brace '}'"
	INVALID_DICT_KEY_ONLY_SIMPLE_VALUE_LITS                  = "invalid key for dictionary literal, only simple value literals are allowed"
	INVALID_DICT_ENTRY_MISSING_COLON_AFTER_KEY               = "invalid dictionary entry: missing colon ':' after key"
	INVALID_DICT_ENTRY_MISSING_SPACE_BETWEEN_KEY_AND_COLON   = "invalid dictionary entry: missing space between key and ':'"
	UNTERMINATED_PATT_UNTERMINATED_EXACT_OCURRENCE_COUNT     = "unterminated pattern: unterminated exact ocurrence count: missing count after '='"
	UNTERMINATED_PAREN_PATTERN_MISSING_PAREN                 = "unterminated parenthesized pattern, missing closing parenthesis"
	UNTERMINATED_PAREN_PATTERN                               = "unterminated parenthesized pattern"
	UNTERMINATED_COMPLEX_STRING_PATT_MISSING_CLOSING_BRACKET = "unterminated complex string pattern: missing closing ')'"
	INVALID_GROUP_NAME_SHOULD_NOT_END_WITH_DASH              = "invalid group name: name should not end with '-'"

	UNTERMINATED_STRING_PATTERN_ELEMENT                        = "unterminated string pattern element"
	UNTERMINATED_UNION_MISSING_CLOSING_PAREN                   = "unterminated union: missing closing ')'"
	UNTERMINATED_KEY_LIST_MISSING_BRACE                        = "unterminated key list, missing closing brace '}'"
	KEY_LIST_CAN_ONLY_CONTAIN_IDENTS                           = "a key list can only contain identifiers"
	INVALID_SCHEME_LIT_MISSING_SCHEME                          = "invalid scheme literal: '://' should be preceded by a scheme"
	INVALID_HOST_LIT                                           = "invalid host literal"
	INVALID_URL                                                = "invalid URL"
	INVALID_URL_OR_HOST                                        = "invalid URL or Host"
	INVALID_HOST_INTERPOLATION                                 = "invalid host interpolation"
	URL_EXPR_CANNOT_CONTAIN_INTERP_NEXT_TO_EACH_OTHER          = "an URL expression cannot contain interpolations next to each others"
	URL_EXPR_CANNOT_END_WITH_SLASH_3DOTS                       = "an URL expression cannot end with /..."
	INVALID_HOST_PATT                                          = "invalid host pattern"
	INVALID_HOST_PATT_SUGGEST_DOUBLE_STAR                      = "invalid host pattern: maybe you wanted to write '**' instead of '*'"
	INVALID_HOST_PATT_AT_MOST_ONE_DOUBLE_STAR                  = "invalid host pattern: at most one '**' can be used"
	INVALID_HOST_PATT_ONLY_SINGLE_OR_DOUBLE_STAR               = "invalid host pattern: more than two '*' do not make sense"
	INVALID_PORT_LITERAL_INVALID_PORT_NUMBER                   = "invalid port literal: invalid port number, maximum is 65_535"
	UNTERMINATED_PORT_LITERAL_MISSING_SCHEME_NAME_AFTER_SLASH  = "unterminated port literal; missing scheme name after '/'"
	UNTERMINATED_BLOCK_MISSING_BRACE                           = "unterminated block, missing closing brace '}'"
	EMPTY_CSS_SELECTOR                                         = "empty CSS selector"
	INVALID_PSEUDO_CSS_SELECTOR_INVALID_NAME                   = "invalid CSS pseudo element selector, invalid name"
	INVALID_CSS_CLASS_SELECTOR_INVALID_NAME                    = "invalid CSS class selector, invalid name"
	INVALID_CSS_SELECTOR                                       = "invalid CSS selector"
	UNTERMINATED_CSS_ATTRIBUTE_SELECTOR_MISSING_BRACKET        = "unterminated CSS attribute selector, missing closing bracket"
	UNTERMINATED_CSS_ATTR_SELECTOR_INVALID_PATTERN             = "unterminated CSS attribute selector, invalid pattern"
	UNTERMINATED_CSS_ATTR_SELECTOR_PATTERN_EXPECTED_AFTER_NAME = "unterminated CSS attribute selector, a pattern is expected after the name"
	CSS_ATTRIBUTE_NAME_SHOULD_START_WITH_ALPHA_CHAR            = "an attribute name should start with an alpha character like identifiers"
	UNTERMINATED_CSS_ATTR_SELECTOR_NAME_EXPECTED               = "unterminated CSS attribute selector, an attribute name was expected"
	UNTERMINATED_CSS_ID_SELECTOR_NAME_EXPECTED                 = "unterminated CSS id selector, a name was expected after '#'"
	UNTERMINATED_CSS_CLASS_SELECTOR_NAME_EXPECTED              = "unterminated CSS class selector, a name was expected"

	//list & tuple literals
	UNTERMINATED_LIST_LIT_MISSING_CLOSING_BRACKET            = "unterminated list literal, missing closing bracket ']'"
	UNTERMINATED_SPREAD_ELEM_MISSING_EXPR                    = "unterminated spread element: missing expression"
	UNTERMINATED_LIST_LIT_MISSING_OPENING_BRACKET_AFTER_TYPE = "unterminated list literal, missing opening bracket '[' after type annotation"

	UNTERMINATED_RUNE_LIT                                            = "unterminated rune literal"
	INVALID_RUNE_LIT_NO_CHAR                                         = "invalid rune literal: no character"
	INVALID_RUNE_LIT_INVALID_SINGLE_CHAR_ESCAPE                      = "invalid rune literal: invalid single character escape"
	UNTERMINATED_RUNE_LIT_MISSING_QUOTE                              = "unterminated rune literal, missing ' at the end"
	INVALID_RUNE_RANGE_EXPR                                          = "invalid rune range expression"
	INVALID_UPPER_BOUND_RANGE_EXPR                                   = "invalid upper-bound range expression"
	UNTERMINATED_QUOTED_STRING_LIT                                   = "unterminated quoted string literal"
	UNTERMINATED_MULTILINE_STRING_LIT                                = "unterminated multiline string literal"
	UNKNOWN_BYTE_SLICE_BASE                                          = "unknown byte slice base"
	UNTERMINATED_HEX_BYTE_SICE_LIT_MISSING_BRACKETS                  = "unterminated hexadecimal byte slice literal: missing brackets"
	UNTERMINATED_BIN_BYTE_SICE_LIT_MISSING_BRACKETS                  = "unterminated binary byte slice literal: missing brackets"
	UNTERMINATED_DECIMAL_BYTE_SICE_LIT_MISSING_BRACKETS              = "unterminated decimal byte slice literal: missing brackets"
	INVALID_HEX_BYTE_SICE_LIT_LENGTH_SHOULD_BE_EVEN                  = "invalid hexadecimal byte slice literal: length should be even"
	INVALID_HEX_BYTE_SICE_LIT_FAILED_TO_DECODE                       = "invalid hexadecimal byte slice literal: failed to decode"
	UNTERMINATED_BYTE_SICE_LIT_MISSING_CLOSING_BRACKET               = "unterminated byte slice literal: missing closing bracket"
	DOT_SHOULD_BE_FOLLOWED_BY                                        = "'.' should be followed by (.)?(/), or a letter"
	DASH_SHOULD_BE_FOLLOWED_BY_OPTION_NAME                           = "'-' should be followed by an option name"
	DOUBLE_DASH_SHOULD_BE_FOLLOWED_BY_OPTION_NAME                    = "'--' should be followed by an option name"
	OPTION_NAME_CAN_ONLY_CONTAIN_ALPHANUM_CHARS                      = "the name of an option can only contain alphanumeric characters"
	UNTERMINATED_OPION_EXPR_EQUAL_ASSIGN_SHOULD_BE_FOLLOWED_BY_EXPR  = "unterminated option expression, '=' should be followed by an expression"
	UNTERMINATED_OPION_PATT_EQUAL_ASSIGN_SHOULD_BE_FOLLOWED_BY_EXPR  = "unterminated option pattern, '=' should be followed by an expression"
	UNTERMINATED_OPION_PATTERN_A_VALUE_IS_EXPECTED_AFTER_EQUAKL_SIGN = "unterminated option pattern, a value is expected after '='"
	AT_SYMBOL_SHOULD_BE_FOLLOWED_BY                                  = "'@' should be followed by '(' <expr> ')' or by the name of variable (@host/path)"
	UNTERMINATED_URL_EXPRESSION                                      = "unterminated url expression"
	INVALID_HOST_ALIAS_DEF_MISSING_VALUE_AFTER_EQL_SIGN              = "unterminated HostAliasDefinition, missing value after '='"

	//parenthesized expression
	UNTERMINATED_PARENTHESIZED_EXPR_MISSING_CLOSING_PAREN = "unterminated parenthesized expression: missing closing parenthesis"

	//unary expression
	UNTERMINATED_UNARY_EXPR_MISSING_OPERAND = "unterminated unary expression: missing operand"

	//binary expression
	INVALID_BIN_EXPR_NON_EXISTING_OPERATOR                    = "invalid binary expression, non existing operator"
	UNTERMINATED_BIN_EXPR_MISSING_OPERATOR                    = "unterminated binary expression: missing operator"
	UNTERMINATED_BIN_EXPR_MISSING_OPERAND_OR_INVALID_OPERATOR = "unterminated binary expression: missing right operand and/or invalid operator"
	UNTERMINATED_BIN_EXPR_MISSING_OPERAND                     = "unterminated binary expression: missing right operand"
	UNTERMINATED_BIN_EXPR_MISSING_PAREN                       = "unterminated binary expression: missing closing parenthesis"
	BIN_EXPR_CHAIN_OPERATORS_SHOULD_BE_THE_SAME               = "the operators of a binary expression chain should be all the same: either 'or' or 'and'"
	MOST_BINARY_EXPRS_MUST_BE_PARENTHESIZED                   = "most binary expressions must be parenthesized, (e.g. '(1 + 2 + 3)' is not valid)"

	UNTERMINATED_MEMB_OR_INDEX_EXPR                          = "unterminated member/index expression"
	UNTERMINATED_IDENT_MEMB_EXPR                             = "unterminated identifier member expression"
	UNTERMINATED_DYN_MEMB_OR_INDEX_EXPR                      = "unterminated dynamic member/index expression"
	UNTERMINATED_INDEX_OR_SLICE_EXPR                         = "unterminated index/slice/double-colon expression"
	INVALID_SLICE_EXPR_SINGLE_COLON                          = "invalid slice expression, a single colon should be present"
	UNTERMINATED_SLICE_EXPR_MISSING_END_INDEX                = "unterminated slice expression, missing end index"
	UNTERMINATED_INDEX_OR_SLICE_EXPR_MISSING_CLOSING_BRACKET = "unterminated index/slice expression, missing closing bracket ']'"
	UNTERMINATED_DOUBLE_COLON_EXPR                           = "unterminated double-colon expression"
	UNTERMINATED_CALL_MISSING_CLOSING_PAREN                  = "unterminated call, missing closing parenthesis ')'"
	UNTERMINATED_GLOBAL_CONS_DECLS                           = "unterminated global const declarations"
	INVALID_GLOBAL_CONST_DECLS_OPENING_PAREN_EXPECTED        = "invalid global const declarations: expected opening parenthesis after ''"
	INVALID_GLOBAL_CONST_DECLS_MISSING_CLOSING_PAREN         = "invalid global const declarations: missing closing parenthesis"
	INVALID_GLOBAL_CONST_DECL_LHS_MUST_BE_AN_IDENT           = "invalid global const declaration: left hand side must be an identifier"
	INVALID_GLOBAL_CONST_DECL_MISSING_EQL_SIGN               = "invalid global const declaration missing '='"

	//pattern call
	UNTERMINATED_PATTERN_CALL_MISSING_CLOSING_PAREN = "unterminated pattern call: missing closing parenthesis ')'"

	//mapping expression
	UNTERMINATED_MAPPING_EXPRESSION_MISSING_BODY              = "unterminated mapping expression: missing body"
	UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE     = "unterminated mapping expression: missing closing brace"
	UNTERMINATED_MAPPING_ENTRY                                = "unterminated mapping entry"
	INVALID_DYNAMIC_MAPPING_ENTRY_GROUP_MATCHING_VAR_EXPECTED = "invalid dynamic mapping entry: group matching variable expected"
	UNTERMINATED_MAPPING_ENTRY_MISSING_ARROW_VALUE            = "unterminated mapping entry: missing '=> <value>' after key"

	//treedata literal
	UNTERMINATED_TREEDATA_LIT_MISSING_OPENING_BRACE   = "unterminated treedata literal: missing opening brace"
	UNTERMINATED_TREEDATA_LIT_MISSING_CLOSING_BRACE   = "unterminated treedata literal: missing closing brace"
	UNTERMINATED_TREEDATA_ENTRY_MISSING_CLOSING_BRACE = "unterminated treedata entry: missing closing brace"
	UNTERMINATED_TREEDATA_ENTRY                       = "unterminated treedata entry"

	//test suite
	UNTERMINATED_TESTSUITE_EXPRESSION_MISSING_BLOCK = "unterminated test suite expression: missing block"
	UNTERMINATED_TESTCASE_EXPRESSION_MISSING_BLOCK  = "unterminated test case expression: missing block"

	// lifetimejob
	UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_META            = "unterminated lifetimejob expression: missing meta"
	UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_EMBEDDED_MODULE = "unterminated lifetimejob expression: missing embedded module"

	//send value expression

	UNTERMINATED_SENDVALUE_EXPRESSION_MISSING_VALUE                 = "unterminated send value expression: missing value after 'sendval' keyword"
	UNTERMINATED_SENDVALUE_EXPRESSION_MISSING_TO_KEYWORD            = "unterminated send value expression: missing value after 'to' keyword after value"
	INVALID_SENDVALUE_EXPRESSION_MISSING_TO_KEYWORD_BEFORE_RECEIVER = "invalid send value expression: 'to' keyword missing before receiver value"

	//concatenation expression
	UNTERMINATED_CONCAT_EXPR_ELEMS_EXPECTED = "unterminated concatenation expression: at least one element is expected after keyword 'concat'"

	//local var declarations
	UNTERMINATED_LOCAL_VAR_DECLS                       = "unterminated local variable declarations"
	INVALID_LOCAL_VAR_DECLS_OPENING_PAREN_EXPECTED     = "invalid local variable declarations, expected opening parenthesis after ''"
	UNTERMINATED_LOCAL_VAR_DECLS_MISSING_CLOSING_PAREN = "unterminated local variable declarations, missing closing parenthesis"
	INVALID_LOCAL_VAR_DECL_LHS_MUST_BE_AN_IDENT        = "invalid local variable declaration, left hand side must be an identifier"
	EQUAL_SIGN_MISSING_AFTER_TYPE_ANNOTATION           = "'=' missing after type annotation"

	//global var declarations
	UNTERMINATED_GLOBAL_VAR_DECLS                       = "unterminated global variable declarations"
	INVALID_GLOBAL_VAR_DECLS_OPENING_PAREN_EXPECTED     = "invalid global variable declarations, expected opening parenthesis after ''"
	UNTERMINATED_GLOBAL_VAR_DECLS_MISSING_CLOSING_PAREN = "unterminated global variable declarations, missing closing parenthesis"
	INVALID_GLOBAL_VAR_DECL_LHS_MUST_BE_AN_IDENT        = "invalid global variable declaration, left hand side must be an identifier"

	//spawn expression
	UNTERMINATED_SPAWN_EXPRESSION_MISSING_EMBEDDED_MODULE_AFTER_GO_KEYWORD = "unterminated spawn expression: missing embedded module after 'go' keyword"
	UNTERMINATED_SPAWN_EXPRESSION_MISSING_DO_KEYWORD_AFTER_META            = "unterminated spawn expression: missing 'do' keyword after meta value"
	UNTERMINATED_SPAWN_EXPRESSION_MISSING_EMBEDDED_MODULE_AFTER_DO_KEYWORD = "unterminated spawn expression: missing embedded module after 'do' keyword"
	SPAWN_EXPR_ARG_SHOULD_BE_FOLLOWED_BY_ALLOW_KEYWORD                     = "spawn expression: argument should be followed by the 'allow' keyword"
	SPAWN_EXPR_ALLOW_KEYWORD_SHOULD_BE_FOLLOWED_BY_OBJ_LIT                 = "spawn expression: 'allow' keyword should be followed by an object literal (permissions)"
	SPAWN_EXPR_ONLY_SIMPLE_CALLS_ARE_SUPPORTED                             = "spawn expression: only simple calls are supported for now"

	// reception handler expression

	UNTERMINATED_RECEP_HANDLER_MISSING_RECEIVED_KEYWORD   = "unterminated reception handler expression: missing 'received' keyword after 'on' keyword"
	INVALID_RECEP_HANDLER_MISSING_RECEIVED_KEYWORD        = "invalid reception handler expression: missing 'received' keyword after 'on' keyword"
	UNTERMINATED_RECEP_HANDLER_MISSING_PATTERN            = "unterminated reception handler expression: missing pattern value"
	UNTERMINATED_RECEP_HANDLER_MISSING_HANDLER_OR_PATTERN = "unterminated reception handler expression: missing handler or pattern"
	MISSING_RECEIVED_KEYWORD                              = "missing 'received' keyword after 'on' keyword"

	//watch expression
	INVALID_WATCH_EXPR                                              = "invalid watch expression"
	UNTERMINATED_WATCH_EXPR_MISSING_MODULE                          = "unterminated watch expression: missing module"
	INVALID_WATCH_EXP_ONLY_SIMPLE_CALLS_ARE_SUPPORTED               = "invalid watch expression: only simple calls are supported for now"
	INVALID_WATCH_EXP_MODULE_SHOULD_BE_FOLLOWED_BY_WITHCONF_KEYWORD = "invalid watch expression: module should be followed by 'with-config' keyword"

	FN_KEYWORD_OR_FUNC_NAME_SHOULD_BE_FOLLOWED_BY_PARAMS       = "function: fn keyword (or function name) should be followed by parameters"
	CAPTURE_LIST_SHOULD_BE_FOLLOWED_BY_PARAMS                  = "capture list should be followed by parameters"
	PARAM_LIST_OF_FUNC_SHOULD_CONTAIN_PARAMETERS_SEP_BY_COMMAS = "the parameter list of a function should contain parameters (a parameter is an identifier followed (or not) by a type) separated by commas"
	UNTERMINATED_CAPTURE_LIST_MISSING_CLOSING_BRACKET          = "unterminated capture list: missing closing bracket"

	PERCENT_FN_SHOULD_BE_FOLLOWED_BY_PARAMETERS                     = "'%fn' should be followed by parameters "
	PARAM_LIST_OF_FUNC_PATT_SHOULD_CONTAIN_PARAMETERS_SEP_BY_COMMAS = "the parameter list of a function pattern should contain parameters (a parameter is a type preceded (or not) by an identifier) separated by commas"

	UNTERMINATED_PARAM_LIST_MISSING_CLOSING_PAREN            = "unterminated parameter list: missing closing parenthesis"
	INVALID_FUNC_SYNTAX                                      = "invalid function syntax"
	PARAM_LIST_OF_FUNC_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW  = "function: parameter list should be followed by a block or an arrow"
	RETURN_TYPE_OF_FUNC_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW = "function: return type should be followed by a block or an arrow, note that 'int{' is a pattern call shorthand, you might have forgotten a space after the return type"
	UNTERMINATED_IF_STMT_MISSING_BLOCK                       = "unterminated if statement: block is missing"
	UNTERMINATED_LIST_TUPLE_PATT_LIT_MISSING_BRACE           = "unterminated list/tuple pattern literal, missing closing bracket ']'"
	INVALID_LIST_TUPLE_PATT_GENERAL_ELEMENT_IF_ELEMENTS      = "invalid list/tuple pattern literal, the general element (after ']') should not be specified if there are elements"

	UNTERMINATED_SWITCH_CASE_MISSING_BLOCK  = "invalid switch case: missing block"
	UNTERMINATED_MATCH_CASE_MISSING_BLOCK   = "invalid match case: missing block"
	UNTERMINATED_DEFAULT_CASE_MISSING_BLOCK = "invalid default case: missing block"

	DEFAULT_CASE_MUST_BE_UNIQUE = "default case must be unique"

	UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE = "unterminated switch statement: missing closing body brace '}'"
	UNTERMINATED_MATCH_STMT_MISSING_CLOSING_BRACE  = "unterminated match statement: missing closing body brace '}'"

	INVALID_SWITCH_CASE_VALUE_EXPLANATION   = "invalid switch case: only simple value literals (1, 1.0, /home, ..) are supported"
	INVALID_MATCH_CASE_VALUE_EXPLANATION    = "invalid match case: only values that are statically known can be used"
	UNTERMINATED_MATCH_STMT                 = "unterminated match statement"
	UNTERMINATED_SWITCH_STMT                = "unterminated switch statement"
	UNTERMINATED_SWITCH_STMT_MISSING_BODY   = "unterminated switch statement: missing body"
	UNTERMINATED_MATCH_STMT_MISSING_BODY    = "unterminated match statement: missing body"
	UNTERMINATED_SWITCH_STMT_MISSING_VALUE  = "unterminated switch statement: missing value"
	UNTERMINATED_MATCH_STMT_MISSING_VALUE   = "unterminated match statement: missing value"
	DROP_PERM_KEYWORD_SHOULD_BE_FOLLOWED_BY = "permission dropping statement: 'drop-perms' keyword should be followed by an object literal (permissions)"

	//module import
	IMPORT_STMT_IMPORT_KEYWORD_SHOULD_BE_FOLLOWED_BY_IDENT = "import statement: the 'import' keyword should be followed by an identifier"
	IMPORT_STMT_SRC_SHOULD_BE_AN_URL_OR_PATH_LIT           = "import statement: source should be a URL literal or Path literal"
	IMPORT_STMT_CONFIG_SHOULD_BE_AN_OBJ_LIT                = "import statement: configuration should be an object literal"

	//inclusion import
	INCLUSION_IMPORT_STMT_SRC_SHOULD_BE_A_PATH_LIT         = "inclusion import statement: source should be path literal (/file.ix, ./file.ix, ...)"
	INCLUSION_IMPORT_STMT_VALID_STR_SHOULD_BE_A_STRING_LIT = "inclusion import statement: validation should be a string literal"

	//import
	PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH     = "path literals used as import sources should not contain '//'"
	PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH = "path literals used as import sources should not contain '..' segments; if possible use an absolute path literal instead"
	PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS   = "path literals used as import sources should not contain segments with only a dot (e.g. /./file.ix); `./` is allowed at the start though"

	PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH     = "the path of URL literals used as import sources should not contain '//'"
	PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH = "the path of URL literals used as import sources should not contain '..' segments"
	PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS   = "the path of URL literals used as import sources should not contain segments with only a dot (e.g. /./file.ix)"

	URL_LITS_AND_PATH_LITS_USED_AS_IMPORT_SRCS_SHOULD_END_WITH_IX = "URL literals and path literals used as import sources should end with `" + inoxconsts.INOXLANG_FILE_EXTENSION + "`"

	UNTERMINATED_EMBEDDED_MODULE = "unterminated embedded module"

	//For ... statement.

	INVALID_FOR_STMT                                        = "invalid for statement"
	UNTERMINATED_FOR_STMT                                   = "unterminated for statement"
	INVALID_FOR_STMT_MISSING_IN_KEYWORD                     = "invalid for statement: missing 'in' keyword "
	INVALID_FOR_STMT_IN_KEYWORD_SHOULD_BE_FOLLOWED_BY_SPACE = "invalid for statement: 'in' keyword should be followed by a space"
	INVALID_FOR_STMT_MISSING_VALUE_AFTER_IN                 = "unterminated for statement: missing value after 'in'"
	UNTERMINATED_FOR_STMT_MISSING_BLOCK                     = "unterminated for statement: missing block"

	//For ... expression.

	INVALID_FOR_EXPR                                        = "invalid for ... expression"
	UNTERMINATED_FOR_EXPR                                   = "unterminated for expression"
	INVALID_FOR_EXPR_MISSING_IN_KEYWORD                     = "invalid for expression: missing 'in' keyword "
	INVALID_FOR_EXPR_IN_KEYWORD_SHOULD_BE_FOLLOWED_BY_SPACE = "invalid for expression: 'in' keyword should be followed by a space"
	INVALID_FOR_EXPR_MISSING_VALUE_AFTER_IN                 = "unterminated for expression: missing value after 'in'"
	UNTERMINATED_FOR_EXPR_MISSING_BODY                      = "unterminated for expression: missing body"
	UNTERMINATED_FOR_EXPR_MISSING_CLOSIN_PAREN              = "unterminated for expression: missing closing parenthesis"

	UNTERMINATED_WALK_STMT_MISSING_WALKED_VALUE        = "unterminated walk statement: missing walked value"
	UNTERMINATED_WALK_STMT_MISSING_ENTRY_VARIABLE_NAME = "unterminated walk statement: missing entry variable's name"
	INVALID_WALK_STMT_MISSING_ENTRY_IDENTIFIER         = "invalid walk statement: missing entry identifier"
	UNTERMINATED_WALK_STMT_MISSING_BLOCK               = "unterminated walk statement: missing block"

	UNTERMINATED_MULTI_ASSIGN_MISSING_EQL_SIGN             = "unterminated multi assign statement: missing '=' sign"
	ASSIGN_KEYWORD_SHOULD_BE_FOLLOWED_BY_IDENTS            = "assign keyword should be followed by identifiers (assign a b = <value>)"
	UNTERMINATED_ASSIGNMENT_MISSING_VALUE_AFTER_EQL_SIGN   = "unterminated assignment, missing value after the '=' sign"
	INVALID_ASSIGN_A_PIPELINE_EXPR_WAS_EXPECTED_AFTER_PIPE = "invalid assignment: a pipeline expression was expected after the '|' symbol"
	UNTERMINATED_ASSIGNMENT_MISSING_TERMINATOR             = "unterminated assignment: missing terminator (';' or end of line), if you are trying to write a binary expression note that binary expressions are always parenthesized, example: myvar = (1 + 2)"

	UNTERMINATED_PIPE_STMT_LAST_STAGE_EMPTY                                       = "unterminated pipeline statement: last stage is empty"
	INVALID_PIPE_STATE_ALL_STAGES_SHOULD_BE_CALLS                                 = "invalid pipeline stage, all pipeline stages should be calls"
	UNTERMINATED_CALL                                                             = "unterminated call"
	A_NON_PAREN_CALL_EXPR_SHOULD_HAVE_ARGS_AND_CALLEE_SHOULD_BE_FOLLOWED_BY_SPACE = "a non-parenthesized call expression should have arguments and the callee (<name>$) should be followed by a space"

	INVALID_INT_LIT                                    = "invalid integer literal"
	UNTERMINATED_INT_RANGE_LIT                         = "unterminated integer range literal"
	UPPER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_INT_LIT     = "upper bound of an integer range literal should be a integer literal"
	UPPER_BOUND_OF_FLOAT_RANGE_LIT_SHOULD_BE_FLOAT_LIT = "upper bound of a float range literal should be a float literal"

	UNTERMINATED_QTY_RANGE_LIT                     = "unterminated quantity range literal"
	UPPER_BOUND_OF_QTY_RANGE_LIT_SHOULD_BE_QTY_LIT = "upper bound of a quantity range literal should be a quantity literal"

	INVALID_FLOAT_LIT                                      = "invalid floating point literal"
	INVALID_QUANTITY_LIT                                   = "invalid quantity literal"
	QUANTITY_LIT_NOT_ALLOWED_WITH_HEXADECIMAL_NUM          = "quantity literals with a hexadecimal number are not allowed"
	QUANTITY_LIT_NOT_ALLOWED_WITH_OCTAL_NUM                = "quantity literals with an octal number are not allowed"
	INVALID_RATE_LIT                                       = "invalid rate literal"
	INVALID_RATE_LIT_DIV_SYMBOL_SHOULD_BE_FOLLOWED_BY_UNIT = "invalid rate literal: '/' should be immediately followed by a unit"

	INVALID_DATE_LIKE_LITERAL                                 = "invalid date-like literal"
	INVALID_DATELIKE_LITERAL_MISSING_LOCATION_PART_AT_THE_END = "invalid date-like literal: missing location part at the end (e.g.,`-UTC`, `-America/Los_Angeles`)"

	//year literal

	INVALID_YEAR_LITERAL = "invalid year literal"

	//date literal

	INVALID_YEAR_OR_DATE_LITERAL                      = "invalid year or date literal"
	INVALID_DATE_LITERAL_DAY_COUNT_PROBABLY_MISSING   = "invalid date literal: the day count is probably missing (example: '-5d' for the 5th day)"
	INVALID_DATE_LITERAL_MONTH_COUNT_PROBABLY_MISSING = "invalid date literal: the month count is probably missing (example: '-1mt' for the first month, January)"

	//datetime literal

	UNTERMINATED_DATE_LITERAL = "unterminated datetime literal"
	INVALID_DATETIME_LITERAL  = "invalid datetime literal"

	INVALID_DATETIME_LITERAL_DAY_COUNT_PROBABLY_MISSING                = "invalid datetime literal: the day count is probably missing (example: '-5d' for the 5th day)"
	INVALID_DATETIME_LITERAL_MONTH_COUNT_PROBABLY_MISSING              = "invalid datetime literal: the month count is probably missing (example: '-1mt' for the first month, January)"
	INVALID_DATETIME_LITERAL_BOTH_MONTH_AND_DAY_COUNT_PROBABLY_MISSING = //
	"invalid datetime literal: both the month and day count are probably missing (example: '-1mt-1d' for the first of January)"

	MISSING_MONTH_VALUE = "missing month value"
	INVALID_MONTH_VALUE = "invalid month value"
	MISSING_DAY_VALUE   = "missing day value"
	INVALID_DAY_VALUE   = "invalid day value"

	//synchronized
	SYNCHRONIZED_KEYWORD_SHOULD_BE_FOLLOWED_BY_SYNC_VALUES = "synchronized keyword should be followed by synchronized values"
	UNTERMINATED_SYNCHRONIZED_MISSING_BLOCK                = "unterminated synchronized block: missing block"

	//object literals
	INVALID_OBJ_REC_LIT_ENTRY_SEPARATION    = "invalid object/record literal, each entry should be followed by '}', newline, or ','."
	INVALID_OBJ_REC_LIT_SPREAD_SEPARATION   = "invalid object/record literal, a spread should be followed by '}', newline or ','."
	MISSING_PROPERTY_VALUE                  = "missing property value"
	MISSING_PROPERTY_PATTERN                = "missing property pattern"
	UNEXPECTED_NEWLINE_AFTER_COLON          = "unexpected newline after colon"
	ONLY_EXPLICIT_KEY_CAN_HAVE_A_TYPE_ANNOT = "only explicit keys can have a type annotation"
	METAPROP_KEY_CANNOT_HAVE_A_TYPE_ANNOT   = "metaproperty keys cannot have a type annotation"
	UNTERMINATED_OBJ_MISSING_CLOSING_BRACE  = "unterminated object literal, missing closing brace '}'"
	UNTERMINATED_REC_MISSING_CLOSING_BRACE  = "unterminated record literal, missing closing brace '}'"

	//object pattern literals
	INVALID_OBJ_PATT_LIT_ENTRY_SEPARATION                = "invalid object/record pattern literal, each entry should be followed by '}', newline, or ','."
	METAPROPS_ARE_NOT_ALLOWED_IN_OBJECT_PATTERNS         = "metaproperties are not allowed in object patterns"
	A_KEY_IS_REQUIRED_FOR_EACH_VALUE_IN_OBJ_REC_PATTERNS = "a key is required for each value in object/record patterns"
	UNTERMINATED_OBJ_PATTERN_MISSING_CLOSING_BRACE       = "unterminated object pattern literal, missing closing brace '}'"
	UNTERMINATED_REC_PATTERN_MISSING_CLOSING_BRACE       = "unterminated record pattern literal, missing closing brace '}'"
	SPREAD_SHOULD_BE_LOCATED_AT_THE_START                = "spread should be located at the start"

	INVALID_DICT_LIT_ENTRY_SEPARATION                     = "invalid dictionary literal, each entry should be followed by '}', newline, or ','."
	UNTERMINATED_IF_STMT_MISSING_BLOCK_AFTER_ELSE         = "unterminated if statement, missing block after 'else'"
	UNTERMINATED_IF_EXPR_MISSING_VALUE_AFTER_ELSE         = "unterminated if expression, missing value after 'else'"
	UNTERMINATED_IF_EXPR_MISSING_CLOSING_PAREN            = "unterminated if expression: missing closing parenthesis'"
	SPREAD_ARGUMENT_CANNOT_BE_FOLLOWED_BY_ADDITIONAL_ARGS = "a spread argument cannot be followed by additional arguments"
	CAPTURE_LIST_SHOULD_ONLY_CONTAIN_IDENTIFIERS          = "capture list should only contain identifiers"
	VARIADIC_PARAM_IS_UNIQUE_AND_SHOULD_BE_LAST_PARAM     = "the variadic parameter should be unique and should be the last parameter"
	STMTS_SHOULD_BE_SEPARATED_BY                          = "statements should be separated by a space, newline or ';'"

	//xml
	UNTERMINATED_XML_EXPRESSION_MISSING_TOP_ELEM_NAME  = "unterminated xml expression: missing name of top element"
	UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING       = "unterminated opening xml tag: missing closing delimiter '>'"
	UNTERMINATED_SELF_CLOSING_XML_TAG_MISSING_CLOSING  = "unterminated self-closing xml tag: missing closing '>' after '/'"
	UNTERMINATED_XML_INTERP                            = "unterminated xml interpolation"
	UNTERMINATED_CLOSING_XML_TAG_MISSING_CLOSING_DELIM = "unterminated closing xml tag: missing closing delimiter '>' after tag name"
	UNTERMINATED_HYPERSCRIPT_ATTRIBUTE_SHORTHAND       = "unterminated hyperscript attribute shorthand"
	EMPTY_XML_INTERP                                   = "xml interpolation should not be empty"
	INVALID_XML_INTERP                                 = "invalid xml interpolation"
	XML_INTERP_SHOULD_CONTAIN_A_SINGLE_EXPR            = "an xml interpolation should contain a single expression"
	XML_ATTRIBUTE_NAME_SHOULD_BE_IDENT                 = "xml attribute's name should be an identifier"
	INVALID_TAG_NAME                                   = "invalid tag name"

	//pattern definition
	UNTERMINATED_PATT_DEF_MISSING_NAME_AFTER_PATTERN_KEYWORD      = "unterminated pattern definition: missing name after 'pattern' keyword"
	UNTERMINATED_PATT_DEF_MISSING_EQUAL_SYMBOL_AFTER_PATTERN_NAME = "unterminated pattern definition: missing '=' symbol after the pattern's name"
	UNTERMINATED_PATT_DEF_MISSING_RHS                             = "unterminated pattern definition: missing pattern after '='"

	//pattern namespace definition
	UNTERMINATED_PATT_NS_DEF_MISSING_NAME_AFTER_PATTERN_KEYWORD      = "unterminated pattern namespace definition: missing name after 'pnamss' keyword"
	UNTERMINATED_PATT_NS_DEF_MISSING_EQUAL_SYMBOL_AFTER_PATTERN_NAME = "unterminated pattern namespace definition: missing '=' symbol after the namespace's name"
	UNTERMINATED_PATT_NS_DEF_MISSING_RHS                             = "unterminated pattern namespace definition: missing definition after '='"
	A_PATTERN_NAMESPACE_NAME_WAS_EXPECTED                            = "a pattern namespace name was expected (e.g. http. , models.), make sure to add a trailing point after the name."

	//extend statement
	UNTERMINATED_EXTEND_STMT_MISSING_PATTERN_TO_EXTEND_AFTER_KEYWORD       = "unterminated extend statement: missing pattern after keyword 'extend'"
	UNTERMINATED_EXTEND_STMT_MISSING_OBJECT_LITERAL_AFTER_EXTENDED_PATTERN = "unterminated extend statement: missing object literal (extension) after pattern"
	A_PATTERN_NAME_WAS_EXPECTED                                            = "a pattern name was expected"
	INVALID_EXTENSION_VALUE_AN_OBJECT_LITERAL_WAS_EXPECTED                 = "invalid extension value: an object literal was expected"

	//struct definition
	UNTERMINATED_STRUCT_DEF_MISSING_NAME_AFTER_KEYWORD           = "unterminated struct definition: missing name after keyword 'struct'"
	A_NAME_WAS_EXPECTED                                          = "a name was expected"
	UNTERMINATED_STRUCT_DEF_MISSING_BODY                         = "unterminated struct definition: missing body"
	UNTERMINATED_STRUCT_BODY_MISSING_CLOSING_BRACE               = "unterminated struct body: missing closing brace"
	ONLY_FIELD_AND_METHOD_DEFINITIONS_ARE_ALLOWED_IN_STRUCT_BODY = "only field and method definitions are allowed inside a struct body"

	//new expression
	UNTERMINATED_NEW_EXPR_MISSING_TYPE_AFTER_KEYWORD   = "unterminated 'new' expression: missing type after keyword 'new'"
	UNTERMINATED_STRUCT_INIT_LIT_MISSING_CLOSING_BRACE = "unterminated struct initialization literal: missing closing brace"

	//struct initalization literal
	ONLY_FIELD_INIT_PAIRS_ALLOWED  = "only field initialization pairs are allowed"
	MISSING_COLON_AFTER_FIELD_NAME = "missing colon after field name"

	//value path literals
	UNTERMINATED_VALUE_PATH_LITERAL = "unterminated value path literal"
)

func fmtInvalidRegexLiteral(err string) string {
	return fmt.Sprintf("invalid regex literal: %s", err)
}

func fmtOnlyIdentsAndStringsValidObjRecordKeysNot(v Node) string {
	var s string
	if lit, ok := v.(*UnquotedStringLiteral); ok {
		s = "(" + lit.Value + ")"
	}
	return fmt.Sprintf("Only identifiers and strings are valid object/record literal keys, not a(n) %T %s", v, s)
}

func fmtOnlyIdentsAndStringsValidObjPatternKeysNot(v Node) string {
	var s string
	if lit, ok := v.(*UnquotedStringLiteral); ok {
		s = "(" + lit.Value + ")"
	}
	return fmt.Sprintf("Only identifiers and strings are valid object pattern literal keys, not a(n) %T %s", v, s)
}

func fmtPrefixPattCannotContainGlobbingPattern(value string) string {
	return fmt.Sprintf("prefix path patterns cannot contain globbing patterns '%s'", value)
}

func fmtSlashDotDotDotCanOnlyBePresentAtEndOfPathPattern(value string) string {
	return fmt.Sprintf("'/...' can only appear at the end of a path pattern '%s'", value)
}

func fmtAPatternWasExpected(s []rune, i int32) string {
	before := string(s[max(0, i-5):max(i, len32(s))])

	return fmt.Sprintf("a pattern was expected at this location: ...%s<<here>>", before)
}

func fmtInvalidObjRecordKeyMissingColonAfterKey(lastKeyName string) string {
	return fmt.Sprintf("invalid object/record literal, missing colon after key '%s'", lastKeyName)
}

func fmtInvalidObjPatternKeyMissingColonAfterKey(lastKeyName string) string {
	return fmt.Sprintf("invalid object pattern literal, missing colon after key '%s'", lastKeyName)
}

func fmtInvalidObjKeyMissingColonAfterTypeAnnotation(lastKeyName string) string {
	return fmt.Sprintf("invalid object literal, missing colon after type annotation for key '%s'", lastKeyName)
}

func fmtInvalidObjRecordKeyCommentBeforeValueOfKey(lastKeyName string) string {
	return fmt.Sprintf("invalid object/record literal, comment before value of key '%s'", lastKeyName)
}

func fmtInvalidObjPatternKeyCommentBeforeValueOfKey(lastKeyName string) string {
	return fmt.Sprintf("invalid object pattern literal, comment before value of key '%s'", lastKeyName)
}

func fmtInvalidURIUnsupportedProtocol(protocol string) string {
	return fmt.Sprintf("invalid URI: unsupported protocol '%s'", protocol)
}

func fmtPropNameShouldStartWithAletterNot(r rune) string {
	return fmt.Sprintf("property name should start with a letter, not '%s'", string(r))
}

func fmtDoubleColonExpressionelementShouldStartWithAletterNot(r rune) string {
	return fmt.Sprintf("element of double-colon expression should start with a letter, not '%s'", string(r))
}

func fmtPatternNamespaceMemberShouldStartWithAletterNot(r rune) string {
	return fmt.Sprintf("pattern namespace member should start with a letter, not '%s'", string(r))
}

func fmtInvalidQueryKeysCannotContaintDollar(key string) string {
	return fmt.Sprintf("invalid query: keys cannot contain '$' or '{' characters: key '%s'", key)
}

func fmtInvalidQueryMissingEqualSignAfterKey(key string) string {
	return fmt.Sprintf("invalid query: missing '=' after key '%s'", key)
}

func fmtInvalidStringLitJSON(jsonErr string) string {
	return fmt.Sprintf("invalid string literal: json string: %s", jsonErr)
}

func fmtExprExpectedHere(s []rune, i int32, showRight bool) string {
	left := string(s[max(0, i-5):i])

	var right = ""
	if showRight {
		right = string(s[i:min(len32(s), i+5)])
	}

	return fmt.Sprintf("an expression was expected: ...%s<<here>>%s...", left, right)
}

func fmtCaseValueExpectedHere(s []rune, i int32, showRight bool) string {
	left := string(s[max(0, i-5):i])

	var right = ""
	if showRight {
		right = string(s[i:min(len(s), int(i+5))])
	}

	return fmt.Sprintf("a value was expected: ...%s<<here>>%s..., object literals should be surrounded by parentheses", left, right)
}

func fmtAPatternWasExpectedHere(s []rune, i int32) string {
	left := string(s[max(0, i-5):i])
	right := string(s[i:min(len(s), int(i+5))])

	return fmt.Sprintf("a pattern was expected: ...%s<<here>>%s...", left, right)
}

func fmtInvalidConstDeclMissingEqualsSign(name string) string {
	return fmt.Sprintf("invalid global const declaration, missing '=' sign after name %s", name)
}

func fmtInvalidLocalVarDeclMissingEqualsSign(name string) string {
	return fmt.Sprintf("invalid local variable declaration, missing '=' sign after name %s", name)
}

func fmtInvalidGlobalVarDeclMissingEqualsSign(name string) string {
	return fmt.Sprintf("invalid global variable declaration, missing '=' sign after name %s", name)
}

func fmtFuncNameShouldBeAnIdentNot(identLike Node) string {
	return fmt.Sprintf("function name should be an identifier, not a(n) %T", identLike)
}

func fmtUnterminatedIfStmtShouldBeFollowedByBlock(r rune) string {
	return fmt.Sprintf("invalid if statement, test expression should be followed by a block, not '%s'", string(r))
}

func fmtUnterminatedIfStmtElseShouldBeFollowedByBlock(r rune) string {
	return fmt.Sprintf("invalid if statement, 'else' should be followed by a block a or 'if', not '%s'", string(r))
}

func fmtForStmtKeyIndexShouldBeFollowedByCommaNot(r rune) string {
	return fmt.Sprintf("for statement: key/index name should be followed by a comma ',' , not %s", string(r))
}

func fmtInvalidForStmtKeyIndexVarShouldBeFollowedByVarNot(keyIndexIdent Node) string {
	return fmt.Sprintf("invalid for statement: 'for <key-index var> <colon> should be followed by a variable, not a(n) %T", keyIndexIdent)
}

func fmtForExprKeyIndexShouldBeFollowedByCommaNot(r rune) string {
	return fmt.Sprintf("for expression: key/index name should be followed by a comma ',' , not %s", string(r))
}

func fmtInvalidForExprKeyIndexVarShouldBeFollowedByVarNot(keyIndexIdent Node) string {
	return fmt.Sprintf("invalid for expression: 'for <key-index var> <colon> should be followed by a variable, not a(n) %T", keyIndexIdent)
}

func fmtInvalidPipelineStageUnexpectedChar(r rune) string {
	return fmt.Sprintf("invalid pipeline stage, unexpected char '%c'", r)
}

func fmtRuneInfo(r rune) string {

	var runeRepr string
	switch r {
	case '\t':
		runeRepr = "'\\t'"
	case '\r':
		runeRepr = "'\\r'"
	case '\n':
		runeRepr = "'\\n'"
	default:
		runeRepr = fmt.Sprintf("'%c'", r)
	}

	runeInfo := runeRepr + " (code: "
	if unicode.IsSpace(r) && r != ' ' && r != '\t' && r != '\r' && r != '\n' {
		runeInfo = runeRepr + " (non regular space, code: "
	}

	runeInfo += strconv.Itoa(int(r)) + ")"
	return runeInfo
}

func fmtUnexpectedCharInBlockOrModule(r rune) string {
	return fmt.Sprintf("unexpected char %s in block or module", fmtRuneInfo(r))
}

func fmtUnexpectedCharInParenthesizedExpression(r rune) string {
	return fmt.Sprintf("unexpected char %s in parenthesized expresison", fmtRuneInfo(r))
}

func fmtUnexpectedCharInCallArguments(r rune) string {
	return fmt.Sprintf("unexpected char %s in call arguments", fmtRuneInfo(r))
}

func fmtUnexpectedCharInPatternCallArguments(r rune) string {
	return fmt.Sprintf("unexpected char %s in pattern call arguments", fmtRuneInfo(r))
}

func fmtUnexpectedCharInParameters(r rune) string {
	return fmt.Sprintf("unexpected char %s in parameters", fmtRuneInfo(r))
}

func fmtUnexpectedCharInCaptureList(r rune) string {
	return fmt.Sprintf("unexpected char %s in capture lisr", fmtRuneInfo(r))
}

func fmtUnexpectedCharInKeyList(r rune) string {
	return fmt.Sprintf("unexpected char %s in key list", fmtRuneInfo(r))
}

func fmtUnexpectedCharInDictionary(r rune) string {
	return fmt.Sprintf("unexpected char %s in dictionary", fmtRuneInfo(r))
}

func fmtUnexpectedCharInObjectRecord(r rune) string {
	return fmt.Sprintf("unexpected char %s in object or record", fmtRuneInfo(r))
}

func fmtUnexpectedCharInObjectPattern(r rune) string {
	return fmt.Sprintf("unexpected char %s in object or record pattern", fmtRuneInfo(r))
}

func fmtUnexpectedCharInSwitchOrMatchStatement(r rune) string {
	return fmt.Sprintf("unexpected char %s in switch or match statement", fmtRuneInfo(r))
}

func fmtUnexpectedCharInMappingExpression(r rune) string {
	return fmt.Sprintf("unexpected char %s in mapping expression", fmtRuneInfo(r))
}

func fmtUnexpectedCharInTreedataLiteral(r rune) string {
	return fmt.Sprintf("unexpected char %s in treedata literal", fmtRuneInfo(r))
}

func fmtUnexpectedCharInHexadecimalByteSliceLiteral(r rune) string {
	return fmt.Sprintf("unexpected char %s in hexadecimal byte slice literal", fmtRuneInfo(r))
}

func fmtUnexpectedCharInBinByteSliceLiteral(r rune) string {
	return fmt.Sprintf("unexpected char %s in binary byte slice literal", fmtRuneInfo(r))
}

func fmtUnexpectedCharInDecimalByteSliceLiteral(r rune) string {
	return fmt.Sprintf("unexpected char %s in decimal byte slice literal", fmtRuneInfo(r))
}

func fmtInvalidByteInDecimalByteSliceLiteral(s []byte) string {
	return fmt.Sprintf("invalid byte %s in decimal byte slice literal", s)
}

func fmtUnexpectedCharInSynchronizedValueList(r rune) string {
	return fmt.Sprintf("unexpected char %s in synchronized value list", fmtRuneInfo(r))
}

func fmtUnexpectedCharInStructBody(r rune) string {
	return fmt.Sprintf("unexpected char %s in struct body", fmtRuneInfo(r))
}

func fmtUnexpectedCharInStructInitLiteral(r rune) string {
	return fmt.Sprintf("unexpected char %s in struct initialization literal", fmtRuneInfo(r))
}

func fmtInvalidSpreadElemExprShouldBeExtrExprNot(expr Node) string {
	return fmt.Sprintf("invalid spread element in object literal: expression should be an extraction expression, not a(n) %T. Example: {...obj.{a, b}}", expr)
}

func fmtInvalidAssignmentInvalidLHS(expr Node) string {
	return fmt.Sprintf("invalid assignment: cannot assign a(n) %T", expr)
}

func fmtExpectedClosingTag(name string) string {
	return fmt.Sprintf("expected closing '%s' tag", name)
}
