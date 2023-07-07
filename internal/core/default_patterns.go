package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANYVAL_PATTERN = &TypePattern{
		Type:          VALUE_TYPE,
		Name:          "any",
		SymbolicValue: &symbolic.Any{},
	}

	IDENT_PATTERN = &TypePattern{
		Type:          IDENTIFIER_TYPE,
		Name:          "ident",
		SymbolicValue: &symbolic.Identifier{},
	}
	PROPNAME_PATTERN = &TypePattern{
		Type:          PROPNAME_TYPE,
		Name:          "propname",
		SymbolicValue: &symbolic.PropertyName{},
	}
	RUNE_PATTERN = &TypePattern{
		Type:          RUNE_TYPE,
		Name:          "rune",
		SymbolicValue: &symbolic.Rune{},
	}
	BYTE_PATTERN = &TypePattern{
		Type:          BYTE_TYPE,
		Name:          "byte",
		SymbolicValue: &symbolic.Byte{},
	}
	ANY_PATH_STRING_PATTERN = NewStringPathPattern("")
	PATH_PATTERN            = &TypePattern{
		Type:          PATH_TYPE,
		Name:          "path",
		SymbolicValue: &symbolic.Path{},
		stringPattern: func() (StringPattern, bool) {
			return ANY_PATH_STRING_PATTERN, true
		},
		symbolicStringPattern: func() (symbolic.StringPatternElement, bool) {
			//TODO
			return symbolic.ANY_STR_PATTERN_ELEM, true
		},
	}
	STR_PATTERN = &TypePattern{
		Type:          STR_LIKE_INTERFACE_TYPE,
		Name:          "str",
		SymbolicValue: symbolic.ANY_STR_LIKE,
	}
	URL_PATTERN = &TypePattern{
		Type:          URL_TYPE,
		Name:          "url",
		SymbolicValue: &symbolic.URL{},
	}
	SCHEME_PATTERN = &TypePattern{
		Type:          SCHEME_TYPE,
		Name:          "scheme",
		SymbolicValue: &symbolic.Scheme{},
	}
	HOST_PATTERN = &TypePattern{
		Type:          HOST_TYPE,
		Name:          "host",
		SymbolicValue: &symbolic.Host{},
	}
	EMAIL_ADDR_PATTERN = &TypePattern{
		Type:          EMAIL_ADDR_TYPE,
		Name:          "emailaddr",
		SymbolicValue: &symbolic.EmailAddress{},
	}
	OBJECT_PATTERN = &TypePattern{
		Type:          OBJECT_TYPE,
		Name:          "obj",
		SymbolicValue: symbolic.NewAnyObject(),
	}
	EMPTY_INEXACT_OBJECT_PATTERN = NewInexactObjectPattern(map[string]Pattern{})
	RECORD_PATTERN               = &TypePattern{
		Type: RECORD_TYPE,
		Name: "rec",
		CallImpl: func(typePattern *TypePattern, values []Value) (Pattern, error) {
			var recordPattern *RecordPattern

			for _, val := range values {
				switch v := val.(type) {
				case *ObjectPattern:
					if recordPattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("pattern")
					}

					recordPattern = &RecordPattern{
						entryPatterns: v.entryPatterns,
						inexact:       v.inexact,
					}
				case *RecordPattern:
					if recordPattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("pattern")
					}

					recordPattern = v
				default:
					return nil, FmtErrInvalidArgument(v)
				}
			}

			return recordPattern, nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			var recordPattern *symbolic.RecordPattern

			for _, val := range values {
				switch v := val.(type) {
				case *symbolic.ObjectPattern:
					if recordPattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("pattern")
					}

					recordPattern = v.ToRecordPattern()
				case *symbolic.RecordPattern:
					if recordPattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("pattern")
					}

					recordPattern = v
				default:
					return nil, errors.New(symbolic.FmtInvalidArg(0, v, symbolic.NewAnyObjectPattern()))
				}
			}

			return recordPattern, nil
		},
		SymbolicValue: symbolic.NewAnyrecord(),
	}
	LIST_PATTERN = &TypePattern{
		Type:          LIST_PTR_TYPE,
		Name:          "list",
		SymbolicValue: symbolic.NewListOf(&symbolic.Any{}),
	}
	TUPLE_PATTERN = &TypePattern{
		Type: TUPLE_TYPE,
		Name: "tuple",
		CallImpl: func(typePattern *TypePattern, values []Value) (Pattern, error) {
			var elemPattern Pattern

			for _, val := range values {
				switch v := val.(type) {
				case Pattern:
					if elemPattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("pattern")
					}
					elemPattern = v
				default:
					return nil, FmtErrInvalidArgument(v)
				}
			}

			return NewTuplePatternOf(elemPattern), nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			var elemPattern symbolic.Pattern

			for _, val := range values {
				switch v := val.(type) {
				case symbolic.Pattern:
					if elemPattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("pattern")
					}
					elemPattern = v
				default:
					return nil, errors.New(symbolic.FmtInvalidArg(0, v, symbolic.ANY_TUPLE))
				}
			}

			return symbolic.NewTuplePatternOf(elemPattern), nil
		},
		SymbolicValue: symbolic.ANY_TUPLE,
	}
	DICTIONARY_PATTERN = &TypePattern{
		Type:          DICT_TYPE,
		Name:          "dict",
		SymbolicValue: symbolic.NewAnyDictionary(),
	}
	RUNESLICE_PATTERN = &TypePattern{
		Type:          RUNE_SLICE_TYPE,
		Name:          "runes",
		SymbolicValue: &symbolic.RuneSlice{},
	}
	BYTESLICE_PATTERN = &TypePattern{
		Type:          BYTE_SLICE_TYPE,
		Name:          "bytes",
		SymbolicValue: &symbolic.ByteSlice{},
	}
	KEYLIST_PATTERN = &TypePattern{
		Type:          KEYLIST_TYPE,
		Name:          "keylist",
		SymbolicValue: symbolic.NewAnyKeyList(),
	}
	BOOL_PATTERN = &TypePattern{
		Type:          BOOL_TYPE,
		Name:          "bool",
		RandomImpl:    RandBool,
		SymbolicValue: &symbolic.Bool{},
	}
	INT_PATTERN = &TypePattern{
		Type:       INT_TYPE,
		Name:       "int",
		RandomImpl: RandInt,
		CallImpl: func(typePattern *TypePattern, values []Value) (Pattern, error) {
			intRangeProvided := false
			var intRange IntRange

			for _, val := range values {
				switch v := val.(type) {
				case IntRange:
					if intRangeProvided {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("range")
					}
					intRange = v
					intRangeProvided = true

					if intRange.unknownStart {
						return nil, fmt.Errorf("provided int range should not have an unknown start")
					}
				default:
					return nil, FmtErrInvalidArgument(v)
				}
			}

			if !intRangeProvided {
				return nil, commonfmt.FmtMissingArgument("range")
			}

			return &IntRangePattern{
				intRange: intRange,
				CallBasedPatternReprMixin: CallBasedPatternReprMixin{
					Callee: typePattern,
					Params: []Value{intRange},
				},
			}, nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			return &symbolic.IntRangePattern{}, nil
		},
		SymbolicValue: &symbolic.Int{},

		stringPattern: func() (StringPattern, bool) {
			//TODO: use real range when int range string pattern supports any range
			return NewIntRangeStringPattern(-999999999999999999, 999999999999999999, nil), true
		},
		symbolicStringPattern: func() (symbolic.StringPatternElement, bool) {
			//TODO
			return symbolic.ANY_STR_PATTERN_ELEM, true
		},
	}
	FLOAT_PATTERN = &TypePattern{
		Type:          FLOAT64_TYPE,
		Name:          "float",
		SymbolicValue: &symbolic.Float{},
	}
	ASTNODE_PATTERN = &TypePattern{
		Type:          NODE_TYPE,
		Name:          "inox.node",
		SymbolicValue: symbolic.ANY_AST_NODE,
	}
	MOD_PATTERN = &TypePattern{
		Type:          MODULE_TYPE,
		Name:          "inox.module",
		SymbolicValue: symbolic.ANY_MODULE,
	}
	HOSTPATTERN_PATTERN = &TypePattern{
		Type:          HOST_PATT_TYPE,
		Name:          "host_patt",
		SymbolicValue: &symbolic.HostPattern{},
	}
	PATHPATTERN_PATTERN = &TypePattern{
		Type:          PATH_PATT_TYPE,
		Name:          "path_patt",
		SymbolicValue: &symbolic.PathPattern{},
	}
	URLPATTERN_PATTERN = &TypePattern{
		Type:          URL_PATT_TYPE,
		Name:          "url_patt",
		SymbolicValue: &symbolic.URLPattern{},
	}
	OPTION_PATTERN = &TypePattern{
		Type:          OPTION_TYPE,
		Name:          "opt",
		SymbolicValue: &symbolic.OptionPattern{},
	}
	FILE_MODE_PATTERN = &TypePattern{
		Type:          FILE_MODE_TYPE,
		Name:          "filemode",
		SymbolicValue: &symbolic.FileMode{},
	}
	DATE_PATTERN = &TypePattern{
		Type:          DATE_TYPE,
		Name:          "date",
		SymbolicValue: &symbolic.Date{},
	}

	PATTERN_PATTERN = &TypePattern{
		Type:          PATTERN_INTERFACE_TYPE,
		Name:          "pattern",
		SymbolicValue: symbolic.ANY_PATTERN,
	}
	READABLE_PATTERN = &TypePattern{
		Type:          READABLE_INTERFACE_TYPE,
		Name:          "readable",
		SymbolicValue: symbolic.ANY_READABLE,
	}
	ITERABLE_PATTERN = &TypePattern{
		Type:          ITERABLE_INTERFACE_TYPE,
		Name:          "iterable",
		SymbolicValue: symbolic.ANY_ITERABLE,
	}
	INDEXABLE_PATTERN = &TypePattern{
		Type:          INDEXABLE_INTERFACE_TYPE,
		Name:          "indexable",
		SymbolicValue: symbolic.ANY_INDEXABLE,
	}
	VALUE_RECEIVER_PATTERN = &TypePattern{
		Type:          VALUE_RECEIVER_INTERFACE_TYPE,
		Name:          "value-receiver",
		SymbolicValue: symbolic.ANY_MSG_RECEIVER,
	}
	STRLIKE_PATTERN = &TypePattern{
		Type:          STRLIKE_INTERFACE_TYPE,
		Name:          "strlike",
		SymbolicValue: symbolic.ANY_STR_LIKE,
	}
	EVENT_PATTERN = &TypePattern{
		Type: EVENT_TYPE,
		Name: "event",
		CallImpl: func(typePattern *TypePattern, args []Value) (Pattern, error) {
			var valuePattern Pattern

			for _, arg := range args {
				switch a := arg.(type) {
				case Pattern:
					if valuePattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("value pattern")
					}
					valuePattern = a
				default:
					if valuePattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("value pattern")
					}
					valuePattern = &ExactValuePattern{value: a}
				}
			}
			return NewEventPattern(valuePattern), nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, args []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			var valuePattern symbolic.Pattern

			for _, arg := range args {
				switch a := arg.(type) {
				case symbolic.Pattern:
					if valuePattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("value pattern")
					}
					valuePattern = a
				default:
					if valuePattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("value pattern")
					}
					p, err := symbolic.NewExactValuePattern(a)
					if err != nil {
						return nil, fmt.Errorf("argument should be immutable")
					}

					valuePattern = p
				}
			}

			patt, err := symbolic.NewEventPattern(valuePattern)
			if err != nil {
				ctx.AddSymbolicGoFunctionError(err.Error())
				return symbolic.NewEventPattern(symbolic.ANY_PATTERN)
			}
			return patt, nil
		},
		SymbolicValue: utils.Must(symbolic.NewEvent(&symbolic.Any{})),
	}
	MUTATION_PATTERN = &TypePattern{
		Type:          MUTATION_TYPE,
		Name:          "mutation",
		SymbolicValue: symbolic.ANY_MUTATION,
		CallImpl: func(typePattern *TypePattern, args []Value) (Pattern, error) {
			switch len(args) {
			case 2:
			case 1:
			default:
				return nil, commonfmt.FmtErrNArgumentsExpected("1 or 2")
			}

			var kind MutationKind

			switch a := args[0].(type) {
			case Identifier:
				k, ok := mutationKindFromString(string(a))
				if !ok {
					return nil, FmtErrInvalidArgumentAtPos(a, 0)
				}
				kind = k
			default:
				return nil, FmtErrInvalidArgumentAtPos(a, 0)
			}

			var data0Pattern Pattern = ANYVAL_PATTERN

			if len(args) > 1 {
				patt, ok := args[1].(Pattern)
				if !ok {
					patt = &ExactValuePattern{value: args[1]}
				}
				data0Pattern = patt
			}

			return NewMutationPattern(kind, data0Pattern), nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, args []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			switch len(args) {
			case 2:
			case 1:
			default:
				return nil, commonfmt.FmtErrNArgumentsExpected("1 or 2")
			}

			switch a := args[0].(type) {
			case *symbolic.Identifier:
				k, ok := mutationKindFromString(a.Name())
				if !ok {
					return nil, fmt.Errorf("unknown mutation kind '%s'", k)
				}
			default:
				return nil, fmt.Errorf("mutation kind expected at position 0 but is a(n) '%s'", symbolic.Stringify(a))
			}

			var data0Pattern symbolic.Pattern = symbolic.ANY_PATTERN //TODO: use symbolic any value

			if len(args) > 1 {
				patt, ok := args[1].(symbolic.Pattern)
				if !ok {
					p, err := symbolic.NewExactValuePattern(args[1])
					if err != nil {
						return nil, fmt.Errorf("second argument should be immutable")
					}
					patt = p
				}
				data0Pattern = patt
			}

			return symbolic.NewMutationPattern(&symbolic.Int{}, data0Pattern), nil
		},
	}
	MSG_PATTERN = &TypePattern{
		Type:          MSG_TYPE,
		Name:          "message",
		SymbolicValue: symbolic.ANY_MSG,
	}
	ERROR_PATTERN = &TypePattern{
		Type:          ERROR_TYPE,
		Name:          "error",
		SymbolicValue: symbolic.ANY_ERR,
	}
	SOURCE_POS_PATTERN = NewInexactRecordPattern(map[string]Pattern{
		"source": STR_PATTERN,
		"line":   INT_PATTERN,
		"column": INT_PATTERN,
		"start":  INT_PATTERN,
		"end":    INT_PATTERN,
	})
	INT_RANGE_PATTERN = &TypePattern{
		Type:          INT_RANGE_TYPE,
		Name:          "int-range",
		SymbolicValue: symbolic.ANY_INT_RANGE,
	}
	VALUE_HISTORY_PATTERN = &TypePattern{
		Type:          VALUE_HISTORY_TYPE,
		Name:          "value-history",
		SymbolicValue: symbolic.ANY_VALUE_HISTORY,
	}
	SYSGRAPH_PATTERN = &TypePattern{
		Type:          SYSGRAPH_TYPE,
		Name:          "sysgraph",
		SymbolicValue: symbolic.ANY_SYSTEM_GRAPH,
	}
	SYSGRAPH_NODE_PATTERN = &TypePattern{
		Type:          SYSGRAPH_NODE_TYPE,
		Name:          "sysgraph.node",
		SymbolicValue: symbolic.ANY_SYSTEM_GRAPH_NODE,
	}
	SECRET_PATTERN = &TypePattern{
		Type: SECRET_TYPE,
		Name: "secret",
		CallImpl: func(typePattern *TypePattern, values []Value) (Pattern, error) {
			var stringPattern StringPattern

			if len(values) == 0 {
				return nil, commonfmt.FmtMissingArgument("pattern")
			}

			for _, val := range values {
				switch v := val.(type) {
				case StringPattern:
					if stringPattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("pattern")
					}

					stringPattern = v
				default:
					return nil, FmtErrInvalidArgument(v)
				}
			}

			return &SecretPattern{stringPattern: stringPattern}, nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			var stringPattern symbolic.StringPatternElement

			if len(values) == 0 {
				return nil, commonfmt.FmtMissingArgument("pattern")
			}

			for _, val := range values {
				switch v := val.(type) {
				case symbolic.StringPatternElement:
					if stringPattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("pattern")
					}

					stringPattern = v
				default:
					return nil, errors.New(symbolic.FmtInvalidArg(0, v, symbolic.ANY_STR_PATTERN_ELEM))
				}
			}

			return symbolic.NewSecretPattern(stringPattern), nil
		},
		SymbolicValue: symbolic.ANY_SECRET,
	}
	SECRET_STRING_PATTERN     = NewSecretPattern(NewRegexPattern(".*"), false)
	SECRET_PEM_STRING_PATTERN = NewSecretPattern(NewPEMRegexPattern(".*"), true)

	DEFAULT_NAMED_PATTERNS = map[string]Pattern{
		"ident":          IDENT_PATTERN,
		"propname":       PROPNAME_PATTERN,
		"rune":           RUNE_PATTERN,
		"byte":           BYTE_PATTERN,
		"str":            STR_PATTERN,
		"path":           PATH_PATTERN,
		"url":            URL_PATTERN,
		"scheme":         SCHEME_PATTERN,
		"host":           HOST_PATTERN,
		"email_addr":     EMAIL_ADDR_PATTERN,
		"secret":         SECRET_PATTERN,
		"secret-string":  SECRET_STRING_PATTERN,
		"obj":            OBJECT_PATTERN,
		"rec":            RECORD_PATTERN,
		"tuple":          TUPLE_PATTERN,
		"list":           LIST_PATTERN,
		"dict":           DICTIONARY_PATTERN,
		"runes":          RUNESLICE_PATTERN,
		"bytes":          BYTESLICE_PATTERN,
		"key_list":       KEYLIST_PATTERN,
		"bool":           BOOL_PATTERN,
		"int":            INT_PATTERN,
		"float":          FLOAT_PATTERN,
		"filemode":       FILE_MODE_PATTERN,
		"date":           DATE_PATTERN,
		"pattern":        PATTERN_PATTERN,
		"readable":       READABLE_PATTERN,
		"iterable":       ITERABLE_PATTERN,
		"indexable":      INDEXABLE_PATTERN,
		"value-receiver": VALUE_RECEIVER_PATTERN,
		"strlike":        STRLIKE_PATTERN,
		"host_patt":      HOSTPATTERN_PATTERN,
		"path_patt":      PATHPATTERN_PATTERN,
		"url_patt":       URLPATTERN_PATTERN,
		"opt":            OPTION_PATTERN,
		"dir_entry": &ObjectPattern{
			entryPatterns: map[string]Pattern{
				"abs-path": PATH_PATTERN,
				"is-dir":   BOOL_PATTERN,
				"size":     INT_PATTERN,
				"mode":     FILE_MODE_PATTERN,
				"mod-time": DATE_PATTERN,
				"name":     STR_PATTERN,
			},
		},
		"event":         EVENT_PATTERN,
		"mutation":      MUTATION_PATTERN,
		"message":       MSG_PATTERN,
		"error":         ERROR_PATTERN,
		"int-range":     INT_RANGE_PATTERN,
		"value-history": VALUE_HISTORY_PATTERN,
		"sysgraph":      SYSGRAPH_PATTERN,
	}

	DEFAULT_PATTERN_NAMESPACES = map[string]*PatternNamespace{
		"inox": {
			Patterns: map[string]Pattern{
				"node":            ASTNODE_PATTERN,
				"module":          MOD_PATTERN,
				"source_position": SOURCE_POS_PATTERN,
			},
		},
		"date_str": {
			Patterns: map[string]Pattern{
				"rfc822":    NewDateFormat(time.RFC822),
				"date-only": NewDateFormat(time.DateOnly),
				"time-only": NewDateFormat(time.TimeOnly),
			},
		},
		"sysgraph": {
			Patterns: map[string]Pattern{
				"node": SYSGRAPH_NODE_PATTERN,
			},
		},
	}
)
