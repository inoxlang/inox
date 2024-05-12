package parse

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"unicode/utf8"

	"slices"
)

const (
	MIN_TOKEN_CACHING_COUNT = 2

	AND_LEN = int32(len(AND_KEYWORD_STRING))
	OR_LEN  = int32(len(OR_KEYWORD_STRING))
	AS_LEN  = int32(len(AS_KEYWORD_STRING))

	OTHERPROPS_KEYWORD_STRING      = "otherprops"
	ASSERT_KEYWORD_STRING          = "assert"
	IF_KEYWORD_STRING              = "if"
	FOR_KEYWORD_STRING             = "for"
	WALK_KEYWORD_STRING            = "walk"
	SWITCH_KEYWORD_STRING          = "switch"
	MATCH_KEYWORD_STRING           = "match"
	STRUCT_KEYWORD_STRING          = "struct"
	NEW_KEYWORD_STRING             = "new"
	INCLUDABLE_FILE_KEYWORD_STRING = "includable-file"
	FN_KEYWORD_STRING              = "fn"
	READONLY_KEYWORD_STRING        = "readonly"

	COYIELD_KEYWORD_STRING = "coyield"
	YIELD_KEYWORD_STRING   = "yield"

	AS_KEYWORD_STRING  = "as"
	AND_KEYWORD_STRING = "and"
	OR_KEYWORD_STRING  = "or"

	OPTIONAL_MARKUP_ELEMENT_QUANTIFIER      = "?"
	OPTIONAL_MARKUP_ELEMENT_QUANTIFIER_RUNE = '?'

	ONE_OR_MORE_MARKUP_ELEMENT_QUANTIFIER      = "+"
	ONE_OR_MORE_MARKUP_ELEMENT_QUANTIFIER_RUNE = '+'

	ZERO_OR_MORE_MARKUP_ELEMENT_QUANTIFIER      = "*"
	ZERO_OR_MORE_MARKUP_ELEMENT_QUANTIFIER_RUNE = '*'

	MARKUP_STAR_WILDCARD = "*"
)

var (
	tokenCache     = map[uintptr][]Token{} //Node address -> tokens
	tokenCacheLock sync.Mutex
)

type Token struct {
	Type    TokenType    `json:"type"`
	SubType TokenSubType `json:"subType"`
	Meta    TokenMeta    `json:"meta"`
	Span    NodeSpan     `json:"span"`
	Raw     string       `json:"raw"`
}

func (t Token) Str() string {
	if t.Raw != "" {
		return t.Raw
	}
	if t.Type <= NEWLINE {
		return tokenStrings[t.Type]
	}
	panic(fmt.Errorf("invalid token: %#v", t))
}

type TokenType uint16

const (
	LAST_TOKEN_TYPE_WITHOUT_VALUE = NEWLINE
)

const (
	//WITH NO ASSOCIATED VALUE
	IF_KEYWORD TokenType = iota + 1
	ELSE_KEYWORD
	PREINIT_KEYWORD
	MANIFEST_KEYWORD
	INCLUDABLE_FILE_KEYWORD
	DROP_PERMS_KEYWORD
	ASSIGN_KEYWORD
	READONLY_KEYWORD
	CONST_KEYWORD
	VAR_KEYWORD
	GLOBALVAR_KEYWORD
	FOR_KEYWORD
	WALK_KEYWORD
	IN_KEYWORD
	NOT_IN_KEYWORD
	IS_KEYWORD
	IS_NOT_KEYWORD
	KEYOF_KEYWORD
	URLOF_KEYWORD
	SUBSTROF_KEYWORD
	NOT_MATCH_KEYWORD
	GO_KEYWORD
	IMPORT_KEYWORD
	FN_KEYWORD
	PERCENT_FN
	SWITCH_KEYWORD
	MATCH_KEYWORD
	DEFAULTCASE_KEYWORD
	RETURN_KEYWORD
	COYIELD_KEYWORD
	YIELD_KEYWORD
	BREAK_KEYWORD
	CONTINUE_KEYWORD
	PRUNE_KEYWORD
	ASSERT_KEYWORD
	SELF_KEYWORD
	MAPPING_KEYWORD
	COMP_KEYWORD
	TREEDATA_KEYWORD
	CONCAT_KEYWORD
	TESTSUITE_KEYWORD
	TESTCASE_KEYWORD
	SYNCHRONIZED_KEYWORD
	ON_KEYWORD
	RECEIVED_KEYWORD
	DO_KEYWORD
	CHUNKED_KEYWORD
	SENDVAL_KEYWORD
	PATTERN_KEYWORD
	PNAMESPACE_KEYWORD
	EXTEND_KEYWORD
	STRUCT_KEYWORD
	NEW_KEYWORD
	TO_KEYWORD
	OTHERPROPS_KEYWORD
	AS_KEYWORD
	AND_KEYWORD
	OR_KEYWORD
	PERCENT_STR
	UNPREFIXED_STR
	PERCENT_SYMBOL
	TILDE
	EXCLAMATION_MARK
	EXCLAMATION_MARK_EQUAL
	DOUBLE_QUESTION_MARK
	PLUS
	PLUS_DOT
	MINUS
	MINUS_DOT
	ASTERISK
	ASTERISK_DOT
	SLASH
	SLASH_DOT
	GREATER_THAN
	GREATER_THAN_DOT
	GREATER_OR_EQUAL
	GREATER_OR_EQUAL_DOT
	LESS_THAN
	LESS_THAN_DOT
	LESS_OR_EQUAL
	LESS_OR_EQUAL_DOT
	SELF_CLOSING_TAG_TERMINATOR
	END_TAG_OPEN_DELIMITER
	OPENING_BRACKET
	CLOSING_BRACKET
	OPENING_CURLY_BRACKET
	CLOSING_CURLY_BRACKET
	OPENING_DICTIONARY_BRACKET
	OPENING_KEYLIST_BRACKET
	OPENING_OBJECT_PATTERN_BRACKET
	OPENING_LIST_PATTERN_BRACKET
	OPENING_QUOTED_STMTS_REGION_BRACE
	UNQUOTED_REGION_OPENING_DELIM
	UNQUOTED_REGION_CLOSING_DELIM
	OPENING_RECORD_BRACKET
	OPENING_TUPLE_BRACKET
	OPENING_PARENTHESIS
	CLOSING_PARENTHESIS
	PATTERN_UNION_OPENING_PIPE
	ARROW
	PIPE
	COMMA
	COLON
	DOUBLE_COLON
	SEMICOLON
	CSS_SELECTOR_PREFIX
	DOT
	TWO_DOTS
	DOT_DOT_LESS_THAN
	THREE_DOTS
	EQUAL
	EQUAL_EQUAL
	PLUS_EQUAL
	MINUS_EQUAL
	MUL_EQUAL
	DIV_EQUAL
	AT_SIGN
	ANTI_SLASH
	DOLLAR
	QUERY_PARAM_QUESTION_MARK
	QUERY_PARAM_SEP
	QUESTION_MARK
	BACKQUOTE
	STR_INTERP_OPENING
	STR_INTERP_CLOSING_BRACKET
	MARKUP_INTERP_OPENING_BRACKET
	MARKUP_INTERP_CLOSING_BRACKET
	NEWLINE

	//WITH VALUE
	UNEXPECTED_CHAR
	INVALID_OPERATOR
	INVALID_INTERP_SLICE
	INVALID_UNQUOTED_REGION_SLICE
	INVALID_URL_LIT
	INVALID_URL_PATT_LIT
	INVALID_HOST_ALIAS
	COMMENT
	INT_LITERAL
	NIL_LITERAL
	FLOAT_LITERAL
	PORT_LITERAL
	BOOLEAN_LITERAL
	DOUBLE_QUOTED_STRING_LITERAL
	UNQUOTED_STRING_LITERAL
	ANNOTATED_REGION_HEADER_TEXT
	REGEX_LITERAL
	RATE_LITERAL
	QUANTITY_LITERAL
	YEAR_LITERAL
	DATE_LITERAL
	DATETIME_LITERAL
	FLAG_LITERAL
	RUNE_LITERAL
	SCHEME_LITERAL
	HOST_LITERAL
	URL_LITERAL
	URL_PATTERN_LITERAL
	HTTP_HOST_PATTERN_LITERAL
	PATTERN_IDENTIFIER_LITERAL
	UNPREFIXED_PATTERN_IDENTIFIER_LITERAL
	PATTERN_NAMESPACE_IDENTIFIER_LITERAL
	UNPREFIXED_PATTERN_NAMESPACE_IDENTIFIER_LITERAL
	IDENTIFIER_LITERAL
	META_IDENTIFIER
	PROP_NAME_LITERAL
	UNAMBIGUOUS_IDENTIFIER_LITERAL
	VARNAME
	ABSOLUTE_PATH_LITERAL
	RELATIVE_PATH_LITERAL
	ABSOLUTE_PATH_PATTERN_LITERAL
	RELATIVE_PATH_PATTERN_LITERAL
	PATH_SLICE
	PATH_PATTERN_SLICE
	STR_TEMPLATE_SLICE
	STR_TEMPLATE_INTERP_TYPE
	BYTE_SLICE_LITERAL
	NAMED_PATH_SEGMENT
	PATTERN_GROUP_NAME
	QUERY_PARAM_KEY_EQUAL
	QUERY_PARAM_SLICE
	OPTION_NAME
	MARKUP_TEXT_SLICE
	MARKUP_ELEMENT_QUANTIFIER
	MARKUP_WILDCARD
	HYPERSCRIPT_CODE_SLICE
	OCCURRENCE_MODIFIER
)

