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
		SymbolicValue: symbolic.ANY,
	}
	SERIALIZABLE_PATTERN = &TypePattern{
		Type:          SERIALIZABLE_TYPE,
		Name:          "serializable",
		SymbolicValue: symbolic.ANY_SERIALIZABLE,
	}

	//TODO: improve (using a type pattern can create issues)
	VAL_PATTERN = &TypePattern{
		Type:          VALUE_TYPE,
		Name:          "__val",
		SymbolicValue: symbolic.NEVER,
		CallImpl: func(typePattern *TypePattern, values []Serializable) (Pattern, error) {
			if len(values) != 1 {
				return nil, commonfmt.FmtErrNArgumentsExpected("1")
			}

			return NewExactValuePattern(values[0]), nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			var recordPattern *symbolic.RecordPattern

			if len(values) != 1 {
				return nil, commonfmt.FmtErrNArgumentsExpected("1")
			}

			symbolic.NewExactValuePattern(values[0].(symbolic.Serializable))

			return recordPattern, nil
		},
	}
	STR_PATTERN_PATTERN = &TypePattern{
		Name:          "string-pattern",
		SymbolicValue: symbolic.NEVER,
		CallImpl: func(typePattern *TypePattern, values []Serializable) (Pattern, error) {
			if len(values) != 1 {
				return nil, commonfmt.FmtErrNArgumentsExpected("1")
			}

			pattern, ok := values[0].(Pattern)
			if !ok {
				return nil, fmt.Errorf("invalid argument")
			}

			stringPattern, ok := pattern.StringPattern()

			if !ok {
				return nil, fmt.Errorf("invalid argument")
			}

			return stringPattern, nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			if len(values) != 1 {
				return nil, commonfmt.FmtErrNArgumentsExpected("1")
			}

			pattern, ok := values[0].(symbolic.Pattern)
			if !ok {
				return nil, errors.New(symbolic.FmtInvalidArg(0, values[0], symbolic.ANY_PATTERN))
			}

			stringPattern, ok := pattern.StringPattern()
			if !ok {
				return nil, commonfmt.FmtErrInvalidArgumentAtPos(0, symbolic.Stringify(pattern)+" given but a pattern having an associated string pattern is expected")
			}

			return stringPattern, nil
		},
	}

	IDENT_PATTERN = &TypePattern{
		Type:          IDENTIFIER_TYPE,
		Name:          "ident",
		SymbolicValue: symbolic.ANY_IDENTIFIER,
	}
	PROPNAME_PATTERN = &TypePattern{
		Type:          PROPNAME_TYPE,
		Name:          "propname",
		SymbolicValue: symbolic.ANY_PROPNAME,
	}
	RUNE_PATTERN = &TypePattern{
		Type:          RUNE_TYPE,
		Name:          "rune",
		SymbolicValue: symbolic.ANY_RUNE,
	}
	BYTE_PATTERN = &TypePattern{
		Type:          BYTE_TYPE,
		Name:          "byte",
		SymbolicValue: symbolic.ANY_BYTE,
	}
	ANY_PATH_STRING_PATTERN = NewStringPathPattern("")

	PATH_PATTERN = &TypePattern{
		Type:          PATH_TYPE,
		Name:          "path",
		SymbolicValue: symbolic.ANY_PATH,
		stringPattern: func() (StringPattern, bool) {
			return ANY_PATH_STRING_PATTERN, true
		},
		symbolicStringPattern: func() (symbolic.StringPattern, bool) {
			//TODO
			return symbolic.ANY_STR_PATTERN, true
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
		SymbolicValue: symbolic.ANY_URL,
	}
	SCHEME_PATTERN = &TypePattern{
		Type:          SCHEME_TYPE,
		Name:          "scheme",
		SymbolicValue: symbolic.ANY_SCHEME,
	}
	HOST_PATTERN = &TypePattern{
		Type:          HOST_TYPE,
		Name:          "host",
		SymbolicValue: symbolic.ANY_HOST,
	}
	EMAIL_ADDR_PATTERN = &TypePattern{
		Type:          EMAIL_ADDR_TYPE,
		Name:          "emailaddr",
		SymbolicValue: symbolic.ANY_EMAIL_ADDR,
	}
	EMPTY_INEXACT_OBJECT_PATTERN = NewInexactObjectPattern(map[string]Pattern{})
	OBJECT_PATTERN               = &TypePattern{
		Type:          OBJECT_TYPE,
		Name:          "object",
		SymbolicValue: symbolic.NewAnyObject(),
	}
	EMPTY_INEXACT_RECORD_PATTERN = NewInexactRecordPattern(map[string]Pattern{})

	RECORD_PATTERN = &TypePattern{
		Type: RECORD_TYPE,
		Name: "record",
		CallImpl: func(typePattern *TypePattern, values []Serializable) (Pattern, error) {
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

	ANY_ELEM_LIST_PATTERN = NewListPatternOf(SERIALIZABLE_PATTERN)

	LIST_PATTERN = &TypePattern{
		Type:          LIST_PTR_TYPE,
		Name:          "list",
		SymbolicValue: symbolic.NewListOf(symbolic.ANY_SERIALIZABLE),
	}

	ANY_ELEM_TUPLE_PATTERN = NewTuplePatternOf(SERIALIZABLE_PATTERN)

	TUPLE_PATTERN = &TypePattern{
		Type: TUPLE_TYPE,
		Name: "tuple",
		CallImpl: func(typePattern *TypePattern, values []Serializable) (Pattern, error) {
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
		SymbolicValue: symbolic.ANY_DICT,
	}
	RUNESLICE_PATTERN = &TypePattern{
		Type:          RUNE_SLICE_TYPE,
		Name:          "runes",
		SymbolicValue: symbolic.ANY_RUNE_SLICE,
	}
	BYTESLICE_PATTERN = &TypePattern{
		Type:          BYTE_SLICE_TYPE,
		Name:          "bytes",
		SymbolicValue: symbolic.ANY_BYTE_SLICE,
	}
	KEYLIST_PATTERN = &TypePattern{
		Type:          KEYLIST_TYPE,
		Name:          "keylist",
		SymbolicValue: symbolic.ANY_KEYLIST,
	}
	BOOL_PATTERN = &TypePattern{
		Type:          BOOL_TYPE,
		Name:          "bool",
		RandomImpl:    RandBool,
		SymbolicValue: symbolic.ANY_BOOL,
	}
	INT_PATTERN = &TypePattern{
		Type:       INT_TYPE,
		Name:       "int",
		RandomImpl: RandInt,
		CallImpl: func(typePattern *TypePattern, values []Serializable) (Pattern, error) {
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
					Params: []Serializable{intRange},
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
		symbolicStringPattern: func() (symbolic.StringPattern, bool) {
			//TODO
			return symbolic.ANY_STR_PATTERN, true
		},
	}
	FLOAT_PATTERN = &TypePattern{
		Type:          FLOAT64_TYPE,
		Name:          "float",
		SymbolicValue: symbolic.ANY_FLOAT,
	}

	PORT_PATTERN = &TypePattern{
		Type:          PORT_TYPE,
		Name:          "port",
		SymbolicValue: symbolic.ANY_PORT,
	}

	BYTECOUNT_PATTERN = &TypePattern{
		Type:          BYTECOUNT_TYPE,
		Name:          "byte-count",
		SymbolicValue: symbolic.ANY_BYTECOUNT,
	}

	LINECOUNT_PATTERN = &TypePattern{
		Type:          LINECOUNT_TYPE,
		Name:          "line-count",
		SymbolicValue: symbolic.ANY_LINECOUNT,
	}

	RUNECOUNT_PATTERN = &TypePattern{
		Type:          RUNECOUNT_TYPE,
		Name:          "rune-count",
		SymbolicValue: symbolic.ANY_RUNECOUNT,
	}

	BYTERATE_PATTERN = &TypePattern{
		Type:          BYTERATE_TYPE,
		Name:          "byte-rate",
		SymbolicValue: symbolic.ANY_BYTERATE,
	}

	SIMPLERATE_PATTERN = &TypePattern{
		Type:          SIMPLERATE_TYPE,
		Name:          "simple-rate",
		SymbolicValue: symbolic.ANY_SIMPLERATE,
	}

	DURATION_PATTERN = &TypePattern{
		Type:          DURATION_TYPE,
		Name:          "duration",
		SymbolicValue: symbolic.ANY_DURATION,
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
		Name:          "host-pattern",
		SymbolicValue: &symbolic.HostPattern{},
	}
	PATHPATTERN_PATTERN = &TypePattern{
		Type:          PATH_PATT_TYPE,
		Name:          "path-pattern",
		SymbolicValue: &symbolic.PathPattern{},
	}
	URLPATTERN_PATTERN = &TypePattern{
		Type:          URL_PATT_TYPE,
		Name:          "url-pattern",
		SymbolicValue: &symbolic.URLPattern{},
	}
	OPTION_PATTERN = &TypePattern{
		Type:          OPTION_TYPE,
		Name:          "opt",
		SymbolicValue: symbolic.ANY_OPTION,
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
	READER_PATTERN = &TypePattern{
		Type:          READER_INTERFACE_TYPE,
		Name:          "reader",
		SymbolicValue: symbolic.ANY_READER,
	}
	ITERABLE_PATTERN = &TypePattern{
		Type:          ITERABLE_INTERFACE_TYPE,
		Name:          "iterable",
		SymbolicValue: symbolic.ANY_ITERABLE,
	}
	SERIALIZABLE_ITERABLE_PATTERN = &TypePattern{
		Type:          SERIALIZABLE_ITERABLE_INTERFACE_TYPE,
		Name:          "serializable-iterable",
		SymbolicValue: symbolic.ANY_SERIALIZABLE_ITERABLE,
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
		CallImpl: func(typePattern *TypePattern, args []Serializable) (Pattern, error) {
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
					p, err := symbolic.NewExactValuePattern(a.(symbolic.Serializable))
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
		CallImpl: func(typePattern *TypePattern, args []Serializable) (Pattern, error) {
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
					p, err := symbolic.NewExactValuePattern(args[1].(symbolic.Serializable))
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
	RUNE_RANGE_PATTERN = &TypePattern{
		Type:          INT_RANGE_TYPE,
		Name:          "rune-range",
		SymbolicValue: symbolic.ANY_RUNE_RANGE,
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
		CallImpl: func(typePattern *TypePattern, values []Serializable) (Pattern, error) {
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

			return &SecretPattern{
				stringPattern: stringPattern,
				CallBasedPatternReprMixin: CallBasedPatternReprMixin{
					Callee: typePattern,
					Params: []Serializable{stringPattern},
				},
			}, nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			var stringPattern symbolic.StringPattern

			if len(values) == 0 {
				return nil, commonfmt.FmtMissingArgument("pattern")
			}

			for _, val := range values {
				switch v := val.(type) {
				case symbolic.StringPattern:
					if stringPattern != nil {
						return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("pattern")
					}

					stringPattern = v
				default:
					return nil, errors.New(symbolic.FmtInvalidArg(0, v, symbolic.ANY_STR_PATTERN))
				}
			}

			return symbolic.NewSecretPattern(stringPattern), nil
		},
		SymbolicValue: symbolic.ANY_SECRET,
	}
	SECRET_STRING_PATTERN     = NewSecretPattern(NewRegexPattern(".*"), false)
	SECRET_PEM_STRING_PATTERN = NewSecretPattern(NewPEMRegexPattern(".*"), true)

	DEFAULT_NAMED_PATTERNS = map[string]Pattern{
		IDENT_PATTERN.Name:                 IDENT_PATTERN,
		PROPNAME_PATTERN.Name:              PROPNAME_PATTERN,
		RUNE_PATTERN.Name:                  RUNE_PATTERN,
		BYTE_PATTERN.Name:                  BYTE_PATTERN,
		STR_PATTERN.Name:                   STR_PATTERN,
		PATH_PATTERN.Name:                  PATH_PATTERN,
		URL_PATTERN.Name:                   URL_PATTERN,
		SCHEME_PATTERN.Name:                SCHEME_PATTERN,
		HOST_PATTERN.Name:                  HOST_PATTERN,
		EMAIL_ADDR_PATTERN.Name:            EMAIL_ADDR_PATTERN,
		SECRET_PATTERN.Name:                SECRET_PATTERN,
		"secret-string":                    SECRET_STRING_PATTERN,
		OBJECT_PATTERN.Name:                OBJECT_PATTERN,
		RECORD_PATTERN.Name:                RECORD_PATTERN,
		TUPLE_PATTERN.Name:                 TUPLE_PATTERN,
		LIST_PATTERN.Name:                  LIST_PATTERN,
		DICTIONARY_PATTERN.Name:            DICTIONARY_PATTERN,
		RUNESLICE_PATTERN.Name:             RUNESLICE_PATTERN,
		BYTESLICE_PATTERN.Name:             BYTESLICE_PATTERN,
		KEYLIST_PATTERN.Name:               KEYLIST_PATTERN,
		BOOL_PATTERN.Name:                  BOOL_PATTERN,
		INT_PATTERN.Name:                   INT_PATTERN,
		LINECOUNT_PATTERN.Name:             LINECOUNT_PATTERN,
		RUNECOUNT_PATTERN.Name:             RUNECOUNT_PATTERN,
		BYTECOUNT_PATTERN.Name:             BYTECOUNT_PATTERN,
		FLOAT_PATTERN.Name:                 FLOAT_PATTERN,
		FILE_MODE_PATTERN.Name:             FILE_MODE_PATTERN,
		DATE_PATTERN.Name:                  DATE_PATTERN,
		PATTERN_PATTERN.Name:               PATTERN_PATTERN,
		READABLE_PATTERN.Name:              READABLE_PATTERN,
		READER_PATTERN.Name:                READER_PATTERN,
		ITERABLE_PATTERN.Name:              ITERABLE_PATTERN,
		SERIALIZABLE_ITERABLE_PATTERN.Name: SERIALIZABLE_ITERABLE_PATTERN,
		INDEXABLE_PATTERN.Name:             INDEXABLE_PATTERN,
		VALUE_RECEIVER_PATTERN.Name:        VALUE_RECEIVER_PATTERN,
		STRLIKE_PATTERN.Name:               STRLIKE_PATTERN,
		HOSTPATTERN_PATTERN.Name:           HOSTPATTERN_PATTERN,
		PATHPATTERN_PATTERN.Name:           PATHPATTERN_PATTERN,
		URLPATTERN_PATTERN.Name:            URLPATTERN_PATTERN,
		OPTION_PATTERN.Name:                OPTION_PATTERN,
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
		EVENT_PATTERN.Name:         EVENT_PATTERN,
		MUTATION_PATTERN.Name:      MUTATION_PATTERN,
		MSG_PATTERN.Name:           MSG_PATTERN,
		ERROR_PATTERN.Name:         ERROR_PATTERN,
		INT_RANGE_PATTERN.Name:     INT_RANGE_PATTERN,
		VALUE_HISTORY_PATTERN.Name: VALUE_HISTORY_PATTERN,
		SYSGRAPH_PATTERN.Name:      SYSGRAPH_PATTERN,
		VAL_PATTERN.Name:           VAL_PATTERN,
	}

	DEFAULT_PATTERN_NAMESPACES = map[string]*PatternNamespace{
		"inox": {
			Patterns: map[string]Pattern{
				"node":            ASTNODE_PATTERN,
				"module":          MOD_PATTERN,
				"source_position": SOURCE_POS_PATTERN,
			},
		},
		DATE_FORMAT_PATTERN_NAMESPACE: {
			Patterns: map[string]Pattern{
				"rfc822":    NewDateFormat(time.RFC822, "rfc822"),
				"date-only": NewDateFormat(time.DateOnly, "date-only"),
				"time-only": NewDateFormat(time.TimeOnly, "time-only"),
			},
		},
		"sysgraph": {
			Patterns: map[string]Pattern{
				"node": SYSGRAPH_NODE_PATTERN,
			},
		},
	}

	//used to prevent some cycles
	getDefaultNamedPattern (func(name string) Pattern) = nil
)

func init() {
	getDefaultNamedPattern = func(name string) Pattern {
		p, ok := DEFAULT_NAMED_PATTERNS[name]
		if !ok {
			panic(fmt.Errorf("%s is not defined", name))
		}
		return p
	}
}
