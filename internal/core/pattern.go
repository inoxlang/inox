package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_UNION_PATTERN_FLATTENING_DEPTH      = 5
	OBJECT_CONSTRAINTS_VERIFICATION_TIMEOUT = 10 * time.Millisecond

	//Default max element count if the number of elements is not exact.
	DEFAULT_LIST_PATTERN_MAX_ELEM_COUNT = math.MaxInt32
)

var (
	ErrPatternNotCallable            = errors.New("pattern is not callable")
	ErrNoDefaultValue                = errors.New("no default value")
	ErrTooDeepUnionPatternFlattening = errors.New("union pattern flattening is too deep")
	ErrInconsistentObjectPattern     = errors.New("inconsistent object pattern")

	_ = []GroupPattern{(*NamedSegmentPathPattern)(nil)}
	_ = []DefaultValuePattern{(*ListPattern)(nil), (*TuplePattern)(nil)}
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

// The Pattern interface is implemented by Inox patterns (e.g. *ObjectPattern, *ListPattern, URLPattern, ....).
// A pattern should always be immutable.
type Pattern interface {
	Serializable
	Iterable

	//Test returns true if the argument matches the pattern.
	Test(*Context, Value) bool

	Random(ctx *Context, options ...Option) Value

	Call(values []Serializable) (Pattern, error)

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

// GroupPattern is implemented by patterns that are able to decompose matched values into groups.
// A canonical implementation example would be RegexPattern.
type GroupPattern interface {
	Pattern
	MatchGroups(*Context, Serializable) (groups map[string]Serializable, ok bool, err error)
	FindGroupMatches(*Context, Serializable, GroupMatchesFindConfig) (groups []*Object, err error)
}

// DefaultValuePattern is implemented by patterns that in most cases can provide
// a default value that matches them. ErrNoDefaultValue should be returned if it's not possible.
// If the default value is mutable a new instance of the default value should be returned each time (no reuse).
type DefaultValuePattern interface {
	Pattern
	DefaultValue(ctx *Context) (Value, error)
}

type CallBasedPatternReprMixin struct {
	Callee Pattern
	Params []Serializable
}

func (m CallBasedPatternReprMixin) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

type NamespaceMemberPatternReprMixin struct {
	NamespaceName string
	MemberName    string
}

func (m NamespaceMemberPatternReprMixin) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	repr := "%" + m.NamespaceName + "." + m.MemberName
	_, err := w.Write(utils.StringAsBytes(repr))
	return err
}

func (m NamespaceMemberPatternReprMixin) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

// A PatternNamespace represents a group of related Inox patterns, PatternNamespace implements Value.
type PatternNamespace struct {
	Patterns map[string]Pattern
}

type NotCallablePatternMixin struct {
}

func (NotCallablePatternMixin) Call(values []Serializable) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

// ExactValuePattern matches values equal to .value: .value.Equal(...) returns true.
type ExactValuePattern struct {
	value Serializable //immutable in most cases
	CallBasedPatternReprMixin

	NotCallablePatternMixin
}

func NewExactValuePattern(value Serializable) *ExactValuePattern {
	if value.IsMutable() {
		panic(ErrValueInExactPatternValueShouldBeImmutable)
	}
	return &ExactValuePattern{
		value: value,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: getDefaultNamedPattern(__VAL_PATTERN_NAME),
			Params: []Serializable{value},
		},
	}
}

// NewMostAdaptedExactPattern returns an *ExactStringPattern if value implements StringLike,
// otherwise an *ExactValuePattern is returned.
func NewMostAdaptedExactPattern(value Serializable) Pattern {
	if value.IsMutable() {
		panic(ErrValueInExactPatternValueShouldBeImmutable)
	}
	if s, ok := value.(StringLike); ok {
		return NewExactStringPattern(Str(s.GetOrBuildString()))
	}
	return NewExactValuePattern(value)
}

func (pattern *ExactValuePattern) Test(ctx *Context, v Value) bool {
	return pattern.value.Equal(ctx, v, map[uintptr]uintptr{}, 0)
}

func (pattern *ExactValuePattern) Value() Serializable {
	if pattern.value.IsMutable() {
		panic(errors.New("retrieving a mutable value is forbidden"))
	}
	return pattern.value
}

func (patt *ExactValuePattern) StringPattern() (StringPattern, bool) {
	if str, ok := patt.value.(StringLike); ok {
		stringPattern := NewExactStringPattern(Str(str.GetOrBuildString()))
		return stringPattern, true
	}
	return nil, false
}