type TokenSubType uint16

const (
	PATH_INTERP_OPENING_BRACE TokenSubType = iota + 1
	PATH_INTERP_CLOSING_BRACE

	HOST_INTERP_OPENING_BRACE
	HOST_INTERP_CLOSING_BRACE

	QUERY_PARAM_INTERP_OPENING_BRACE
	QUERY_PARAM_INTERP_CLOSING_BRACE

	MARKUP_INTERP_OPENING_BRACE
	MARKUP_INTERP_CLOSING_BRACE

	UNDERSCORE_ATTR_SHORTHAND_OPENING_BRACE
	UNDERSCORE_ATTR_SHORTHAND_CLOSING_BRACE

	BLOCK_OPENING_BRACE
	BLOCK_CLOSING_BRACE

	OBJECT_LIKE_OPENING_BRACE
	OBJECT_LIKE_CLOSING_BRACE

	QUOTED_STMTS_CLOSING_BRACE

	UNPREFIXED_PATTERN_UNION_PIPE
	STRING_PATTERN_UNION_PIPE
	CALL_PIPE

	ASSIGN_EQUAL
	FLAG_EQUAL

	MARKUP_TAG_OPENING_BRACKET
	MARKUP_TAG_CLOSING_BRACKET
	MARKUP_ATTR_EQUAL
)

type TokenMeta uint16

const (
	//one bit per meta type

	Callee TokenMeta = 1 << iota
	ParamName
	PropName
	DeclFnName
)

func (m TokenMeta) Strings() ([16]string, int) {
	var strings [16]string
	i := 0

	if m&Callee != 0 {
		strings[i] = "callee"
		i++
	}
	if m&ParamName != 0 {
		strings[i] = "param"
		i++
	}
	if m&PropName != 0 {
		strings[i] = "prop"
		i++
	}
	if m&DeclFnName != 0 {
		strings[i] = READONLY_KEYWORD_STRING
		i++
	}
	return strings, i
}

