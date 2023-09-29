package parse

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"unicode/utf8"

	"slices"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	MIN_TOKEN_CACHING_COUNT = 2
)

var (
	tokenCache     = map[uintptr][]Token{}
	tokenCacheLock sync.Mutex
)

type Token struct {
	Type     TokenType
	Meta     TokenMeta
	UserMeta uint32
	Span     NodeSpan
	Raw      string
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
	//WITH NO ASSOCIATED VALUE
	IF_KEYWORD TokenType = iota + 1
	ELSE_KEYWORD
	PREINIT_KEYWORD
	MANIFEST_KEYWORD
	INCLUDABLE_CHUNK_KEYWORD
	DROP_PERMS_KEYWORD
	ASSIGN_KEYWORD
	READONLY_KEYWORD
	CONST_KEYWORD
	VAR_KEYWORD
	FOR_KEYWORD
	WALK_KEYWORD
	IN_KEYWORD
	GO_KEYWORD
	IMPORT_KEYWORD
	FN_KEYWORD
	PERCENT_FN
	SWITCH_KEYWORD
	MATCH_KEYWORD
	DEFAULTCASE_KEYWORD
	RETURN_KEYWORD
	YIELD_KEYWORD
	BREAK_KEYWORD
	CONTINUE_KEYWORD
	PRUNE_KEYWORD
	ASSERT_KEYWORD
	SELF_KEYWORD
	MAPPING_KEYWORD
	COMP_KEYWORD
	UDATA_KEYWORD
	CONCAT_KEYWORD
	TESTSUITE_KEYWORD
	TESTCASE_KEYWORD
	SYNCHRONIZED_KEYWORD
	LIFETIMEJOB_KEYWORD
	ON_KEYWORD
	RECEIVED_KEYWORD
	DO_KEYWORD
	CHUNKED_KEYWORD
	SENDVAL_KEYWORD
	PATTERN_KEYWORD
	PNAMESPACE_KEYWORD
	EXTEND_KEYWORD
	TO_KEYWORD
	AND_KEYWORD
	OR_KEYWORD
	PERCENT_STR
	IN
	NOT_IN
	IS
	IS_NOT
	KEYOF
	NOT_MATCH
	SUBSTROF
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
	OPENING_RECORD_BRACKET
	OPENING_TUPLE_BRACKET
	OPENING_PARENTHESIS
	CLOSING_PARENTHESIS
	SINGLE_INTERP_OPENING_BRACE
	SINGLE_INTERP_CLOSING_BRACE
	PATTERN_UNION_OPENING_PIPE
	PATTERN_UNION_PIPE
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
	DOT_LESS_THAN
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
	STR_INTERP_OPENING_BRACKETS
	STR_INTERP_CLOSING_BRACKETS
	XML_INTERP_OPENING_BRACKET
	XML_INTERP_CLOSING_BRACKET
	NEWLINE

	//WITH VALUE
	UNEXPECTED_CHAR
	INVALID_OPERATOR
	INVALID_INTERP_SLICE
	INVALID_URL_LIT
	INVALID_HOST_ALIAS
	COMMENT
	INT_LITERAL
	NIL_LITERAL
	FLOAT_LITERAL
	PORT_LITERAL
	BOOLEAN_LITERAL
	QUOTED_STRING_LITERAL
	UNQUOTED_STRING_LITERAL
	MULTILINE_STRING_LITERAL
	REGEX_LITERAL
	RATE_LITERAL
	QUANTITY_LITERAL
	DATE_LITERAL
	FLAG_LITERAL
	RUNE_LITERAL
	AT_HOST_LITERAL
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
	PROP_NAME_LITERAL
	UNAMBIGUOUS_IDENTIFIER_LITERAL
	LOCAL_VARNAME
	GLOBAL_VARNAME
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
	XML_TEXT_SLICE
	OCCURRENCE_MODIFIER
)

type TokenMeta uint16