// A TypePattern matches values implementing .Type (if .Type is an interface) or having their type equal to .Type.
// TypePattern implements Value.
type TypePattern struct {
	Type          reflect.Type
	Name          string
	SymbolicValue symbolic.Value
	RandomImpl    func(options ...Option) Value

	CallImpl         func(pattern *TypePattern, values []Serializable) (Pattern, error)
	SymbolicCallImpl func(ctx *symbolic.Context, values []symbolic.Value) (symbolic.Pattern, error)

	stringPattern         func() (StringPattern, bool)
	symbolicStringPattern func() (symbolic.StringPattern, bool)
}

func (pattern *TypePattern) Test(ctx *Context, v Value) bool {
	if pattern.Type.Kind() == reflect.Interface {
		return reflect.TypeOf(v).Implements(pattern.Type)
	}
	return pattern.Type == reflect.TypeOf(v)
}

func (patt *TypePattern) Call(values []Serializable) (Pattern, error) {
	if patt.CallImpl == nil {
		return nil, ErrPatternNotCallable
	}
	return patt.CallImpl(patt, values)
}

func (patt *TypePattern) StringPattern() (StringPattern, bool) {
	if patt.stringPattern == nil {
		return nil, false
	}

	return patt.stringPattern()
}

type UnionPattern struct {
	NotCallablePatternMixin
	node     parse.Node
	cases    []Pattern
	disjoint bool
}

func NewUnionPattern(cases []Pattern, node parse.Node) *UnionPattern {
	cases = flattenUnionPatternCases(cases, false, 0)
	return &UnionPattern{node: node, cases: cases}
}

func NewDisjointUnionPattern(cases []Pattern, node parse.Node) *UnionPattern {
	cases = flattenUnionPatternCases(cases, true, 0)
	return &UnionPattern{node: node, cases: cases, disjoint: true}
}

func flattenUnionPatternCases(cases []Pattern, disjoint bool, depth int) (results []Pattern) {
	if depth > MAX_UNION_PATTERN_FLATTENING_DEPTH {
		panic(ErrTooDeepUnionPatternFlattening)
	}

	if len(cases) == 0 {
		panic(errors.New("cases should have at least one element"))
	}

	changes := false
	results = cases

	for i, case_ := range cases {
		if union, ok := case_.(*UnionPattern); ok && union.disjoint == disjoint {
			if !changes {
				results = slices.Clone(cases[:i])
			}
			changes = true
			results = append(results, flattenUnionPatternCases(union.cases, disjoint, depth+1)...)
		} else if changes {
			results = append(results, case_)
		}
	}

	return
}

func (patt *UnionPattern) Test(ctx *Context, v Value) bool {
	if patt.disjoint {
		matchingCases := 0
		for _, case_ := range patt.cases {
			if case_.Test(ctx, v) {
				matchingCases++
				if matchingCases > 1 {
					return false
				}
			}
		}
		return matchingCases != 0
	} else {
		for _, case_ := range patt.cases {
			if case_.Test(ctx, v) {
				return true
			}
		}
		return false
	}
}

// the result should not be modified.
func (patt *UnionPattern) Cases() []Pattern {
	return patt.cases
}