var tokenStrings = [...]string{
	IF_KEYWORD:                        IF_KEYWORD_STRING,
	ELSE_KEYWORD:                      "else",
	PREINIT_KEYWORD:                   "preinit",
	MANIFEST_KEYWORD:                  "manifest",
	INCLUDABLE_FILE_KEYWORD:           INCLUDABLE_FILE_KEYWORD_STRING,
	DROP_PERMS_KEYWORD:                "drop-perms",
	ASSIGN_KEYWORD:                    "assign",
	READONLY_KEYWORD:                  READONLY_KEYWORD_STRING,
	CONST_KEYWORD:                     "const",
	VAR_KEYWORD:                       "var",
	GLOBALVAR_KEYWORD:                 "globalvar",
	FOR_KEYWORD:                       FOR_KEYWORD_STRING,
	WALK_KEYWORD:                      WALK_KEYWORD_STRING,
	IN_KEYWORD:                        "in",
	GO_KEYWORD:                        "go",
	IMPORT_KEYWORD:                    "import",
	FN_KEYWORD:                        FN_KEYWORD_STRING,
	SWITCH_KEYWORD:                    SWITCH_KEYWORD_STRING,
	MATCH_KEYWORD:                     MATCH_KEYWORD_STRING,
	DEFAULTCASE_KEYWORD:               "defaultcase",
	RETURN_KEYWORD:                    "return",
	COYIELD_KEYWORD:                   COYIELD_KEYWORD_STRING,
	YIELD_KEYWORD:                     YIELD_KEYWORD_STRING,
	BREAK_KEYWORD:                     "break",
	CONTINUE_KEYWORD:                  "continue",
	PRUNE_KEYWORD:                     "prune",
	ASSERT_KEYWORD:                    ASSERT_KEYWORD_STRING,
	SELF_KEYWORD:                      "self",
	MAPPING_KEYWORD:                   "Mapping",
	COMP_KEYWORD:                      "comp",
	TREEDATA_KEYWORD:                  "treedata",
	CONCAT_KEYWORD:                    "concat",
	TESTSUITE_KEYWORD:                 "testsuite",
	TESTCASE_KEYWORD:                  "testcase",
	SYNCHRONIZED_KEYWORD:              "synchronized",
	ON_KEYWORD:                        "on",
	RECEIVED_KEYWORD:                  "received",
	DO_KEYWORD:                        "do",
	CHUNKED_KEYWORD:                   "chunked",
	SENDVAL_KEYWORD:                   "sendval",
	PATTERN_KEYWORD:                   "pattern",
	PNAMESPACE_KEYWORD:                "pnamespace",
	EXTEND_KEYWORD:                    "extend",
	STRUCT_KEYWORD:                    STRUCT_KEYWORD_STRING,
	NEW_KEYWORD:                       NEW_KEYWORD_STRING,
	TO_KEYWORD:                        "to",
	OTHERPROPS_KEYWORD:                OTHERPROPS_KEYWORD_STRING,
	AS_KEYWORD:                        AS_KEYWORD_STRING,
	AND_KEYWORD:                       AND_KEYWORD_STRING,
	OR_KEYWORD:                        OR_KEYWORD_STRING,
	PERCENT_FN:                        "%fn",
	PERCENT_SYMBOL:                    "%",
	TILDE:                             "~",
	EXCLAMATION_MARK:                  "!",
	EXCLAMATION_MARK_EQUAL:            "!=",
	DOUBLE_QUESTION_MARK:              "??",
	PERCENT_STR:                       "%str",
	UNPREFIXED_STR:                    "str",
	NOT_IN_KEYWORD:                    "not-in",
	IS_KEYWORD:                        "is",
	IS_NOT_KEYWORD:                    "is-not",
	KEYOF_KEYWORD:                     "keyof",
	URLOF_KEYWORD:                     "urlof",
	NOT_MATCH_KEYWORD:                 "not-match",
	SUBSTROF_KEYWORD:                  "substrof",
	SELF_CLOSING_TAG_TERMINATOR:       "/>",
	END_TAG_OPEN_DELIMITER:            "</",
	OPENING_BRACKET:                   "[",
	CLOSING_BRACKET:                   "]",
	OPENING_CURLY_BRACKET:             "{",
	CLOSING_CURLY_BRACKET:             "}",
	OPENING_DICTIONARY_BRACKET:        ":{",
	OPENING_KEYLIST_BRACKET:           ".{",
	OPENING_RECORD_BRACKET:            "#{",
	OPENING_TUPLE_BRACKET:             "#[",
	OPENING_OBJECT_PATTERN_BRACKET:    "%{",
	OPENING_LIST_PATTERN_BRACKET:      "%[",
	OPENING_QUOTED_STMTS_REGION_BRACE: "@{",
	UNQUOTED_REGION_OPENING_DELIM:     "<{",
	UNQUOTED_REGION_CLOSING_DELIM:     "}>",
	OPENING_PARENTHESIS:               "(",
	CLOSING_PARENTHESIS:               ")",
	PATTERN_UNION_OPENING_PIPE:        "%|",
	ARROW:                             "=>",
	PIPE:                              "|",
	COMMA:                             ",",
	COLON:                             ":",
	DOUBLE_COLON:                      "::",
	SEMICOLON:                         ";",
	CSS_SELECTOR_PREFIX:               "s!",
	TWO_DOTS:                          "..",
	THREE_DOTS:                        "...",
	PLUS:                              "+",
	PLUS_DOT:                          "+.",
	MINUS:                             "-",
	MINUS_DOT:                         "-.",
	ASTERISK:                          "*",
	ASTERISK_DOT:                      "*.",
	SLASH:                             "/",
	SLASH_DOT:                         "/.",
	DOT:                               ".",
	DOT_DOT_LESS_THAN:                 "..<",
	GREATER_THAN:                      ">",
	GREATER_THAN_DOT:                  ">.",
	GREATER_OR_EQUAL:                  ">=",
	GREATER_OR_EQUAL_DOT:              ">=.",
	LESS_THAN:                         "<",
	LESS_THAN_DOT:                     "<.",
	LESS_OR_EQUAL:                     "<=",
	LESS_OR_EQUAL_DOT:                 "<=.",
	EQUAL:                             "=",
	EQUAL_EQUAL:                       "==",
	PLUS_EQUAL:                        "+=",
	MINUS_EQUAL:                       "-=",
	MUL_EQUAL:                         "*=",
	DIV_EQUAL:                         "/=",
	AT_SIGN:                           "@",
	DOLLAR:                            "$",
	ANTI_SLASH:                        "\\",
	QUERY_PARAM_QUESTION_MARK:         "?",
	QUERY_PARAM_SEP:                   "&",
	QUESTION_MARK:                     "?",
	BACKQUOTE:                         "`",
	STR_INTERP_OPENING:                "${",
	STR_INTERP_CLOSING_BRACKET:        "}",
	NEWLINE:                           "\n",

	// WITH VALUE
	INT_LITERAL:                           "<?>",
	NIL_LITERAL:                           "<?>",
	FLOAT_LITERAL:                         "<?>",
	PORT_LITERAL:                          "<?>",
	BOOLEAN_LITERAL:                       "<?>",
	DOUBLE_QUOTED_STRING_LITERAL:          "<?>",
	UNQUOTED_STRING_LITERAL:               "<?>",
	ANNOTATED_REGION_HEADER_TEXT:          "<?>",
	REGEX_LITERAL:                         "<?>",
	RATE_LITERAL:                          "<?>",
	QUANTITY_LITERAL:                      "<?>",
	DATETIME_LITERAL:                      "<?>",
	FLAG_LITERAL:                          "<?>",
	RUNE_LITERAL:                          "<?>",
	HOST_LITERAL:                          "<?>",
	URL_LITERAL:                           "<?>",
	URL_PATTERN_LITERAL:                   "<?>",
	HTTP_HOST_PATTERN_LITERAL:             "<?>",
	PATTERN_IDENTIFIER_LITERAL:            "<?>",
	UNPREFIXED_PATTERN_IDENTIFIER_LITERAL: "<?>",
	PATTERN_NAMESPACE_IDENTIFIER_LITERAL:  "<?>",
	UNPREFIXED_PATTERN_NAMESPACE_IDENTIFIER_LITERAL: "<?>",
	IDENTIFIER_LITERAL:             "<?>",
	META_IDENTIFIER:                "<?>",
	UNAMBIGUOUS_IDENTIFIER_LITERAL: "<?>",
	ABSOLUTE_PATH_LITERAL:          "<?>",
	RELATIVE_PATH_LITERAL:          "<?>",
	ABSOLUTE_PATH_PATTERN_LITERAL:  "<?>",
	RELATIVE_PATH_PATTERN_LITERAL:  "<?>",
	PATH_SLICE:                     "<?>",
	PATH_PATTERN_SLICE:             "<?>",
	STR_TEMPLATE_SLICE:             "<?>",
	STR_TEMPLATE_INTERP_TYPE:       "<?>",
	BYTE_SLICE_LITERAL:             "<?>",
	MARKUP_TEXT_SLICE:              "<?>",
	MARKUP_ELEMENT_QUANTIFIER:      "<?>",
	MARKUP_WILDCARD:                "<?>",
	HYPERSCRIPT_CODE_SLICE:         "<?>",
	NAMED_PATH_SEGMENT:             "<?>",
}