const (
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
		strings[i] = "fn"
		i++
	}
	return strings, i
}

var tokenStrings = [...]string{
	IF_KEYWORD:                     "if",
	ELSE_KEYWORD:                   "else",
	PREINIT_KEYWORD:                "preinit",
	MANIFEST_KEYWORD:               "manifest",
	INCLUDABLE_CHUNK_KEYWORD:       "includable-chunk",
	DROP_PERMS_KEYWORD:             "drop-perms",
	ASSIGN_KEYWORD:                 "assign",
	READONLY_KEYWORD:               "readonly",
	CONST_KEYWORD:                  "const",
	VAR_KEYWORD:                    "var",
	FOR_KEYWORD:                    "for",
	WALK_KEYWORD:                   "walk",
	IN_KEYWORD:                     "in",
	GO_KEYWORD:                     "go",
	IMPORT_KEYWORD:                 "import",
	FN_KEYWORD:                     "fn",
	SWITCH_KEYWORD:                 "switch",
	MATCH_KEYWORD:                  "match",
	DEFAULTCASE_KEYWORD:            "defaultcase",
	RETURN_KEYWORD:                 "return",
	YIELD_KEYWORD:                  "yield",
	BREAK_KEYWORD:                  "break",
	CONTINUE_KEYWORD:               "continue",
	PRUNE_KEYWORD:                  "prune",
	ASSERT_KEYWORD:                 "assert",
	SELF_KEYWORD:                   "self",
	MAPPING_KEYWORD:                "Mapping",
	COMP_KEYWORD:                   "comp",
	UDATA_KEYWORD:                  "udata",
	CONCAT_KEYWORD:                 "concat",
	TESTSUITE_KEYWORD:              "testsuite",
	TESTCASE_KEYWORD:               "testcase",
	SYNCHRONIZED_KEYWORD:           "synchronized",
	LIFETIMEJOB_KEYWORD:            "lifetimejob",
	ON_KEYWORD:                     "on",
	RECEIVED_KEYWORD:               "received",
	DO_KEYWORD:                     "do",
	CHUNKED_KEYWORD:                "chunked",
	SENDVAL_KEYWORD:                "sendval",
	PATTERN_KEYWORD:                "pattern",
	PNAMESPACE_KEYWORD:             "pnamespace",
	EXTEND_KEYWORD:                 "extend",
	TO_KEYWORD:                     "to",
	AND_KEYWORD:                    "and",
	OR_KEYWORD:                     "or",
	PERCENT_FN:                     "%fn",
	PERCENT_SYMBOL:                 "%",
	TILDE:                          "~",
	EXCLAMATION_MARK:               "!",
	EXCLAMATION_MARK_EQUAL:         "!=",
	DOUBLE_QUESTION_MARK:           "??",
	PERCENT_STR:                    "%str",
	IN:                             "in",
	NOT_IN:                         "not-in",
	IS:                             "is",
	IS_NOT:                         "is-not",
	KEYOF:                          "keyof",
	NOT_MATCH:                      "not-match",
	SUBSTROF:                       "substrof",
	SELF_CLOSING_TAG_TERMINATOR:    "/>",
	END_TAG_OPEN_DELIMITER:         "</",
	OPENING_BRACKET:                "[",
	CLOSING_BRACKET:                "]",
	OPENING_CURLY_BRACKET:          "{",
	CLOSING_CURLY_BRACKET:          "}",
	OPENING_DICTIONARY_BRACKET:     ":{",
	OPENING_KEYLIST_BRACKET:        ".{",
	OPENING_RECORD_BRACKET:         "#{",
	OPENING_TUPLE_BRACKET:          "#[",
	OPENING_OBJECT_PATTERN_BRACKET: "%{",
	OPENING_LIST_PATTERN_BRACKET:   "%[",
	OPENING_PARENTHESIS:            "(",
	CLOSING_PARENTHESIS:            ")",
	SINGLE_INTERP_OPENING_BRACE:    "{",
	SINGLE_INTERP_CLOSING_BRACE:    "}",
	PATTERN_UNION_OPENING_PIPE:     "%|",
	PATTERN_UNION_PIPE:             "|",
	ARROW:                          "=>",
	PIPE:                           "|",
	COMMA:                          ",",
	COLON:                          ":",
	DOUBLE_COLON:                   "::",
	SEMICOLON:                      ";",
	CSS_SELECTOR_PREFIX:            "s!",
	TWO_DOTS:                       "..",
	THREE_DOTS:                     "...",
	DOT_LESS_THAN:                  ".<",
	PLUS:                           "+",
	PLUS_DOT:                       "+.",
	MINUS:                          "-",
	MINUS_DOT:                      "-.",
	ASTERISK:                       "*",
	ASTERISK_DOT:                   "*.",
	SLASH:                          "/",
	SLASH_DOT:                      "/.",
	DOT:                            ".",
	DOT_DOT_LESS_THAN:              "..<",
	GREATER_THAN:                   ">",
	GREATER_THAN_DOT:               ">.",
	GREATER_OR_EQUAL:               ">=",
	GREATER_OR_EQUAL_DOT:           ">=.",
	LESS_THAN:                      "<",
	LESS_THAN_DOT:                  "<.",
	LESS_OR_EQUAL:                  "<=",
	LESS_OR_EQUAL_DOT:              "<=.",
	EQUAL:                          "=",
	EQUAL_EQUAL:                    "==",
	PLUS_EQUAL:                     "+=",
	MINUS_EQUAL:                    "-=",
	MUL_EQUAL:                      "*=",
	DIV_EQUAL:                      "/=",
	AT_SIGN:                        "@",
	DOLLAR:                         "$",
	ANTI_SLASH:                     "\\",
	QUERY_PARAM_QUESTION_MARK:      "?",
	QUERY_PARAM_SEP:                "&",
	QUESTION_MARK:                  "?",
	BACKQUOTE:                      "`",
	STR_INTERP_OPENING_BRACKETS:    "{{",
	STR_INTERP_CLOSING_BRACKETS:    "}}",
	NEWLINE:                        "\n",

	// WITH VALUE
	INT_LITERAL:                           "<?>",
	NIL_LITERAL:                           "<?>",
	FLOAT_LITERAL:                         "<?>",
	PORT_LITERAL:                          "<?>",
	BOOLEAN_LITERAL:                       "<?>",
	QUOTED_STRING_LITERAL:                 "<?>",
	UNQUOTED_STRING_LITERAL:               "<?>",
	MULTILINE_STRING_LITERAL:              "<?>",
	REGEX_LITERAL:                         "<?>",
	RATE_LITERAL:                          "<?>",
	QUANTITY_LITERAL:                      "<?>",
	DATE_LITERAL:                          "<?>",
	FLAG_LITERAL:                          "<?>",
	RUNE_LITERAL:                          "<?>",
	AT_HOST_LITERAL:                       "<?>",
	HOST_LITERAL:                          "<?>",
	URL_LITERAL:                           "<?>",
	URL_PATTERN_LITERAL:                   "<?>",
	HTTP_HOST_PATTERN_LITERAL:             "<?>",
	PATTERN_IDENTIFIER_LITERAL:            "<?>",
	UNPREFIXED_PATTERN_IDENTIFIER_LITERAL: "<?>",
	PATTERN_NAMESPACE_IDENTIFIER_LITERAL:  "<?>",
	UNPREFIXED_PATTERN_NAMESPACE_IDENTIFIER_LITERAL: "<?>",
	IDENTIFIER_LITERAL:             "<?>",
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
	NAMED_PATH_SEGMENT:             "<?>",
}