func (patt *UnionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type IntersectionPattern struct {
	NotCallablePatternMixin

	node  parse.Node
	cases []Pattern
}

func simplifyIntersection(cases []Pattern) Pattern {
	if len(cases) == 1 {
		return cases[1]
	}

	casesToKeep := []int{}
	for i, case_ := range cases {
		isSuperTypeOfAllOtherCases := true

		for j, otherCase := range cases {
			if i == j {
				continue
			}
			if !isObviousSubType(otherCase, case_) {
				isSuperTypeOfAllOtherCases = false
				break
			}
		}

		if !isSuperTypeOfAllOtherCases {
			casesToKeep = append(casesToKeep, i)
		}
	}

	var remainingCases []Pattern

	for _, remainingCaseIndex := range casesToKeep {
		remainingCases = append(remainingCases, cases[remainingCaseIndex])
	}

	if len(remainingCases) == 1 {
		return remainingCases[0]
	}

	if len(remainingCases) == 0 {
		panic(ErrUnreachable)
	}

	return NewIntersectionPattern(remainingCases, nil)
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

// An ObjectPattern represents a pattern matching Inox objects (e.g. {a: 1}), ObjectPattern implements Value.
type ObjectPattern struct {
	NotCallablePatternMixin
	entries                 []ObjectPatternEntry //sorted by name in lexicographic order
	dependentKeys           []string             //entries with dependencies
	optionalEntryCount      int32
	inexact                 bool //if true the matched object can have additional properties
	complexPropertyPatterns []*ComplexPropertyConstraint
}

type PropertyDependencies struct {
	RequiredKeys []string
	Pattern      Pattern //object or record pattern
}

type ObjectPatternEntry struct {
	Name       string
	Pattern    Pattern
	IsOptional bool

	//an entry is considered to have dependencies if .RequiredKeys > 0 or .Pattern is not nil.
	Dependencies PropertyDependencies
}

func NewInexactObjectPattern(entries []ObjectPatternEntry) *ObjectPattern {
	p := NewObjectPattern(true, entries)
	return p
}

func NewExactObjectPattern(entries []ObjectPatternEntry) *ObjectPattern {
	return NewObjectPattern(false, entries)
}

func NewObjectPattern(inexact bool, entries []ObjectPatternEntry) *ObjectPattern {
	p := &ObjectPattern{inexact: inexact, entries: entries}
	p.init()
	p.assertIsConsistent()
	return p
}

func (patt *ObjectPattern) init() {
	entries := patt.entries
	//Sort entries by lexicographic order.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	//Find what entries are optional and what entries are dependent on other entries.
	patt.optionalEntryCount = 0

	var dependentKeys []string
	for _, entry := range entries {
		if entry.IsOptional {
			patt.optionalEntryCount++
		}
		if entry.Dependencies.Pattern != nil || len(entry.Dependencies.RequiredKeys) > 0 {
			dependentKeys = append(dependentKeys, entry.Name)
		}
	}
	//Sort dependent keys by lexicographic order.
	sort.Strings(dependentKeys)
	patt.dependentKeys = dependentKeys
}

func (patt *ObjectPattern) isConsistent() bool {
	//check that all dependent keys are present in the entries

	for _, dependentKey := range patt.dependentKeys {
		if !patt.HasRequiredOrOptionalEntry(dependentKey) {
			return false
		}
	}
	return true
}

func (patt *ObjectPattern) assertIsConsistent() {
	if !patt.isConsistent() {
		panic(ErrInconsistentObjectPattern)
	}
}

func (patt *ObjectPattern) WithConstraints(constraints []*ComplexPropertyConstraint) *ObjectPattern {
	newPatt := *patt
	newPatt.complexPropertyPatterns = constraints
	return &newPatt
}

func (patt *ObjectPattern) Test(ctx *Context, v Value) bool {
	obj, ok := v.(*Object)
	if !ok {
		return false
	}
	if !patt.inexact && patt.optionalEntryCount == 0 && len(obj.keys) != len(patt.entries) {
		return false
	}

	propNames := obj.PropertyNames(ctx)

	//check dependencies
	for _, entry := range patt.entries {
		deps := entry.Dependencies
		for _, dep := range deps.RequiredKeys {
			if !slices.Contains(propNames, dep) {
				return false
			}
		}
		if deps.Pattern != nil && deps.Pattern.Test(ctx, v) {
			return false
		}
	}

	for _, entry := range patt.entries {
		if !obj.HasProp(ctx, entry.Name) {
			if entry.IsOptional {
				continue
			}
			return false
		}
		value := obj.Prop(ctx, entry.Name)
		if !entry.Pattern.Test(ctx, value) {
			return false
		}
	}

	//If the pattern is exact check that there are no additional properties.
	if !patt.inexact {
		for _, propName := range obj.PropertyNames(ctx) {
			if !patt.HasRequiredOrOptionalEntry(propName) {
				return false
			}
		}
	}

	if len(patt.complexPropertyPatterns) == 0 {
		return true
	}

	parentCtx, cancel := context.WithTimeout(ctx, OBJECT_CONSTRAINTS_VERIFICATION_TIMEOUT)
	defer cancel()

	//TODO: optimize based on what operations are performed during the check
	//TODO: set max CPU time to 1ms and timeout to 200ms

	state := NewTreeWalkState(NewContext(ContextConfig{
		DoNotSpawnDoneGoroutine: true,
		ParentStdLibContext:     parentCtx,
	}))
	defer state.Global.Ctx.CancelGracefully()
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

func (patt *ObjectPattern) ForEachEntry(fn func(entry ObjectPatternEntry) error) error {
	for _, entry := range patt.entries {
		err := fn(entry)
		if err != nil {
			return err
		}
	}
	return nil
}

func (patt *ObjectPattern) EntryCount() int {
	return len(patt.entries)
}

type ObjectPatternEntriesHelper []ObjectPatternEntry

func (h ObjectPatternEntriesHelper) Entry(name string) (pattern Pattern, optional bool, yes bool) {
	entry, ok := h.CompleteEntry(name)
	if !ok {
		return nil, false, false
	}
	return entry.Pattern, entry.IsOptional, true
}

func (h ObjectPatternEntriesHelper) HasRequiredOrOptionalEntry(name string) bool {
	_, ok := h.CompleteEntry(name)
	return ok
}

func (h ObjectPatternEntriesHelper) CompleteEntry(name string) (ObjectPatternEntry, bool) {
	index, ok := slices.BinarySearchFunc(h, name, func(ope ObjectPatternEntry, s string) int {
		return strings.Compare(ope.Name, s)
	})

	if !ok {
		return ObjectPatternEntry{}, false
	}
	return h[index], true
}

func (h ObjectPatternEntriesHelper) setEntry(name string, entry ObjectPatternEntry) ObjectPatternEntriesHelper {
	index, ok := slices.BinarySearchFunc(h, name, func(ope ObjectPatternEntry, s string) int {
		return strings.Compare(ope.Name, s)
	})

	if ok {
		h[index] = entry
		return h
	} else {
		return slices.Insert(h, index, entry)
	}
}

func (patt *ObjectPattern) Entry(name string) (pattern Pattern, optional bool, yes bool) {
	return ObjectPatternEntriesHelper(patt.entries).Entry(name)
}

func (patt *ObjectPattern) HasRequiredOrOptionalEntry(name string) bool {
	return ObjectPatternEntriesHelper(patt.entries).HasRequiredOrOptionalEntry(name)
}

func (patt *ObjectPattern) CompleteEntry(name string) (ObjectPatternEntry, bool) {
	return ObjectPatternEntriesHelper(patt.entries).CompleteEntry(name)
}

// A RecordPattern represents a pattern matching Inox records (e.g. #{a: 1}), RecordPattern implements Value.
type RecordPattern struct {
	NotCallablePatternMixin
	entries            []RecordPatternEntry
	optionalEntryCount int32
	inexact            bool //if true the matched object can have additional properties
}

type RecordPatternEntry struct {
	Name       string
	Pattern    Pattern
	IsOptional bool
}

func NewInexactRecordPattern(entries []RecordPatternEntry) *RecordPattern {
	p := NewRecordPattern(true, entries)
	return p
}

func NewExactRecordPattern(entries []RecordPatternEntry) *RecordPattern {
	return NewRecordPattern(false, entries)
}

func NewRecordPattern(inexact bool, entries []RecordPatternEntry) *RecordPattern {
	p := &RecordPattern{inexact: inexact, entries: entries}
	p.init()
	return p
}

func (patt *RecordPattern) init() {
	entries := patt.entries

	//Sort entries by lexicographic order.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	patt.optionalEntryCount = 0

	for _, entry := range entries {
		if entry.IsOptional {
			patt.optionalEntryCount++
		}
	}
}

func (patt *RecordPattern) Test(ctx *Context, v Value) bool {
	rec, ok := v.(*Record)
	if !ok {
		return false
	}
	if !patt.inexact && patt.optionalEntryCount == 0 && len(rec.keys) != len(patt.entries) {
		return false
	}

	for _, entry := range patt.entries {
		if !rec.HasProp(ctx, entry.Name) {
			if entry.IsOptional {
				continue
			}
			return false
		}
		value := rec.Prop(ctx, entry.Name)
		if !entry.Pattern.Test(ctx, value) {
			return false
		}
	}

	//If the pattern is exact check that there are no additional properties.
	if !patt.inexact {
		for _, propName := range rec.PropertyNames(ctx) {
			if !patt.HasRequiredOrOptionalEntry(propName) {
				return false
			}
		}
	}
	return true
}

func (patt *RecordPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt *RecordPattern) ForEachEntry(fn func(entry RecordPatternEntry) error) error {
	for _, entry := range patt.entries {
		err := fn(entry)
		if err != nil {
			return err
		}
	}
	return nil
}

func (patt *RecordPattern) HasRequiredOrOptionalEntry(name string) bool {
	_, ok := patt.CompleteEntry(name)
	return ok
}

func (patt *RecordPattern) Entry(name string) (pattern Pattern, optional bool, yes bool) {
	entry, ok := patt.CompleteEntry(name)
	if !ok {
		return nil, false, false
	}
	return entry.Pattern, entry.IsOptional, true
}

func (patt *RecordPattern) CompleteEntry(name string) (RecordPatternEntry, bool) {
	index, ok := slices.BinarySearchFunc(patt.entries, name, func(ope RecordPatternEntry, s string) int {
		return strings.Compare(ope.Name, s)
	})

	if !ok {
		return RecordPatternEntry{}, false
	}
	return patt.entries[index], true
}

type ComplexPropertyConstraint struct {
	NotCallablePatternMixin
	Properties []string
	Expr       parse.Node
}

// A ListPattern represents a pattern matching Inox lists (e.g. [1, 2]), ListPattern implements Value.
type ListPattern struct {
	NotCallablePatternMixin
	elementPatterns       []Pattern
	generalElementPattern Pattern

	containedElement    Pattern
	minElemCountPlusOne int //zero if not set
	maxElemCount        int //ignored if minElemCountPlusOne <= 0
}

func NewListPatternOf(generalElementPattern Pattern) *ListPattern {
	return &ListPattern{generalElementPattern: generalElementPattern}
}

func NewListPattern(elementPatterns []Pattern) *ListPattern {
	if elementPatterns == nil {
		elementPatterns = []Pattern{}
	}
	return &ListPattern{elementPatterns: elementPatterns}
}

func NewListPatternVariadic(elementPatterns ...Pattern) *ListPattern {
	if elementPatterns == nil {
		elementPatterns = []Pattern{}
	}
	return &ListPattern{elementPatterns: elementPatterns}
}

// WithMinMaxElements return a new version of the pattern with the given minimum count constraints.
func (patt *ListPattern) WithMinElements(minCount int) *ListPattern {
	if minCount < 0 {
		panic(errors.New("minCount should not be negative"))
	}

	newPattern := *patt
	newPattern.minElemCountPlusOne = minCount + 1
	if patt.minElemCountPlusOne == 0 {
		newPattern.maxElemCount = DEFAULT_LIST_PATTERN_MAX_ELEM_COUNT
	}

	return &newPattern
}

// WithMinMaxElements return a new version of the pattern with the given minimum & maximum element count constraints.
func (patt *ListPattern) WithMinMaxElements(minCount, maxCount int) *ListPattern {
	if minCount > maxCount {
		panic(errors.New("minCount should be less or equal to maxCount"))
	}

	newPattern := *patt
	newPattern.minElemCountPlusOne = minCount + 1
	newPattern.maxElemCount = maxCount

	return &newPattern
}

// WithMinMaxElements return a new version of the pattern that expects at least one occurrence of element.
func (patt *ListPattern) WithElement(element Pattern) *ListPattern {
	if element == nil {
		panic(errors.New("element should not be nil"))
	}
	newPattern := *patt
	newPattern.containedElement = element
	return &newPattern
}

func (patt ListPattern) Test(ctx *Context, v Value) bool {
	list, ok := v.(*List)
	if !ok {
		return false
	}

	length := list.Len()

	if length < patt.MinElementCount() || length > patt.MaxElementCount() {
		return false
	}

	// if patt.containedElement is nil we assume that we already found the contained element
	containedElementFound := patt.containedElement == nil

	if patt.generalElementPattern != nil {
		for i := 0; i < length; i++ {
			e := list.At(ctx, i)

			if !patt.generalElementPattern.Test(ctx, e) {
				return false
			}

			if !containedElementFound && patt.containedElement.Test(ctx, e) {
				containedElementFound = true
			}
		}
		return containedElementFound
	}

	if length != len(patt.elementPatterns) {
		return false
	}

	for i, elementPattern := range patt.elementPatterns {
		e := list.At(ctx, i)

		if !ok || !elementPattern.Test(ctx, list.At(ctx, i)) {
			return false
		}

		if !containedElementFound && patt.containedElement.Test(ctx, e) {
			containedElementFound = true
		}
	}
	return containedElementFound
}

func (patt *ListPattern) ExactElementCount() (int, bool) {
	if patt.elementPatterns != nil {
		return len(patt.elementPatterns), true
	}
	return 0, false
}

// MinElementCount returns the minimum number of elements if
// it is defined. The function panics otherwise. The minimum
// is not defined for list patterns with an exact number of elements.
func (patt *ListPattern) MinElementCount() int {
	if patt.minElemCountPlusOne > 0 {
		if patt.elementPatterns != nil {
			panic(ErrUnreachable)
		}
		return patt.minElemCountPlusOne - 1
	}
	if patt.elementPatterns == nil {
		return 0
	}
	return len(patt.elementPatterns)
}

// MaxElementCount returns the maximum number of elements if
// it is defined. The function panics otherwise. The maximum
// is not defined for most patterns with an exact number of elements.
func (patt *ListPattern) MaxElementCount() int {
	if patt.minElemCountPlusOne > 0 {
		if patt.elementPatterns != nil {
			panic(ErrUnreachable)
		}
		return patt.maxElemCount
	}
	if patt.elementPatterns == nil {
		return DEFAULT_LIST_PATTERN_MAX_ELEM_COUNT
	}
	return len(patt.elementPatterns)
}

func (patt *ListPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt *ListPattern) DefaultValue(ctx *Context) (Value, error) {
	if patt.generalElementPattern != nil {
		//TODO: add elem type
		return NewWrappedValueList(), nil
	}
	return nil, ErrNoDefaultValue
}

func (patt *ListPattern) ElementPatternAt(i int) (Pattern, bool) {
	if patt.elementPatterns != nil {
		if i < 0 || i >= len(patt.elementPatterns) {
			return nil, false
		}
		return patt.elementPatterns[i], true
	}
	return patt.generalElementPattern, true
}

// A TuplePattern represents a pattern matching Inox tuples (e.g. #[1, 2]), ListPattern TuplePattern Value.
type TuplePattern struct {
	NotCallablePatternMixin
	elementPatterns       []Pattern
	generalElementPattern Pattern
}

func NewTuplePatternOf(generalElementPattern Pattern) *TuplePattern {
	return &TuplePattern{generalElementPattern: generalElementPattern}
}

func NewTuplePattern(elementPatterns []Pattern) *TuplePattern {
	if elementPatterns == nil {
		elementPatterns = []Pattern{}
	}
	return &TuplePattern{elementPatterns: elementPatterns}
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

func (patt *TuplePattern) ElementPatternAt(i int) (Pattern, bool) {
	if patt.elementPatterns != nil {
		if i < 0 || i >= len(patt.elementPatterns) {
			return nil, false
		}
		return patt.elementPatterns[i], true
	}
	return patt.generalElementPattern, true
}

func (patt *TuplePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt *TuplePattern) DefaultValue(ctx *Context) (Value, error) {
	if patt.generalElementPattern != nil {
		return NewTuple(nil), nil
	}
	return nil, ErrNoDefaultValue
}

func (patt *TuplePattern) ExactElementCount() (int, bool) {
	if patt.elementPatterns != nil {
		return len(patt.elementPatterns), true
	}
	return 0, false
}

type OptionPattern struct {
	NotCallablePatternMixin
	name  string
	value Pattern
}

func (patt OptionPattern) Test(ctx *Context, v Value) bool {
	opt, ok := v.(Option)
	return ok && opt.Name == patt.name && patt.value.Test(ctx, opt.Value)
}

func (patt *OptionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// A DifferencePattern represents a pattern that matches the same values as a 'base' pattern except
// all values matched by a 'removed' pattern.
type DifferencePattern struct {
	NotCallablePatternMixin
	base    Pattern
	removed Pattern
}

func NewDifferencePattern(base, removed Pattern) *DifferencePattern {
	return &DifferencePattern{
		base:    base,
		removed: removed,
	}
}

func (patt *DifferencePattern) Test(ctx *Context, v Value) bool {
	return patt.base.Test(ctx, v) && !patt.removed.Test(ctx, v)
}

func (patt *DifferencePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// An OptionalPattern represents a pattern that matches the nil value in additional to the same values as an underlying pattern.
// Optional pattern expressions (e.g. `%int?`) evaluate to an optional pattern.
type OptionalPattern struct {
	NotCallablePatternMixin
	pattern Pattern
}

func NewOptionalPattern(ctx *Context, pattern Pattern) (*OptionalPattern, error) {
	if pattern.Test(ctx, Nil) {
		return nil, errors.New("cannot create optional pattern with pattern that already matches nil")
	}
	return &OptionalPattern{
		pattern: pattern,
	}, nil
}

func (patt *OptionalPattern) Test(ctx *Context, v Value) bool {
	if _, ok := v.(NilT); ok {
		return true
	}
	return patt.pattern.Test(ctx, v)
}

func (patt *OptionalPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// A FunctionPattern represents a pattern that matches that either matches any function or
// functions with certain parameters and return types.
// Inox's function pattern literals (e.g. fn() int) evaluate to a function pattern.
type FunctionPattern struct {
	node      *parse.FunctionPatternExpression //if nil, matches any function
	nodeChunk *parse.Chunk

	symbolicValue *symbolic.FunctionPattern //used for checking functions

	NotCallablePatternMixin
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

		panic(errors.New("testing a Go function against a function pattern is not supported yet"))

	case *InoxFunction:

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

		symbolicFn := fn.symbolicValue
		if symbolicFn == nil {
			panic(errors.New("cannot Test() function against function pattern, Inox function has nil .SymbolicValue"))
		}

		return patt.symbolicValue.TestValue(symbolicFn, symbolic.RecTestCallState{})
	default:
		return false
	}
}

func (patt *FunctionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// An IntRangePattern represents a pattern matching integers in a given range.
type IntRangePattern struct {
	intRange        IntRange
	multipleOf      Int    //ignored if <= 0
	multipleOfFloat *Float //can be nil

	CallBasedPatternReprMixin
	NotCallablePatternMixin
}

// multipleOf is ignored if not greater than zero
func NewIncludedEndIntRangePattern(start, end int64, multipleOf int64) *IntRangePattern {
	if end < start {
		panic(fmt.Errorf("failed to create int range pattern, end < start"))
	}

	if multipleOf <= 0 {
		multipleOf = 0
	}

	range_ := NewIncludedEndIntRange(start, end)
	return &IntRangePattern{
		intRange:   range_,
		multipleOf: Int(multipleOf),
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: INT_PATTERN,
			Params: []Serializable{range_},
		},
	}
}

// multipleOf is ignored if not greater than zero
func NewIntRangePattern(intRange IntRange, multipleOf int64) *IntRangePattern {
	if intRange.end < intRange.start {
		panic(fmt.Errorf("failed to create int range pattern, end < start"))
	}

	if multipleOf <= 0 {
		multipleOf = 0
	}

	return &IntRangePattern{
		intRange:   intRange,
		multipleOf: Int(multipleOf),
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: INT_PATTERN,
			Params: []Serializable{intRange},
		},
	}
}

func NewIntRangePatternFloatMultiple(intRange IntRange, multipleOf Float) *IntRangePattern {
	if intRange.end < intRange.start {
		panic(fmt.Errorf("failed to create int range pattern, end < start"))
	}

	pattern := &IntRangePattern{
		intRange: intRange,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: INT_PATTERN,
			Params: []Serializable{intRange},
		},
	}

	if multipleOf <= 0 {
		pattern.multipleOfFloat = &multipleOf
	}

	return pattern
}

func NewSingleElementIntRangePattern(n int64) *IntRangePattern {
	range_ := IntRange{inclusiveEnd: true, start: n, end: n, step: 1}
	return &IntRangePattern{
		intRange: range_,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: INT_PATTERN,
			Params: []Serializable{range_},
		},
	}
}

func (patt *IntRangePattern) Test(ctx *Context, v Value) bool {
	n, ok := v.(Int)
	if !ok {
		return false
	}
	return patt.Includes(ctx, n)
}

func (patt *IntRangePattern) Includes(ctx *Context, n Int) bool {
	if n < Int(patt.intRange.start) ||
		n > Int(patt.intRange.InclusiveEnd()) {
		return false
	}

	if patt.multipleOfFloat != nil {
		float := *patt.multipleOfFloat
		res := Float(n) / float

		return utils.IsWholeInt64(res)
	} else {
		return patt.multipleOf <= 0 || (n%patt.multipleOf) == 0
	}
}

func (patt *IntRangePattern) StringPattern() (StringPattern, bool) {
	if patt.multipleOf > 0 {
		return nil, false
	}
	return NewIntRangeStringPattern(patt.intRange.start, patt.intRange.InclusiveEnd(), nil), true
}

func (patt *IntRangePattern) Range() IntRange {
	return patt.intRange
}

func (patt *IntRangePattern) HasMultipleOfConstraint() bool {
	return patt.multipleOf > 0 || patt.multipleOfFloat != nil
}

// An FloatRangePattern represents a pattern matching floats in a given range.
type FloatRangePattern struct {
	floatRange FloatRange
	multipleOf Float

	CallBasedPatternReprMixin
	NotCallablePatternMixin
}

// multipleOf is ignored if not greater than zero
func NewFloatRangePattern(floatRange FloatRange, multipleOf float64) *FloatRangePattern {
	if floatRange.end < floatRange.start {
		panic(fmt.Errorf("failed to create float range pattern, end < start"))
	}

	if multipleOf <= 0 {
		multipleOf = 0
	}

	return &FloatRangePattern{
		floatRange: floatRange,
		multipleOf: Float(multipleOf),
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: FLOAT_PATTERN,
			Params: []Serializable{floatRange},
		},
	}
}

func NewSingleElementFloatRangePattern(n float64) *FloatRangePattern {
	range_ := FloatRange{inclusiveEnd: true, start: n, end: n}
	return &FloatRangePattern{
		floatRange: range_,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: FLOAT_PATTERN,
			Params: []Serializable{range_},
		},
	}
}

func (patt *FloatRangePattern) Test(ctx *Context, v Value) bool {
	n, ok := v.(Float)
	if !ok {
		return false
	}

	if n < Float(patt.floatRange.start) ||
		n > Float(patt.floatRange.InclusiveEnd()) {
		return false
	}

	if patt.multipleOf <= 0 {
		return true
	}

	res := n / patt.multipleOf
	return utils.IsWholeInt64(res)
}

func (patt *FloatRangePattern) StringPattern() (StringPattern, bool) {
	if patt.multipleOf > 0 {
		return nil, false
	}

	start := patt.floatRange.start
	end := patt.floatRange.InclusiveEnd()
	switch {
	case start == -math.MaxFloat64 && end == math.MaxFloat64,
		start == -math.MaxFloat64 && end == 0,
		start == 0 && end == math.MaxFloat64:
		return NewFloatRangeStringPattern(start, end, nil), true
	default:
		return nil, false
	}
}

func (patt *FloatRangePattern) Range() FloatRange {
	return patt.floatRange
}

type EventPattern struct {
	ValuePattern Pattern
	CallBasedPatternReprMixin

	NotCallablePatternMixin
}

func NewEventPattern(valuePattern Pattern) *EventPattern {
	return &EventPattern{
		ValuePattern: valuePattern,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: getDefaultNamedPattern("event"),
			Params: []Serializable{valuePattern},
		},
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
	kind  MutationKind
	data0 Pattern
	CallBasedPatternReprMixin

	NotCallablePatternMixin
}

func NewMutationPattern(kind MutationKind, data0Pattern Pattern) *MutationPattern {
	return &MutationPattern{
		kind:  kind,
		data0: data0Pattern,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: getDefaultNamedPattern("mutation"),
			Params: []Serializable{Identifier(kind.String()), data0Pattern},
		},
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

func isFloatPattern(p Pattern) bool {
	switch pattern := p.(type) {
	case *TypePattern:
		if pattern.Type == FLOAT64_TYPE {
			return true
		}
	case *FloatRangePattern:
		return true
	}
	return false
}

func isIntPattern(p Pattern) bool {
	switch pattern := p.(type) {
	case *TypePattern:
		if pattern.Type == INT_TYPE {
			return true
		}
	case *IntRangePattern:
		return true
	}
	return false
}

func isTypePattern(p Pattern, typ reflect.Type) bool {
	switch pattern := p.(type) {
	case *TypePattern:
		return pattern.Type == typ
	}
	return false
}

func isObviousSubType(p Pattern, superType Pattern) bool {
	switch pattern := p.(type) {
	case *TypePattern:
		otherTypePattern, ok := superType.(*TypePattern)
		if !ok {
			return false
		}

		return pattern.Type.AssignableTo(otherTypePattern.Type)
	case *IntRangePattern:
		return isTypePattern(superType, INT_TYPE)
	case *FloatRangePattern:
		return isTypePattern(superType, FLOAT64_TYPE)
	case StringPattern:
		return isTypePattern(superType, STRING_TYPE) || isTypePattern(superType, STR_LIKE_INTERFACE_TYPE)
	case *ObjectPattern:
		return isTypePattern(superType, OBJECT_TYPE)
	case *RecordPattern:
		return isTypePattern(superType, RECORD_TYPE)
	case *ListPattern:
		return isTypePattern(superType, LIST_PTR_TYPE)
	case *TuplePattern:
		return isTypePattern(superType, TUPLE_TYPE)
	}
	return false
}