var tokenTypenames = [...]string{
	IF_KEYWORD:                        "IF_KEYWORD",
	ELSE_KEYWORD:                      "ELSE_KEYWORD",
	PREINIT_KEYWORD:                   "PREINIT_KEYWORD",
	MANIFEST_KEYWORD:                  "inoxconsts.MANIFEST_KEYWORD",
	INCLUDABLE_FILE_KEYWORD:           "INCLUDABLE_CHUNK_KEYWORD",
	DROP_PERMS_KEYWORD:                "DROP_PERMS_KEYWORD",
	ASSIGN_KEYWORD:                    "ASSIGN_KEYWORD",
	READONLY_KEYWORD:                  "READONLY_KEYWORD",
	CONST_KEYWORD:                     "CONST_KEYWORD",
	VAR_KEYWORD:                       "VAR_KEYWORD",
	GLOBALVAR_KEYWORD:                 "GLOBALVAR_KEYWORD",
	FOR_KEYWORD:                       "FOR_KEYWORD",
	WALK_KEYWORD:                      "WALK_KEYWORD",
	IN_KEYWORD:                        "IN_KEYWORD",
	NOT_IN_KEYWORD:                    "NOT_IN_KEYWORD",
	IS_KEYWORD:                        "IS_KEYWORD",
	IS_NOT_KEYWORD:                    "IS_NOT_KEYWORD",
	KEYOF_KEYWORD:                     "KEYOF_KEYWORD",
	URLOF_KEYWORD:                     "URLOF_KEYWORD",
	NOT_MATCH_KEYWORD:                 "NOT_MATCH_KEYWORD",
	SUBSTROF_KEYWORD:                  "SUBSTROF_KEYWORD",
	GO_KEYWORD:                        "GO_KEYWORD",
	IMPORT_KEYWORD:                    "IMPORT_KEYWORD",
	FN_KEYWORD:                        "FN_KEYWORD",
	PERCENT_FN:                        "PERCENT_FN",
	SWITCH_KEYWORD:                    "SWITCH_KEYWORD",
	MATCH_KEYWORD:                     "MATCH_KEYWORD",
	DEFAULTCASE_KEYWORD:               "DEFAULTCASE_KEYWORD",
	RETURN_KEYWORD:                    "RETURN_KEYWORD",
	COYIELD_KEYWORD:                   "YIELD_KEYWORD",
	BREAK_KEYWORD:                     "BREAK_KEYWORD",
	CONTINUE_KEYWORD:                  "CONTINUE_KEYWORD",
	PRUNE_KEYWORD:                     "PRUNE_KEYWORD",
	ASSERT_KEYWORD:                    "ASSERT_KEYWORD",
	SELF_KEYWORD:                      "SELF_KEYWORD",
	MAPPING_KEYWORD:                   "MAPPING_KEYWORD",
	COMP_KEYWORD:                      "COMP_KEYWORD",
	TREEDATA_KEYWORD:                  "TREEDATA_KEYWORD",
	CONCAT_KEYWORD:                    "CONCAT_KEYWORD",
	TESTSUITE_KEYWORD:                 "TESTSUITE_KEYWORD",
	TESTCASE_KEYWORD:                  "TESTCASE_KEYWORD",
	SYNCHRONIZED_KEYWORD:              "SYNCHRONIZED_KEYWORD",
	ON_KEYWORD:                        "ON_KEYWORD",
	RECEIVED_KEYWORD:                  "RECEIVED_KEYWORD",
	DO_KEYWORD:                        "DO_KEYWORD",
	CHUNKED_KEYWORD:                   "CHUNKED_KEYWORD",
	SENDVAL_KEYWORD:                   "SENDVAL_KEYWORD",
	PATTERN_KEYWORD:                   "PATTERN_KEYWORD",
	PNAMESPACE_KEYWORD:                "PNAMESPACE_KEYWORD",
	EXTEND_KEYWORD:                    "EXTEND_KEYWORD",
	STRUCT_KEYWORD:                    "STRUCT_KEYWORD",
	NEW_KEYWORD:                       "NEW_KEYWORD",
	TO_KEYWORD:                        "TO_KEYWORD",
	OTHERPROPS_KEYWORD:                "OTHERPROPS_KEYWORD",
	AS_KEYWORD:                        "AS_KEYWORD",
	AND_KEYWORD:                       "AND_KEYWORD",
	OR_KEYWORD:                        "OR_KEYWORD",
	PERCENT_STR:                       "PERCENT_STR",
	UNPREFIXED_STR:                    "UNPREFIXED_STR",
	PERCENT_SYMBOL:                    "PERCENT_SYMBOL",
	TILDE:                             "TILDE",
	EXCLAMATION_MARK:                  "EXCLAMATION_MARK",
	EXCLAMATION_MARK_EQUAL:            "EXCLAMATION_MARK_EQUAL",
	DOUBLE_QUESTION_MARK:              "DOUBLE_QUESTION_MARK",
	PLUS:                              "PLUS",
	PLUS_DOT:                          "PLUS_DOT",
	MINUS:                             "MINUS",
	MINUS_DOT:                         "MINUS_DOT",
	ASTERISK:                          "ASTERISK",
	ASTERISK_DOT:                      "ASTERISK_DOT",
	SLASH:                             "SLASH",
	SLASH_DOT:                         "SLASH_DOT",
	GREATER_THAN:                      "GREATER_THAN",
	GREATER_THAN_DOT:                  "GREATER_THAN_DOT",
	GREATER_OR_EQUAL:                  "GREATER_OR_EQUAL",
	GREATER_OR_EQUAL_DOT:              "GREATER_OR_EQUAL_DOT",
	LESS_THAN:                         "LESS_THAN",
	LESS_THAN_DOT:                     "LESS_THAN_DOT",
	LESS_OR_EQUAL:                     "LESS_OR_EQUAL",
	LESS_OR_EQUAL_DOT:                 "LESS_OR_EQUAL_DOT",
	OPENING_BRACKET:                   "OPENING_BRACKET",
	SELF_CLOSING_TAG_TERMINATOR:       "SELF_CLOSING_TAG_TERMINATOR",
	END_TAG_OPEN_DELIMITER:            "END_TAG_OPEN_DELIMITER",
	CLOSING_BRACKET:                   "CLOSING_BRACKET",
	OPENING_CURLY_BRACKET:             "OPENING_CURLY_BRACKET",
	CLOSING_CURLY_BRACKET:             "CLOSING_CURLY_BRACKET",
	OPENING_DICTIONARY_BRACKET:        "OPENING_DICTIONARY_BRACKET",
	OPENING_OBJECT_PATTERN_BRACKET:    "OPENING_OBJECT_PATTERN_BRACKET",
	OPENING_LIST_PATTERN_BRACKET:      "OPENING_LIST_PATTERN_BRACKET",
	OPENING_QUOTED_STMTS_REGION_BRACE: "OPENING_QUOTED_STMTS_REGION_BRACE",
	UNQUOTED_REGION_OPENING_DELIM:     "UNQUOTED_REGION_OPENING_DELIM",
	UNQUOTED_REGION_CLOSING_DELIM:     "UNQUOTED_REGION_CLOSING_DELIM",
	OPENING_RECORD_BRACKET:            "OPENING_RECORD_BRACKET",
	OPENING_TUPLE_BRACKET:             "OPENING_TUPLE_BRACKET",
	OPENING_PARENTHESIS:               "OPENING_PARENTHESIS",
	CLOSING_PARENTHESIS:               "CLOSING_PARENTHESIS",
	PATTERN_UNION_OPENING_PIPE:        "PATTERN_UNION_OPENING_PIPE",
	ARROW:                             "ARROW",
	PIPE:                              "PIPE",
	COMMA:                             "COMMA",
	COLON:                             "COLON",
	DOUBLE_COLON:                      "DOUBLE_COLON",
	SEMICOLON:                         "SEMICOLON",
	CSS_SELECTOR_PREFIX:               "CSS_SELECTOR_PREFIX",
	DOT:                               "DOT",
	TWO_DOTS:                          "TWO_DOTS",
	DOT_DOT_LESS_THAN:                 "DOT_DOT_LESS_THAN",
	THREE_DOTS:                        "THREE_DOTS",
	EQUAL:                             "EQUAL",
	EQUAL_EQUAL:                       "EQUAL_EQUAL",
	PLUS_EQUAL:                        "PLUS_EQUAL",
	MINUS_EQUAL:                       "MINUS_EQUAL",
	MUL_EQUAL:                         "MUL_EQUAL",
	DIV_EQUAL:                         "DIV_EQUAL",
	AT_SIGN:                           "AT_SIGN",
	ANTI_SLASH:                        "ANTI_SLASH",
	DOLLAR:                            "DOLLAR",
	QUERY_PARAM_QUESTION_MARK:         "QUERY_PARAM_QUESTION_MARK",
	QUESTION_MARK:                     "QUESTION_MARK",
	BACKQUOTE:                         "BACKQUOTE",
	STR_INTERP_OPENING:                "STR_INTERP_OPENING_BRACKETS",
	STR_INTERP_CLOSING_BRACKET:        "STR_INTERP_CLOSING_BRACKETS",
	NEWLINE:                           "NEWLINE",

	//WITH: "WITH", VALUE: "VALUE",
	UNEXPECTED_CHAR:                       "UNEXPECTED_CHAR",
	INVALID_OPERATOR:                      "INVALID_OPERATOR",
	INVALID_INTERP_SLICE:                  "INVALID_INTERP_SLICE",
	INVALID_UNQUOTED_REGION_SLICE:         "INVALID_UNQUOTED_REGION_SLICE",
	INVALID_URL_LIT:                       "INVALID_URL_LIT",
	INVALID_URL_PATT_LIT:                  "INVALID_URL_PATT_LIT",
	INVALID_HOST_ALIAS:                    "INVALID_HOST_ALIAS",
	COMMENT:                               "COMMENT",
	INT_LITERAL:                           "INT_LITERAL",
	NIL_LITERAL:                           "NIL_LITERAL",
	FLOAT_LITERAL:                         "FLOAT_LITERAL",
	PORT_LITERAL:                          "PORT_LITERAL",
	BOOLEAN_LITERAL:                       "BOOLEAN_LITERAL",
	DOUBLE_QUOTED_STRING_LITERAL:          "DOUBLE_QUOTED_STRING_LITERAL",
	UNQUOTED_STRING_LITERAL:               "UNQUOTED_STRING_LITERAL",
	ANNOTATED_REGION_HEADER_TEXT:          "ANNOTATED_REGION_HEADER_TEXT",
	REGEX_LITERAL:                         "REGEX_LITERAL",
	RATE_LITERAL:                          "RATE_LITERAL",
	QUANTITY_LITERAL:                      "QUANTITY_LITERAL",
	DATETIME_LITERAL:                      "DATETIME_LITERAL",
	FLAG_LITERAL:                          "FLAG_LITERAL",
	RUNE_LITERAL:                          "RUNE_LITERAL",
	SCHEME_LITERAL:                        "SCHEME_LITERAL",
	HOST_LITERAL:                          "HOST_LITERAL",
	URL_LITERAL:                           "URL_LITERAL",
	URL_PATTERN_LITERAL:                   "URL_PATTERN_LITERAL",
	HTTP_HOST_PATTERN_LITERAL:             "HTTP_HOST_PATTERN_LITERAL",
	PATTERN_IDENTIFIER_LITERAL:            "PATTERN_IDENTIFIER_LITERAL",
	UNPREFIXED_PATTERN_IDENTIFIER_LITERAL: "UNPREFIXED_PATTERN_IDENTIFIER_LITERAL",
	PATTERN_NAMESPACE_IDENTIFIER_LITERAL:  "PATTERN_NAMESPACE_IDENTIFIER_LITERAL",
	UNPREFIXED_PATTERN_NAMESPACE_IDENTIFIER_LITERAL: "UNPREFIXED_PATTERN_NAMESPACE_IDENTIFIER_LITERAL",
	IDENTIFIER_LITERAL:             "IDENTIFIER_LITERAL",
	META_IDENTIFIER:                "META_IDENTIFIER",
	PROP_NAME_LITERAL:              "PROP_NAME_LITERAL",
	UNAMBIGUOUS_IDENTIFIER_LITERAL: "UNAMBIGUOUS_IDENTIFIER_LITERAL",
	VARNAME:                        "VARNAME",
	ABSOLUTE_PATH_LITERAL:          "ABSOLUTE_PATH_LITERAL",
	RELATIVE_PATH_LITERAL:          "RELATIVE_PATH_LITERAL",
	ABSOLUTE_PATH_PATTERN_LITERAL:  "ABSOLUTE_PATH_PATTERN_LITERAL",
	RELATIVE_PATH_PATTERN_LITERAL:  "RELATIVE_PATH_PATTERN_LITERAL",
	PATH_SLICE:                     "PATH_SLICE",
	PATH_PATTERN_SLICE:             "PATH_PATTERN_SLICE",
	STR_TEMPLATE_SLICE:             "STR_TEMPLATE_SLICE",
	STR_TEMPLATE_INTERP_TYPE:       "STR_TEMPLATE_INTERP_TYPE",
	BYTE_SLICE_LITERAL:             "BYTE_SLICE_LITERAL",
	NAMED_PATH_SEGMENT:             "NAMED_PATH_SEGMENT",
	PATTERN_GROUP_NAME:             "PATTERN_GROUP_NAME",
	QUERY_PARAM_KEY_EQUAL:          "QUERY_PARAM_KEY_EQUAL",
	QUERY_PARAM_SEP:                "QUERY_PARAM_SEP",
	QUERY_PARAM_SLICE:              "QUERY_PARAM_SLICE",
	OPTION_NAME:                    "OPTION_NAME",
	MARKUP_TEXT_SLICE:              "MARKUP_TEXT_SLICE",
	MARKUP_ELEMENT_QUANTIFIER:      "MARKUP_ELEMENT_QUANTIFIER",
	MARKUP_WILDCARD:                "MARKUP_WILDCARD",
	HYPERSCRIPT_CODE_SLICE:         "HYPERSCRIPT_CODE_SLICE",
	OCCURRENCE_MODIFIER:            "OCCURRENCE_MODIFIER",
}

