package internal

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"regexp/syntax"
	"time"
	"unicode/utf8"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"

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
	RECORD_PATTERN = &TypePattern{
		Type: RECORD_TYPE,
		Name: "rec",
		CallImpl: func(values []Value) (Pattern, error) {
			var recordPattern *RecordPattern

			for _, val := range values {
				switch v := val.(type) {
				case *ObjectPattern:
					if recordPattern != nil {
						return nil, FmtErrArgumentProvidedAtLeastTwice("pattern")
					}

					recordPattern = &RecordPattern{
						entryPatterns: v.entryPatterns,
						inexact:       v.inexact,
					}
				case *RecordPattern:
					if recordPattern != nil {
						return nil, FmtErrArgumentProvidedAtLeastTwice("pattern")
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
						return nil, FmtErrArgumentProvidedAtLeastTwice("pattern")
					}

					recordPattern = v.ToRecordPattern()
				case *symbolic.RecordPattern:
					if recordPattern != nil {
						return nil, FmtErrArgumentProvidedAtLeastTwice("pattern")
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
		CallImpl: func(values []Value) (Pattern, error) {
			var elemPattern Pattern

			for _, val := range values {
				switch v := val.(type) {
				case Pattern:
					if elemPattern != nil {
						return nil, FmtErrArgumentProvidedAtLeastTwice("pattern")
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
						return nil, FmtErrArgumentProvidedAtLeastTwice("pattern")
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
		CallImpl: func(values []Value) (Pattern, error) {
			intRangeProvided := false
			var intRange IntRange

			for _, val := range values {
				switch v := val.(type) {
				case IntRange:
					if intRangeProvided {
						return nil, FmtErrArgumentProvidedAtLeastTwice("range")
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

			return &IntRangePattern{intRange: intRange}, nil
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
		CallImpl: func(args []Value) (Pattern, error) {
			var valuePattern Pattern

			for _, arg := range args {
				switch a := arg.(type) {
				case Pattern:
					if valuePattern != nil {
						return nil, FmtErrArgumentProvidedAtLeastTwice("value pattern")
					}
					valuePattern = a
				default:
					if valuePattern != nil {
						return nil, FmtErrArgumentProvidedAtLeastTwice("value pattern")
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
						return nil, FmtErrArgumentProvidedAtLeastTwice("value pattern")
					}
					valuePattern = a
				default:
					if valuePattern != nil {
						return nil, FmtErrArgumentProvidedAtLeastTwice("value pattern")
					}
					valuePattern = symbolic.NewExactValuePattern(a)
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
		CallImpl: func(args []Value) (Pattern, error) {
			switch len(args) {
			case 2:
			case 1:
			default:
				return nil, FmtErrNArgumentsExpected("1 or 2")
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
				return nil, FmtErrNArgumentsExpected("1 or 2")
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
					patt = symbolic.NewExactValuePattern(args[1])
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
		CallImpl: func(values []Value) (Pattern, error) {
			var stringPattern StringPattern

			if len(values) == 0 {
				return nil, FmtMissingArgument("pattern")
			}

			for _, val := range values {
				switch v := val.(type) {
				case StringPattern:
					if stringPattern != nil {
						return nil, FmtErrArgumentProvidedAtLeastTwice("pattern")
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
				return nil, FmtMissingArgument("pattern")
			}

			for _, val := range values {
				switch v := val.(type) {
				case symbolic.StringPatternElement:
					if stringPattern != nil {
						return nil, FmtErrArgumentProvidedAtLeastTwice("pattern")
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
	SECRET_STRING_PATTERN = NewSecretPattern(NewRegexPattern(".*"), true)

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

	ErrPatternNotCallable = errors.New("pattern is not callable")

	_ = []GroupPattern{&NamedSegmentPathPattern{}}
)

func RegisterDefaultPattern(s string, m Pattern) {
	if _, ok := DEFAULT_NAMED_PATTERNS[s]; ok {
		panic(fmt.Errorf("pattern '%s' is already registered", s))
	}
	DEFAULT_NAMED_PATTERNS[s] = m
}

func RegisterDefaultPatternNamespace(s string, ns *PatternNamespace) {
	if _, ok := DEFAULT_PATTERN_NAMESPACES[s]; ok {
		panic(fmt.Errorf("pattern namespace '%s' is already registered", s))
	}
	DEFAULT_PATTERN_NAMESPACES[s] = ns
}

type Pattern interface {
	Value
	Iterable

	//Test returns true if the argument matches the pattern.
	Test(*Context, Value) bool

	Random(ctx *Context, options ...Option) Value

	Call(values []Value) (Pattern, error)

	StringPattern() (StringPattern, bool)
}

type GroupMatchesFindConfigKind int

const (
	FindFirstGroupMatches GroupMatchesFindConfigKind = iota
	FindAllGroupMatches
)

type GroupMatchesFindConfig struct {
	Kind GroupMatchesFindConfigKind
}

type GroupPattern interface {
	Pattern
	MatchGroups(*Context, Value) (groups map[string]Value, ok bool, err error)
	FindGroupMatches(*Context, Value, GroupMatchesFindConfig) (groups []*Object, err error)
}

type PatternNamespace struct {
	NoReprMixin
	Patterns map[string]Pattern
}

type NotCallablePatternMixin struct {
}

func (NotCallablePatternMixin) Call(values []Value) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

// ExactValuePattern matches values equal to .value: .value.Equal(...) returns true.
type ExactValuePattern struct {
	NotCallablePatternMixin
	NoReprMixin

	value  Value
	regexp *syntax.Regexp
}

func NewExactValuePattern(value Value) *ExactValuePattern {
	return &ExactValuePattern{value: value}
}

func (pattern *ExactValuePattern) Test(ctx *Context, v Value) bool {
	return pattern.value.Equal(ctx, v, map[uintptr]uintptr{}, 0)
}

func (pattern *ExactValuePattern) Regex() string {
	s, isString := pattern.value.(Str)
	if !isString {
		panic(errors.New("cannot get regex for a ExactSimpleValuePattern that has a non-string value"))
	}
	return regexp.QuoteMeta(string(s))
}

func (patt *ExactValuePattern) CompiledRegex() *regexp.Regexp {
	s, isString := patt.value.(Str)
	if !isString {
		panic(errors.New("cannot get regex for a ExactSimpleValuePattern that has a non-string value"))
	}

	return regexp.MustCompile(regexp.QuoteMeta(string(s)))
}

func (pattern *ExactValuePattern) HasRegex() bool {
	_, isString := pattern.value.(Str)
	return isString
}

func (pattern *ExactValuePattern) validate(parsed string, i *int) bool {
	s, isString := pattern.value.(Str)
	if !isString {
		panic(errors.New("an ExactSimpleValuePattern that doesn't have a string value cannot validate"))
	}

	length := len(s)
	index := *i
	if len(parsed)-index < length {
		return false
	}

	if parsed[index:index+length] == string(s) {
		*i += length
		return true
	}
	return false
}

func (patt *ExactValuePattern) Parse(ctx *Context, s string) (Value, error) {
	str, isString := patt.value.(Str)
	if !isString {
		panic(errors.New("an ExactSimpleValuePattern that doesn't have a string value cannot validate"))
	}

	if s != string(str) {
		return nil, errors.New("string not equal to expected string")
	}

	return Str(s), nil
}

func (pattern *ExactValuePattern) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (matches []Value, err error) {
	_, isString := pattern.value.(Str)
	if !isString {
		panic(errors.New("an ExactSimpleValuePattern that doesn't have a string value cannot find matches"))
	}

	return FindMatchesForStringPattern(ctx, pattern, val, config)
}

func (pattern *ExactValuePattern) LengthRange() IntRange {
	s, isString := pattern.value.(Str)
	if !isString {
		panic(errors.New("an ExactSimpleValuePattern that doesn't have a string value has no length range"))
	}

	//cache ?

	length := utf8.RuneCountInString(string(s))
	return IntRange{
		Start:        int64(length),
		End:          int64(length),
		inclusiveEnd: true,
		Step:         1,
	}
}

func (pattern *ExactValuePattern) EffectiveLengthRange() IntRange {
	return pattern.LengthRange()
}

func (patt *ExactValuePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// TypePattern matches values implementing .Type (if .Type is an interface) or having their type equal to .Type
type TypePattern struct {
	Type          reflect.Type
	Name          string
	SymbolicValue symbolic.SymbolicValue
	RandomImpl    func(options ...Option) Value

	CallImpl         func(values []Value) (Pattern, error)
	SymbolicCallImpl func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error)

	stringPattern         func() (StringPattern, bool)
	symbolicStringPattern func() (symbolic.StringPatternElement, bool)
}

func (pattern *TypePattern) Test(ctx *Context, v Value) bool {
	if pattern.Type.Kind() == reflect.Interface {
		return reflect.TypeOf(v).Implements(pattern.Type)
	}
	return pattern.Type == reflect.TypeOf(v)
}

func (patt *TypePattern) Call(values []Value) (Pattern, error) {
	if patt.CallImpl == nil {
		return nil, ErrPatternNotCallable
	}
	return patt.CallImpl(values)
}

func (patt *TypePattern) StringPattern() (StringPattern, bool) {
	if patt.stringPattern == nil {
		return nil, false
	}

	return patt.stringPattern()
}

type UnionPattern struct {
	NotCallablePatternMixin
	node  parse.Node
	cases []Pattern
}

func NewUnionPattern(cases []Pattern, node parse.Node) *UnionPattern {
	return &UnionPattern{node: node, cases: cases}
}

func (patt *UnionPattern) Test(ctx *Context, v Value) bool {
	for _, case_ := range patt.cases {
		if case_.Test(ctx, v) {
			return true
		}
	}
	return false
}

func (patt *UnionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type IntersectionPattern struct {
	NotCallablePatternMixin
	NoReprMixin

	node  parse.Node
	cases []Pattern
}

func NewIntersectionPattern(cases []Pattern, node parse.Node) *IntersectionPattern {
	return &IntersectionPattern{node: node, cases: cases}
}

func (patt *IntersectionPattern) Test(ctx *Context, v Value) bool {
	for _, case_ := range patt.cases {
		if !case_.Test(ctx, v) {
			return false
		}
	}
	return true
}

func (patt *IntersectionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type ObjectPattern struct {
	NotCallablePatternMixin
	entryPatterns           map[string]Pattern
	optionalEntries         map[string]struct{}
	inexact                 bool //if true the matched object can have additional properties
	complexPropertyPatterns []*ComplexPropertyConstraint
}

func NewExactObjectPattern(entries map[string]Pattern) *ObjectPattern {
	return &ObjectPattern{entryPatterns: entries}
}

func NewInexactExactObjectPattern(entries map[string]Pattern) *ObjectPattern {
	return &ObjectPattern{entryPatterns: entries, inexact: true}
}

func (patt *ObjectPattern) Test(ctx *Context, v Value) bool {
	obj, ok := v.(*Object)
	if !ok {
		return false
	}
	if !patt.inexact && len(patt.optionalEntries) == 0 && len(obj.keys) != len(patt.entryPatterns) {
		return false
	}

	for key, valuePattern := range patt.entryPatterns {
		if !obj.HasProp(ctx, key) {
			if _, ok := patt.optionalEntries[key]; ok {
				continue
			}
			return false
		}
		value := obj.Prop(ctx, key)
		if !valuePattern.Test(ctx, value) {
			return false
		}
	}

	// if pattern is exact check that there are no additional properties
	if !patt.inexact {
		for _, propName := range obj.PropertyNames(ctx) {
			if _, ok := patt.entryPatterns[propName]; !ok {
				return false
			}
		}
	}

	state := NewTreeWalkState(NewContext(ContextConfig{}))
	state.self = obj

	for _, constraint := range patt.complexPropertyPatterns {
		res, err := TreeWalkEval(constraint.Expr, state)
		if err != nil {
			if ctx != nil {
				ctx.Logger().Print("error when checking a complex property pattern: " + err.Error())
			}
			//TODO: log error some where
			return false
		}
		if b, ok := res.(Bool); !ok {
			ctx.Logger().Print("error when checking a multiproperty pattern")
		} else if !b {
			return false
		}
	}
	return true
}

func (patt *ObjectPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt *ObjectPattern) ForEachEntry(fn func(propName string, propPattern Pattern, isOptional bool) error) error {
	for propName, propPattern := range patt.entryPatterns {
		_, isOptional := patt.optionalEntries[propName]
		if err := fn(propName, propPattern, isOptional); err != nil {
			return err
		}
	}
	return nil
}

func (patt *ObjectPattern) EntryCount() int {
	return len(patt.entryPatterns)
}

type RecordPattern struct {
	NotCallablePatternMixin
	entryPatterns   map[string]Pattern
	optionalEntries map[string]struct{}
	inexact         bool //if true the matched object can have additional properties
}

func NewInexactRecordPattern(entries map[string]Pattern) *RecordPattern {
	return &RecordPattern{
		entryPatterns: utils.CopyMap(entries),
		inexact:       true,
	}
}

func (patt *RecordPattern) Test(ctx *Context, v Value) bool {
	rec, ok := v.(*Record)
	if !ok {
		return false
	}
	if !patt.inexact && len(patt.optionalEntries) == 0 && len(rec.keys) != len(patt.entryPatterns) {
		return false
	}

	for key, valuePattern := range patt.entryPatterns {
		if !rec.HasProp(ctx, key) {
			if _, ok := patt.optionalEntries[key]; ok {
				continue
			}
			return false
		}
		value := rec.Prop(ctx, key)
		if !valuePattern.Test(ctx, value) {
			return false
		}
	}

	// if pattern is exact check that there are no additional properties
	if !patt.inexact {
		for _, propName := range rec.PropertyNames(ctx) {
			if _, ok := patt.entryPatterns[propName]; !ok {
				return false
			}
		}
	}
	return true
}

func (patt *RecordPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type ComplexPropertyConstraint struct {
	NotCallablePatternMixin
	Properties []string
	Expr       parse.Node
}

type ListPattern struct {
	NotCallablePatternMixin
	elementPatterns       []Pattern
	generalElementPattern Pattern
}

func NewListPatternOf(generalElementPattern Pattern) *ListPattern {
	return &ListPattern{generalElementPattern: generalElementPattern}
}

func (patt ListPattern) Test(ctx *Context, v Value) bool {
	list, ok := v.(*List)
	if !ok {
		return false
	}
	if patt.generalElementPattern != nil {
		length := list.Len()
		for i := 0; i < length; i++ {
			e := list.At(ctx, i)
			if !patt.generalElementPattern.Test(ctx, e) {
				return false
			}
		}
		return true
	}
	if list.Len() != len(patt.elementPatterns) {
		return false
	}
	for i, elementPattern := range patt.elementPatterns {
		if !ok || !elementPattern.Test(ctx, list.At(ctx, i)) {
			return false
		}
	}
	return true
}

func (patt *ListPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type TuplePattern struct {
	NotCallablePatternMixin
	elementPatterns       []Pattern
	generalElementPattern Pattern
}

func NewTuplePatternOf(generalElementPattern Pattern) *TuplePattern {
	return &TuplePattern{generalElementPattern: generalElementPattern}
}

func (patt *TuplePattern) Test(ctx *Context, v Value) bool {
	tuple, ok := v.(*Tuple)
	if !ok {
		return false
	}
	if patt.generalElementPattern != nil {
		length := tuple.Len()
		for i := 0; i < length; i++ {
			e := tuple.At(ctx, i)
			if !patt.generalElementPattern.Test(ctx, e) {
				return false
			}
		}
		return true
	}
	if tuple.Len() != len(patt.elementPatterns) {
		return false
	}
	for i, elementPattern := range patt.elementPatterns {
		if !ok || !elementPattern.Test(ctx, tuple.At(ctx, i)) {
			return false
		}
	}
	return true
}

func (patt *TuplePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type OptionPattern struct {
	NotCallablePatternMixin
	Name  string
	Value Pattern
}

func (patt OptionPattern) Test(ctx *Context, v Value) bool {
	opt, ok := v.(Option)
	return ok && opt.Name == patt.Name && patt.Value.Test(ctx, opt.Value)
}

func (patt *OptionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type DifferencePattern struct {
	NotCallablePatternMixin
	base    Pattern
	removed Pattern
}

func (patt *DifferencePattern) Test(ctx *Context, v Value) bool {
	return patt.base.Test(ctx, v) && !patt.removed.Test(ctx, v)
}

func (patt *DifferencePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type OptionalPattern struct {
	NotCallablePatternMixin
	Pattern Pattern
}

func NewOptionalPattern(ctx *Context, pattern Pattern) (*OptionalPattern, error) {
	if pattern.Test(ctx, Nil) {
		return nil, errors.New("cannot create optional pattern with pattern that already matches nil")
	}
	return &OptionalPattern{
		Pattern: pattern,
	}, nil
}

func (patt *OptionalPattern) Test(ctx *Context, v Value) bool {
	if _, ok := v.(NilT); ok {
		return true
	}
	return patt.Pattern.Test(ctx, v)
}

func (patt *OptionalPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type FunctionPattern struct {
	NoReprMixin
	NotCallablePatternMixin
	node *parse.FunctionPatternExpression //if nil, matches any function

	symbolicValue *symbolic.FunctionPattern //used for checking functions
}

func (patt *FunctionPattern) Test(ctx *Context, v Value) bool {
	switch fn := v.(type) {
	case *GoFunction:
		if patt.node == nil {
			return true
		}

		if fn.fn == nil {
			return false
		}

		panic(errors.New("testing a go function against a function pattern is not supported yet"))

	case *InoxFunction:

		//TO KEEP IN SYNC WITH CONCRETE FUNCTION PATTERN
		if patt.node == nil {
			return true
		}

		fnExpr := fn.FuncExpr()
		if fnExpr == nil {
			return false
		}

		if len(fnExpr.Parameters) != len(patt.node.Parameters) || fnExpr.NonVariadicParamCount() != patt.node.NonVariadicParamCount() {
			return false
		}

		for i, param := range patt.node.Parameters {
			actualParam := fnExpr.Parameters[i]

			if (param.Type == nil) != (actualParam.Type == nil) {
				return false
			}

			if param.Type != nil && parse.SPrint(param.Type, parse.PrintConfig{TrimStart: true}) != parse.SPrint(actualParam.Type, parse.PrintConfig{TrimStart: true}) {
				return false
			}
		}

		symbolicFn := fn.symbolicValue
		if symbolicFn == nil {
			panic(errors.New("cannot Test() function against function pattern, Inox function has nil .SymbolicValue"))
		}

		return patt.symbolicValue.TestValue(symbolicFn)
	default:
		return false
	}
}

func (patt *FunctionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type IntRangePattern struct {
	NotCallablePatternMixin
	intRange IntRange
}

func NewIncludedEndIntRangePattern(start, end int64) *IntRangePattern {
	if end < start {
		panic(fmt.Errorf("failed to create int range pattern, end < start"))
	}
	return &IntRangePattern{
		intRange: NewIncludedEndIntRange(start, end),
	}
}

func NewSingleElementIntRangePattern(n int64) *IntRangePattern {
	return &IntRangePattern{
		intRange: IntRange{inclusiveEnd: true, Start: n, End: n, Step: 1},
	}
}

func (patt *IntRangePattern) Test(ctx *Context, v Value) bool {
	n, ok := v.(Int)
	if !ok {
		return false
	}

	return n >= Int(patt.intRange.Start) && n <= Int(patt.intRange.InclusiveEnd())
}

func (patt *IntRangePattern) StringPattern() (StringPattern, bool) {
	return NewIntRangeStringPattern(patt.intRange.Start, patt.intRange.InclusiveEnd(), nil), true
}

type EventPattern struct {
	NoReprMixin
	NotClonableMixin
	NotCallablePatternMixin
	ValuePattern Pattern
}

func NewEventPattern(valuePattern Pattern) *EventPattern {
	return &EventPattern{
		ValuePattern: valuePattern,
	}
}

func (patt *EventPattern) Test(ctx *Context, v Value) bool {
	e, ok := v.(*Event)
	if !ok {
		return false
	}

	if patt.ValuePattern == nil {
		return true
	}
	return patt.ValuePattern.Test(ctx, e.value)
}

func (patt *EventPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type MutationPattern struct {
	NoReprMixin
	NotClonableMixin
	NotCallablePatternMixin
	kind  MutationKind
	data0 Pattern
}

func NewMutationPattern(kind MutationKind, data0Pattern Pattern) *MutationPattern {
	return &MutationPattern{
		kind:  kind,
		data0: data0Pattern,
	}
}

func (patt *MutationPattern) Test(ctx *Context, v Value) bool {
	_, ok := v.(Mutation)
	if !ok {
		return false
	}

	panic(ErrNotImplementedYet)
	//return patt.kind == m.Kind && patt.data0.Test(ctx, m.Data0)
}

func (patt *MutationPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}