var tokenTypenames = [...]string{
	IF_KEYWORD:                     "IF_KEYWORD",
	ELSE_KEYWORD:                   "ELSE_KEYWORD",
	PREINIT_KEYWORD:                "PREINIT_KEYWORD",
	MANIFEST_KEYWORD:               "MANIFEST_KEYWORD",
	INCLUDABLE_CHUNK_KEYWORD:       "INCLUDABLE_CHUNK_KEYWORD",
	DROP_PERMS_KEYWORD:             "DROP_PERMS_KEYWORD",
	ASSIGN_KEYWORD:                 "ASSIGN_KEYWORD",
	READONLY_KEYWORD:               "READONLY_KEYWORD",
	CONST_KEYWORD:                  "CONST_KEYWORD",
	VAR_KEYWORD:                    "VAR_KEYWORD",
	FOR_KEYWORD:                    "FOR_KEYWORD",
	WALK_KEYWORD:                   "WALK_KEYWORD",
	IN_KEYWORD:                     "IN_KEYWORD",
	GO_KEYWORD:                     "GO_KEYWORD",
	IMPORT_KEYWORD:                 "IMPORT_KEYWORD",
	FN_KEYWORD:                     "FN_KEYWORD",
	PERCENT_FN:                     "PERCENT_FN",
	SWITCH_KEYWORD:                 "SWITCH_KEYWORD",
	MATCH_KEYWORD:                  "MATCH_KEYWORD",
	DEFAULTCASE_KEYWORD:            "DEFAULTCASE_KEYWORD",
	RETURN_KEYWORD:                 "RETURN_KEYWORD",
	YIELD_KEYWORD:                  "YIELD_KEYWORD",
	BREAK_KEYWORD:                  "BREAK_KEYWORD",
	CONTINUE_KEYWORD:               "CONTINUE_KEYWORD",
	PRUNE_KEYWORD:                  "PRUNE_KEYWORD",
	ASSERT_KEYWORD:                 "ASSERT_KEYWORD",
	SELF_KEYWORD:                   "SELF_KEYWORD",
	MAPPING_KEYWORD:                "MAPPING_KEYWORD",
	COMP_KEYWORD:                   "COMP_KEYWORD",
	UDATA_KEYWORD:                  "UDATA_KEYWORD",
	CONCAT_KEYWORD:                 "CONCAT_KEYWORD",
	TESTSUITE_KEYWORD:              "TESTSUITE_KEYWORD",
	TESTCASE_KEYWORD:               "TESTCASE_KEYWORD",
	SYNCHRONIZED_KEYWORD:           "SYNCHRONIZED_KEYWORD",
	LIFETIMEJOB_KEYWORD:            "LIFETIMEJOB_KEYWORD",
	ON_KEYWORD:                     "ON_KEYWORD",
	RECEIVED_KEYWORD:               "RECEIVED_KEYWORD",
	DO_KEYWORD:                     "DO_KEYWORD",
	CHUNKED_KEYWORD:                "CHUNKED_KEYWORD",
	SENDVAL_KEYWORD:                "SENDVAL_KEYWORD",
	PATTERN_KEYWORD:                "PATTERN_KEYWORD",
	PNAMESPACE_KEYWORD:             "PNAMESPACE_KEYWORD",
	EXTEND_KEYWORD:                 "EXTEND_KEYWORD",
	TO_KEYWORD:                     "TO_KEYWORD",
	AND_KEYWORD:                    "AND_KEYWORD",
	OR_KEYWORD:                     "OR_KEYWORD",
	PERCENT_STR:                    "PERCENT_STR",
	IN:                             "IN",
	NOT_IN:                         "NOT_IN",
	IS:                             "IS",
	IS_NOT:                         "IS_NOT",
	KEYOF:                          "KEYOF",
	NOT_MATCH:                      "NOT_MATCH",
	SUBSTROF:                       "SUBSTROF",
	PERCENT_SYMBOL:                 "PERCENT_SYMBOL",
	TILDE:                          "TILDE",
	EXCLAMATION_MARK:               "EXCLAMATION_MARK",
	EXCLAMATION_MARK_EQUAL:         "EXCLAMATION_MARK_EQUAL",
	DOUBLE_QUESTION_MARK:           "DOUBLE_QUESTION_MARK",
	PLUS:                           "PLUS",
	PLUS_DOT:                       "PLUS_DOT",
	MINUS:                          "MINUS",
	MINUS_DOT:                      "MINUS_DOT",
	ASTERISK:                       "ASTERISK",
	ASTERISK_DOT:                   "ASTERISK_DOT",
	SLASH:                          "SLASH",
	SLASH_DOT:                      "SLASH_DOT",
	GREATER_THAN:                   "GREATER_THAN",
	GREATER_THAN_DOT:               "GREATER_THAN_DOT",
	GREATER_OR_EQUAL:               "GREATER_OR_EQUAL",
	GREATER_OR_EQUAL_DOT:           "GREATER_OR_EQUAL_DOT",
	LESS_THAN:                      "LESS_THAN",
	LESS_THAN_DOT:                  "LESS_THAN_DOT",
	LESS_OR_EQUAL:                  "LESS_OR_EQUAL",
	LESS_OR_EQUAL_DOT:              "LESS_OR_EQUAL_DOT",
	OPENING_BRACKET:                "OPENING_BRACKET",
	SELF_CLOSING_TAG_TERMINATOR:    "SELF_CLOSING_TAG_TERMINATOR",
	END_TAG_OPEN_DELIMITER:         "END_TAG_OPEN_DELIMITER",
	CLOSING_BRACKET:                "CLOSING_BRACKET",
	OPENING_CURLY_BRACKET:          "OPENING_CURLY_BRACKET",
	CLOSING_CURLY_BRACKET:          "CLOSING_CURLY_BRACKET",
	OPENING_DICTIONARY_BRACKET:     "OPENING_DICTIONARY_BRACKET",
	OPENING_OBJECT_PATTERN_BRACKET: "OPENING_OBJECT_PATTERN_BRACKET",
	OPENING_LIST_PATTERN_BRACKET:   "OPENING_LIST_PATTERN_BRACKET",
	OPENING_RECORD_BRACKET:         "OPENING_RECORD_BRACKET",
	OPENING_TUPLE_BRACKET:          "OPENING_TUPLE_BRACKET",
	OPENING_PARENTHESIS:            "OPENING_PARENTHESIS",
	CLOSING_PARENTHESIS:            "CLOSING_PARENTHESIS",
	SINGLE_INTERP_OPENING_BRACE:    "SINGLE_INTERP_OPENING_BRACE",
	SINGLE_INTERP_CLOSING_BRACE:    "SINGLE_INTERP_CLOSING_BRACE",
	PATTERN_UNION_OPENING_PIPE:     "PATTERN_UNION_OPENING_PIPE",
	PATTERN_UNION_PIPE:             "PATTERN_UNION_PIPE",
	ARROW:                          "ARROW",
	PIPE:                           "PIPE",
	COMMA:                          "COMMA",
	COLON:                          "COLON",
	DOUBLE_COLON:                   "DOUBLE_COLON",
	SEMICOLON:                      "SEMICOLON",
	CSS_SELECTOR_PREFIX:            "CSS_SELECTOR_PREFIX",
	DOT:                            "DOT",
	TWO_DOTS:                       "TWO_DOTS",
	DOT_DOT_LESS_THAN:              "DOT_DOT_LESS_THAN",
	THREE_DOTS:                     "THREE_DOTS",
	DOT_LESS_THAN:                  "DOT_LESS_THAN",
	EQUAL:                          "EQUAL",
	EQUAL_EQUAL:                    "EQUAL_EQUAL",
	PLUS_EQUAL:                     "PLUS_EQUAL",
	MINUS_EQUAL:                    "MINUS_EQUAL",
	MUL_EQUAL:                      "MUL_EQUAL",
	DIV_EQUAL:                      "DIV_EQUAL",
	AT_SIGN:                        "AT_SIGN",
	ANTI_SLASH:                     "ANTI_SLASH",
	DOLLAR:                         "DOLLAR",
	QUERY_PARAM_QUESTION_MARK:      "QUERY_PARAM_QUESTION_MARK",
	QUESTION_MARK:                  "QUESTION_MARK",
	BACKQUOTE:                      "BACKQUOTE",
	STR_INTERP_OPENING_BRACKETS:    "STR_INTERP_OPENING_BRACKETS",
	STR_INTERP_CLOSING_BRACKETS:    "STR_INTERP_CLOSING_BRACKETS",
	NEWLINE:                        "NEWLINE",

	//WITH: "WITH", VALUE: "VALUE",
	UNEXPECTED_CHAR:                       "UNEXPECTED_CHAR",
	INVALID_OPERATOR:                      "INVALID_OPERATOR",
	INVALID_INTERP_SLICE:                  "INVALID_INTERP_SLICE",
	INVALID_URL_LIT:                       "INVALID_URL_LIT",
	INVALID_HOST_ALIAS:                    "INVALID_HOST_ALIAS",
	COMMENT:                               "COMMENT",
	INT_LITERAL:                           "INT_LITERAL",
	NIL_LITERAL:                           "NIL_LITERAL",
	FLOAT_LITERAL:                         "FLOAT_LITERAL",
	PORT_LITERAL:                          "PORT_LITERAL",
	BOOLEAN_LITERAL:                       "BOOLEAN_LITERAL",
	QUOTED_STRING_LITERAL:                 "QUOTED_STRING_LITERAL",
	UNQUOTED_STRING_LITERAL:               "UNQUOTED_STRING_LITERAL",
	MULTILINE_STRING_LITERAL:              "MULTILINE_STRING_LITERAL",
	REGEX_LITERAL:                         "REGEX_LITERAL",
	RATE_LITERAL:                          "RATE_LITERAL",
	QUANTITY_LITERAL:                      "QUANTITY_LITERAL",
	DATE_LITERAL:                          "DATE_LITERAL",
	FLAG_LITERAL:                          "FLAG_LITERAL",
	RUNE_LITERAL:                          "RUNE_LITERAL",
	AT_HOST_LITERAL:                       "AT_HOST_LITERAL",
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
	PROP_NAME_LITERAL:              "PROP_NAME_LITERAL",
	UNAMBIGUOUS_IDENTIFIER_LITERAL: "UNAMBIGUOUS_IDENTIFIER_LITERAL",
	LOCAL_VARNAME:                  "LOCAL_VARNAME",
	GLOBAL_VARNAME:                 "GLOBAL_VARNAME",
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
	XML_TEXT_SLICE:                 "XML_TEXT_SLICE",
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

// GetTokens reconstructs a sequence of tokens from a Node, the returned slice should NOT be modified.
func GetTokens(node Node, addMeta bool) []Token {
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
		return utils.CopySlice(tokens)
	}

	//we unlock when the function is finished because we modify the tokenCache at the end
	defer tokenCacheLock.Unlock()

	// some tokens can be missing if parent if GetTokens() is called on a leaf node (parents could hold the tokens)

	tokens := make([]Token, 0)
	Walk(node, func(node, parentNode, _ Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
		tokens = append(tokens, node.Base().Tokens...)

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

			return Continue, nil
		case *DynamicMemberExpression:
			i := n.Left.Base().Span.End

			tokens = append(tokens, Token{
				Type: DOT_LESS_THAN,
				Span: NodeSpan{i, i + 2},
			})

			if n.Optional {
				tokens = append(tokens, Token{
					Type: QUESTION_MARK,
					Span: NodeSpan{i + 2, i + 3},
				})
			}

			return Continue, nil
		case *IdentifierMemberExpression:
			for _, ident := range n.PropertyNames {
				i := ident.Base().Span.Start - 1

				tokens = append(tokens, Token{
					Type: DOT,
					Span: NodeSpan{i, i + 1},
				})
			}
			return Continue, nil
		case *URLExpression:

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

			return Continue, nil
		case *URLQueryParameter:

			tokens = append(tokens, Token{
				Type: QUERY_PARAM_KEY_EQUAL,
				Span: NodeSpan{n.Span.Start, n.Span.Start + int32(utf8.RuneCountInString(n.Name)) + 1},
				Raw:  n.Name + "=",
			})

			return Continue, nil
		case *OptionalPatternExpression:
			tokens = append(tokens, Token{
				Type: QUESTION_MARK,
				Span: NodeSpan{n.Span.End - 1, n.Span.End},
			})
			return Continue, nil

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
				Type: EQUAL,
				Span: NodeSpan{nameEnd, nameEnd + 1},
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
				Type: EQUAL,
				Span: NodeSpan{nameEnd, nameEnd + 1},
			})
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
		case *QuotedStringLiteral:
			tokenType = QUOTED_STRING_LITERAL
			raw = n.Raw
		case *UnquotedStringLiteral:
			tokenType = UNQUOTED_STRING_LITERAL
			raw = n.Raw
		case *MultilineStringLiteral:
			tokenType = MULTILINE_STRING_LITERAL
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
		case *DateLiteral:
			tokenType = DATE_LITERAL
			raw = n.Raw
		case *FlagLiteral:
			tokenType = FLAG_LITERAL
			raw = n.Raw
		case *RuneLiteral:
			tokenType = RUNE_LITERAL
			raw = "'" + string(n.Value) + "'"
		case *AtHostLiteral:
			tokenType = AT_HOST_LITERAL
			raw = n.Value
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
		case *UnambiguousIdentifierLiteral:
			tokenType = UNAMBIGUOUS_IDENTIFIER_LITERAL
			raw = "#" + n.Name
		case *Variable:
			tokenType = LOCAL_VARNAME
			raw = "$" + n.Name
		case *GlobalVariable:
			tokenType = GLOBAL_VARNAME
			raw = "$$" + n.Name
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
		case *InvalidAliasRelatedNode:
			tokenType = INVALID_HOST_ALIAS
			raw = n.Raw
		case *Comment:
			tokenType = COMMENT
			raw = n.Raw
		case *XMLText:
			tokenType = XML_TEXT_SLICE
			raw = n.Raw
		}

		if tokenType > 0 {
			tokens = append(tokens, Token{
				Type: tokenType,
				Meta: tokenMeta,
				Span: literalSpan,
				Raw:  raw,
			})
		}
		return Continue, nil
	}, nil)

	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].Span.Start < tokens[j].Span.Start
	})

	var uniqueTokens []Token

	for i := 0; i < len(tokens); i++ {
		if tokens[i].Span.End == tokens[i].Span.Start { //ignore empty tokens
			continue
		}
		if i != 0 && tokens[i].Span == tokens[i-1].Span {
			continue
		}
		uniqueTokens = append(uniqueTokens, tokens[i])
	}

	if len(tokens) >= MIN_TOKEN_CACHING_COUNT {
		tokenCache[ptr] = utils.CopySlice(uniqueTokens)

		// we remove the cache entry when the node is no longer reachable
		runtime.SetFinalizer(node, func(n Node) {
			tokenCacheLock.Lock()
			delete(tokenCache, reflect.ValueOf(n).Pointer())
			tokenCacheLock.Unlock()
		})
	}

	return uniqueTokens
}

func GetFirstToken(node Node) Token {
	tokens := GetTokens(node, false)
	return tokens[0]
}

func GetFirstTokenString(node Node) string {
	tokens := GetTokens(node, false)
	return tokens[0].Raw
}

func GetTokenAtPosition(pos int, node Node) (Token, bool) {
	tokens := GetTokens(node, false)
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