func (t TokenType) String() string {
	return tokenTypenames[t]
}

func (t Token) Token() string {
	return tokenStrings[t.Type]
}

func (t Token) String() string {
	return fmt.Sprintf("\"%s\"(%d-%d)", t.Token(), t.Span.Start, t.Span.End)
}

// GetTokens retrieves the tokens located in a node' span, the returned slice should NOT be modified.
// Note that this function is not efficient.
func GetTokens(node Node, chunk *Chunk, addMeta bool) []Token {
	if node == nil {
		return nil
	}

	ptr := reflect.ValueOf(node).Pointer()
	if ptr == 0 {
		return nil
	}

	tokenCacheLock.Lock()
	if tokens, ok := tokenCache[ptr]; ok {
		tokenCacheLock.Unlock()
		return slices.Clone(tokens)
	}

	chunkTokens := chunk.Tokens

	//we unlock when the function is finished because we modify the tokenCache at the end
	defer tokenCacheLock.Unlock()

	tokens := make([]Token, 0)

	//first we add the valueless tokens

	nodeBase := node.Base()
	startTokenIndex, _ := slices.BinarySearchFunc(chunkTokens, nodeBase.Span, func(t Token, ns NodeSpan) int {
		return int(t.Span.Start) - int(ns.Start)
	})

	for _, token := range chunkTokens[startTokenIndex:] {
		if token.Span.Start >= nodeBase.Span.End {
			break
		}
		tokens = append(tokens, token)
	}

	//get tokens of all nodes

	Walk(node, func(node, parentNode, _ Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
		//TODO: avoid allocating strings if original code string is available

		switch n := node.(type) {
		case *MemberExpression:
			i := n.Left.Base().Span.End

			tokens = append(tokens, Token{
				Type: DOT,
				Span: NodeSpan{i, i + 1},
			})

			if n.Optional {
				tokens = append(tokens, Token{
					Type: QUESTION_MARK,
					Span: NodeSpan{i + 1, i + 2},
				})
			}

			return ContinueTraversal, nil
		case *IdentifierMemberExpression:
			for _, ident := range n.PropertyNames {
				i := ident.Base().Span.Start - 1

				tokens = append(tokens, Token{
					Type: DOT,
					Span: NodeSpan{i, i + 1},
				})
			}
			return ContinueTraversal, nil
		case *URLExpression:

			if _, ok := n.HostPart.(*IdentifierLiteral); ok {
				tokens = append(tokens, Token{
					Type: AT_SIGN,
					Span: NodeSpan{n.Span.Start, n.Span.Start + 1},
				})
			}

			for i, r := range n.Raw {
				switch r {
				case '?':
					tokens = append(tokens, Token{
						Type: QUERY_PARAM_QUESTION_MARK,
						Span: NodeSpan{n.Span.Start + int32(i), n.Span.Start + int32(i) + 1},
					})
				case '&':
					tokens = append(tokens, Token{
						Type: QUERY_PARAM_SEP,
						Span: NodeSpan{n.Span.Start + int32(i), n.Span.Start + int32(i) + 1},
					})
				}
			}

			return ContinueTraversal, nil
		case *URLQueryParameter:

			tokens = append(tokens, Token{
				Type: QUERY_PARAM_KEY_EQUAL,
				Span: NodeSpan{n.Span.Start, n.Span.Start + int32(utf8.RuneCountInString(n.Name)) + 1},
				Raw:  n.Name + "=",
			})

			return ContinueTraversal, nil
		case *OptionalPatternExpression:
			tokens = append(tokens, Token{
				Type: QUESTION_MARK,
				Span: NodeSpan{n.Span.End - 1, n.Span.End},
			})
			return ContinueTraversal, nil

		case *BooleanConversionExpression:
			tokens = append(tokens, Token{
				Type: QUESTION_MARK,
				Span: NodeSpan{n.Span.End - 1, n.Span.End},
			})

		case *OptionExpression:
			namePart := "-"
			if !n.SingleDash {
				namePart = "--"
			}

			namePart += n.Name
			nameEnd := n.Span.Start + int32(utf8.RuneCountInString(namePart))

			tokens = append(tokens, Token{
				Type: OPTION_NAME,
				Span: NodeSpan{n.Span.Start, nameEnd},
				Raw:  namePart,
			})
			tokens = append(tokens, Token{
				Type:    EQUAL,
				SubType: FLAG_EQUAL,
				Span:    NodeSpan{nameEnd, nameEnd + 1},
			})
		case *OptionPatternLiteral:
			namePart := "%-"
			if n.Unprefixed {
				namePart = "-"
				if !n.SingleDash {
					namePart = "--"
				}
			} else if !n.SingleDash {
				namePart = "%--"
			}

			namePart += n.Name
			nameEnd := n.Span.Start + int32(utf8.RuneCountInString(namePart))

			tokens = append(tokens, Token{
				Type: OPTION_NAME,
				Span: NodeSpan{n.Span.Start, nameEnd},
				Raw:  namePart,
			})
			tokens = append(tokens, Token{
				Type:    EQUAL,
				SubType: FLAG_EQUAL,
				Span:    NodeSpan{nameEnd, nameEnd + 1},
			})
		case *MarkupElement:
			if n.RawElementContent != "" {
				start := n.Opening.Span.End
				end := n.Span.End
				if n.Closing != nil {
					end = n.Closing.Span.Start
				}

				tokens = append(tokens, Token{
					Type: MARKUP_TEXT_SLICE,
					Span: NodeSpan{start, end},
					Raw:  n.RawElementContent,
				})
			}
		case *MarkupPatternElement:
			if n.RawElementContent != "" {
				start := n.Opening.Span.End
				end := n.Span.End
				if n.Closing != nil {
					end = n.Closing.Span.Start
				}

				tokens = append(tokens, Token{
					Type: MARKUP_TEXT_SLICE,
					Span: NodeSpan{start, end},
					Raw:  n.RawElementContent,
				})
			}
		case *MarkupPatternOpeningTag:
			if n.Quantifier != 0 {
				var raw string
				switch n.Quantifier {
				case OptionalMarkupElement:
					raw = OPTIONAL_MARKUP_ELEMENT_QUANTIFIER
				case ZeroOrMoreMarkupElements:
					raw = ZERO_OR_MORE_MARKUP_ELEMENT_QUANTIFIER
				case OneOrMoreMarkupElements:
					raw = ONE_OR_MORE_MARKUP_ELEMENT_QUANTIFIER
				default:
					panic(ErrUnreachable)
				}

				nameSpan := n.Name.Base().Span

				tokens = append(tokens, Token{
					Type: MARKUP_ELEMENT_QUANTIFIER,
					Raw:  raw,
					Span: NodeSpan{nameSpan.End, nameSpan.End + 1},
				})
			}
		case *ObjectDestructurationProperty:
			if n.Nillable {
				tokens = append(tokens, Token{
					Type: QUESTION_MARK,
					Span: NodeSpan{n.PropertyName.Span.End, n.PropertyName.Span.End + 1},
				})
			}
		}

		var tokenType TokenType
		var tokenMeta TokenMeta
		var raw = ""
		literalSpan := node.Base().Span

		// literals
		switch n := node.(type) {
		case *IntLiteral:
			tokenType = INT_LITERAL
			raw = n.Raw
		case *SelfExpression:
			tokenType = SELF_KEYWORD
			raw = "self"
		case *NilLiteral:
			tokenType = NIL_LITERAL
			raw = "nil"
		case *FloatLiteral:
			tokenType = FLOAT_LITERAL
			raw = n.Raw
		case *PortLiteral:
			tokenType = PORT_LITERAL
			raw = n.Raw
		case *BooleanLiteral:
			tokenType = BOOLEAN_LITERAL
			if n.Value {
				raw = "true"
			} else {
				raw = "false"
			}
		case *DoubleQuotedStringLiteral:
			tokenType = DOUBLE_QUOTED_STRING_LITERAL
			raw = n.Raw
		case *UnquotedStringLiteral:
			tokenType = UNQUOTED_STRING_LITERAL
			raw = n.Raw
		case *MultilineStringLiteral:
			tokenType = STR_TEMPLATE_SLICE
			raw = n.RawWithoutQuotes()

			literalSpan.Start = n.NodeBase.Span.Start + 1

			if n.IsUnterminated {
				literalSpan.End = n.NodeBase.Span.End
			} else {
				literalSpan.End = n.NodeBase.Span.End - 1
			}
		case *AnnotatedRegionHeaderText:
			tokenType = ANNOTATED_REGION_HEADER_TEXT
			raw = n.Raw
		case *RegularExpressionLiteral:
			tokenType = REGEX_LITERAL
			raw = n.Raw
		case *RateLiteral:
			tokenType = RATE_LITERAL
			raw = n.Raw
		case *QuantityLiteral:
			tokenType = QUANTITY_LITERAL
			raw = n.Raw
		case *YearLiteral:
			tokenType = YEAR_LITERAL
			raw = n.Raw
		case *DateLiteral:
			tokenType = DATE_LITERAL
			raw = n.Raw
		case *DateTimeLiteral:
			tokenType = DATETIME_LITERAL
			raw = n.Raw
		case *FlagLiteral:
			tokenType = FLAG_LITERAL
			raw = n.Raw
		case *RuneLiteral:
			tokenType = RUNE_LITERAL
			raw = "'" + string(n.Value) + "'"
		case *SchemeLiteral:
			tokenType = SCHEME_LITERAL
			raw = n.Name + "://"
		case *HostLiteral:
			tokenType = HOST_LITERAL
			raw = n.Value
		case *URLLiteral:
			tokenType = URL_LITERAL
			raw = n.Value
		case *URLPatternLiteral:
			tokenType = URL_PATTERN_LITERAL
			raw = n.Raw
		case *HostPatternLiteral:
			tokenType = HTTP_HOST_PATTERN_LITERAL
			raw = n.Raw
		case *PatternIdentifierLiteral:
			if n.Unprefixed {
				tokenType = UNPREFIXED_PATTERN_IDENTIFIER_LITERAL
				raw = n.Name
			} else {
				tokenType = PATTERN_IDENTIFIER_LITERAL
				raw = "%" + n.Name
			}
		case *PatternNamespaceIdentifierLiteral:
			if n.Unprefixed {
				tokenType = UNPREFIXED_PATTERN_NAMESPACE_IDENTIFIER_LITERAL
				raw = n.Name + "."
			} else {
				tokenType = PATTERN_NAMESPACE_IDENTIFIER_LITERAL
				raw = "%" + n.Name + "."
			}
		case *IdentifierLiteral:
			tokenType = IDENTIFIER_LITERAL
			raw = n.Name
			if addMeta {
				switch p := parentNode.(type) {
				case *MemberExpression:
					if n == p.PropertyName {
						tokenMeta |= PropName
					}
					if len(ancestorChain) >= 2 {
						callExpr, ok := ancestorChain[len(ancestorChain)-2].(*CallExpression)
						if ok && p == callExpr.Callee {
							tokenMeta |= Callee
						}
					}
				case *IdentifierMemberExpression:
					for _, propName := range p.PropertyNames {
						if n == propName {
							tokenMeta |= PropName
							break
						}
					}

					if len(ancestorChain) >= 2 && len(p.PropertyNames) > 0 && n == p.PropertyNames[len(p.PropertyNames)-1] {
						callExpr, ok := ancestorChain[len(ancestorChain)-2].(*CallExpression)
						if ok && p == callExpr.Callee {
							tokenMeta |= Callee
						}
					}

				case *CallExpression:
					if n == p.Callee {
						tokenMeta |= Callee
					}
				case *FunctionParameter:
					if n == p.Var {
						tokenMeta |= ParamName
					}
				case *FunctionDeclaration:
					if n == p.Name {
						tokenMeta |= DeclFnName
					}
				}
			}
		case *MetaIdentifier:
			tokenType = META_IDENTIFIER
			raw = "@" + n.Name
		case *UnambiguousIdentifierLiteral:
			tokenType = UNAMBIGUOUS_IDENTIFIER_LITERAL
			raw = "#" + n.Name
		case *Variable:
			tokenType = VARNAME
			raw = "$" + n.Name
		case *PropertyNameLiteral:
			tokenType = PROP_NAME_LITERAL
			raw = "." + n.Name
		case *AbsolutePathLiteral:
			tokenType = ABSOLUTE_PATH_LITERAL
			raw = n.Raw
		case *RelativePathLiteral:
			tokenType = RELATIVE_PATH_LITERAL
			raw = n.Raw
		case *AbsolutePathPatternLiteral:
			tokenType = ABSOLUTE_PATH_PATTERN_LITERAL
			raw = n.Raw
		case *RelativePathPatternLiteral:
			tokenType = RELATIVE_PATH_PATTERN_LITERAL
			raw = n.Raw
		case *PathSlice:
			tokenType = PATH_SLICE
			raw = n.Value
		case *PathPatternSlice:
			tokenType = PATH_PATTERN_SLICE
			raw = n.Value
		case *NamedPathSegment:
			tokenType = NAMED_PATH_SEGMENT
			raw = ":" + n.Name
		case *StringTemplateSlice:
			tokenType = STR_TEMPLATE_SLICE
			raw = n.Raw
		case *ByteSliceLiteral:
			tokenType = BYTE_SLICE_LITERAL
			raw = n.Raw
		case *PatternGroupName:
			tokenType = PATTERN_GROUP_NAME
			raw = n.Name
		case *URLQueryParameterValueSlice:
			tokenType = QUERY_PARAM_SLICE
			raw = n.Value
		case *InvalidURL:
			tokenType = INVALID_URL_LIT
			raw = n.Value
		case *InvalidURLPattern:
			tokenType = INVALID_URL_PATT_LIT
			raw = n.Value
		case *InvalidAliasRelatedNode:
			tokenType = INVALID_HOST_ALIAS
			raw = n.Raw
		case *Comment:
			tokenType = COMMENT
			raw = n.Raw
		case *MarkupText:
			tokenType = MARKUP_TEXT_SLICE
			raw = n.Raw
		case *HyperscriptAttributeShorthand:
			tokenType = HYPERSCRIPT_CODE_SLICE
			raw = n.Value
			literalSpan.Start = n.NodeBase.Span.Start + 1
			if n.IsUnterminated {
				literalSpan.End = n.NodeBase.Span.End
			} else {
				literalSpan.End = n.NodeBase.Span.End - 1
			}
		case *MarkupPatternWildcard:
			tokenType = HYPERSCRIPT_CODE_SLICE
			raw = MARKUP_STAR_WILDCARD
		}

		if tokenType > 0 {
			tokens = append(tokens, Token{
				Type: tokenType,
				Meta: tokenMeta,
				Span: literalSpan,
				Raw:  raw,
			})
		}
		return ContinueTraversal, nil
	}, nil)

	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].Span.Start < tokens[j].Span.Start
	})

	var uniqueTokens []Token

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token.Span.End == token.Span.Start { //ignore empty tokens
			continue
		}

		if token.Type <= LAST_TOKEN_TYPE_WITHOUT_VALUE {
			token.Raw = tokenStrings[token.Type]
		}

		if i != 0 && token.Span == tokens[i-1].Span {
			continue
		}
		uniqueTokens = append(uniqueTokens, token)
	}

	if len(tokens) >= MIN_TOKEN_CACHING_COUNT {
		tokenCache[ptr] = slices.Clone(uniqueTokens)

		// we remove the cache entry when the node is no longer reachable
		runtime.SetFinalizer(node, func(n Node) {
			tokenCacheLock.Lock()
			delete(tokenCache, reflect.ValueOf(n).Pointer())
			tokenCacheLock.Unlock()
		})
	}

	return uniqueTokens
}

func GetFirstToken(node Node, chunk *Chunk) Token {
	tokens := GetTokens(node, chunk, false)
	return tokens[0]
}

func GetFirstTokenString(node Node, chunk *Chunk) string {
	tokens := GetTokens(node, chunk, false)
	return tokens[0].Raw
}

func GetTokenAtPosition(pos int, node Node, chunk *Chunk) (Token, bool) {
	tokens := GetTokens(node, chunk, false)
	index, ok := slices.BinarySearchFunc(tokens, Token{Type: 0, Span: NodeSpan{int32(pos), int32(pos + 1)}}, func(a, b Token) int {
		if a.Type > 0 && b.Type > 0 {
			return int(a.Span.Start - b.Span.Start)
		}
		if a.Type == 0 {
			if int(b.Span.End) <= pos {
				return 1
			}
			if int(b.Span.Start) >= pos+1 {
				return -1
			}
			return 0
		} else {
			if int(a.Span.End) <= pos {
				return -1
			}
			if int(a.Span.Start) >= pos+1 {
				return 1
			}
			return 0
		}
	})
	if !ok {
		return Token{}, false
	}

	return tokens[index], true
}
