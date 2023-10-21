package symbolic

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/inoxlang/inox/internal/commonfmt"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

var (
	DICTIONARY_PROPNAMES = []string{"get", "set"}
	LIST_PROPNAMES       = []string{"append"}

	ANY_INDEXABLE    = &AnyIndexable{}
	ANY_ARRAY        = NewArrayOf(ANY_SERIALIZABLE)
	ANY_TUPLE        = NewTupleOf(ANY_SERIALIZABLE)
	ANY_OBJ          = &Object{}
	ANY_READONLY_OBJ = &Object{readonly: true}
	ANY_REC          = &Record{}
	ANY_NAMESPACE    = NewAnyNamespace()
	ANY_DICT         = NewAnyDictionary()
	ANY_KEYLIST      = NewAnyKeyList()

	EMPTY_OBJECT          = NewEmptyObject()
	EMPTY_READONLY_OBJECT = NewEmptyReadonlyObject()
	EMPTY_LIST            = NewList()
	EMPTY_READONLY_LIST   = NewReadonlyList()
	EMPTY_TUPLE           = NewTuple()

	_ = []Indexable{
		(*String)(nil), (*Array)(nil), (*List)(nil), (*Tuple)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil), (*Object)(nil), (*IntRange)(nil),
		(*AnyStringLike)(nil), (*AnyIndexable)(nil),
	}

	_ = []Sequence{
		(*String)(nil), (*Array)(nil), (*List)(nil), (*Tuple)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil),
	}

	_ = []IProps{(*Object)(nil), (*Record)(nil), (*Namespace)(nil), (*Dictionary)(nil), (*List)(nil)}
	_ = []InexactCapable{(*Object)(nil), (*Record)(nil)}
)

// An Indexable represents a symbolic Indexable.
type Indexable interface {
	Iterable
	element() Value
	elementAt(i int) Value
	KnownLen() int
	HasKnownLen() bool
}

type InexactCapable interface {
	Value

	//TestExact should behave like Test() at the only difference that inexactness should be ignored.
	//For example an inexact object should not match an another object that has additional properties.
	TestExact(v Value) bool
}

// An Array represents a symbolic Array.
type Array struct {
	elements       []Value
	generalElement Value
}

func NewArray(elements ...Value) *Array {
	if elements == nil {
		elements = []Value{}
	}
	return &Array{elements: elements}
}

func NewArrayOf(generalElement Value) *Array {
	return &Array{generalElement: generalElement}
}

func (a *Array) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherArray, ok := v.(*Array)
	if !ok {
		return false
	}

	if a.elements == nil {
		if otherArray.elements == nil {
			return a.generalElement.Test(otherArray.generalElement, state)
		}

		for _, elem := range otherArray.elements {
			if !a.generalElement.Test(elem, state) {
				return false
			}
		}
		return true
	}

	if len(a.elements) != len(otherArray.elements) || otherArray.elements == nil {
		return false
	}

	for i, e := range a.elements {
		if !e.Test(otherArray.elements[i], state) {
			return false
		}
	}
	return true
}

func (a *Array) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if a.elements != nil {
		length := a.KnownLen()

		if w.Depth > config.MaxDepth && length > 0 {
			w.WriteName("Array(...)")
			return
		}

		w.WriteName("Array(")

		indentCount := w.ParentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)
		printIndices := !config.Compact && length > 10

		for i := 0; i < length; i++ {
			v := a.elements[i]

			if !config.Compact {
				w.WriteLFCR()
				w.WriteBytes(indent)

				//index
				if printIndices {
					if config.Colorize {
						w.WriteBytes(config.Colors.DiscreteColor)
					}
					if i < 10 {
						w.WriteByte(' ')
					}
					w.WriteString(strconv.FormatInt(int64(i), 10))
					w.WriteBytes(COLON_SPACE)
					if config.Colorize {
						w.WriteBytes(ANSI_RESET_SEQUENCE)
					}
				}
			}

			//element
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == length-1

			if !isLastEntry {
				w.WriteBytes(COMMA_SPACE)
			}

		}

		var end []byte
		if !config.Compact && length > 0 {
			end = append(end, '\n', '\r')
			end = append(end, bytes.Repeat(config.Indent, w.Depth)...)
		}
		end = append(end, ')')

		w.WriteBytes(end)
		return
	}

	w.WriteName("array(")
	a.generalElement.PrettyPrint(w, config)
	w.WriteString(")")
}

func (a *Array) HasKnownLen() bool {
	return a.elements != nil
}

func (a *Array) KnownLen() int {
	if a.elements == nil {
		panic("cannot get length of a symbolic array with no known length")
	}

	return len(a.elements)
}

func (a *Array) element() Value {
	if a.elements != nil {
		if len(a.elements) == 0 {
			return ANY
		}
		return joinValues(a.elements)
	}
	return ANY
}

func (a *Array) elementAt(i int) Value {
	if a.elements != nil {
		if len(a.elements) == 0 || i >= len(a.elements) {
			return ANY // return "never" ?
		}
		return a.elements[i]
	}
	return ANY
}

func (a *Array) slice(start, end *Int) Sequence {
	return ANY_ARRAY
}

func (a *Array) IteratorElementKey() Value {
	return ANY_INT
}

func (a *Array) IteratorElementValue() Value {
	return a.element()
}

func (*Array) WidestOfType() Value {
	return ANY_ARRAY
}

// A List represents a symbolic List.
type List struct {
	elements       []Serializable
	generalElement Serializable
	readonly       bool

	SerializableMixin
	PseudoClonableMixin
	UnassignablePropsMixin
}

func NewList(elements ...Serializable) *List {
	if elements == nil {
		elements = []Serializable{}
	}
	return &List{elements: elements}
}

func NewReadonlyList(elements ...Serializable) *List {
	list := NewList(elements...)
	list.readonly = true
	return list
}

func NewListOf(generalElement Serializable) *List {
	return &List{generalElement: generalElement}
}

func (list *List) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherList, ok := v.(*List)
	if !ok || list.readonly != otherList.readonly {
		return false
	}

	if list.elements == nil {
		if otherList.elements == nil {
			return list.generalElement.Test(otherList.generalElement, state)
		}

		for _, elem := range otherList.elements {
			if !list.generalElement.Test(elem, state) {
				return false
			}
		}
		return true
	}

	if len(list.elements) != len(otherList.elements) || otherList.elements == nil {
		return false
	}

	for i, e := range list.elements {
		if !e.Test(otherList.elements[i], state) {
			return false
		}
	}
	return true
}

func (list *List) IsConcretizable() bool {
	//TODO: support constraints
	if list.generalElement != nil {
		return false
	}

	for _, elem := range list.elements {
		if potentiallyConcretizable, ok := elem.(PotentiallyConcretizable); !ok || !potentiallyConcretizable.IsConcretizable() {
			return false
		}
	}

	return true
}

func (list *List) Concretize(ctx ConcreteContext) any {
	if !list.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concreteElements := make([]any, len(list.elements))
	for i, e := range list.elements {
		concreteElements[i] = utils.Must(Concretize(e, ctx))
	}
	return extData.ConcreteValueFactories.CreateList(concreteElements)
}

func (list *List) IsReadonly() bool {
	return list.readonly
}

func (list *List) ToReadonly() (PotentiallyReadonly, error) {
	if list.readonly {
		return list, nil
	}

	if list.generalElement != nil {
		readonly := NewListOf(list.generalElement)
		readonly.readonly = true
		return readonly, nil
	}

	elements := make([]Serializable, len(list.elements))

	for i, e := range list.elements {
		if !e.IsMutable() {
			elements[i] = e
			continue
		}
		potentiallyReadonly, ok := e.(PotentiallyReadonly)
		if !ok {
			return nil, FmtElementError(i, ErrNotConvertibleToReadonly)
		}
		readonly, err := potentiallyReadonly.ToReadonly()
		if err != nil {
			return nil, FmtElementError(i, err)
		}
		elements[i] = readonly.(Serializable)
	}

	readonly := NewList(elements...)
	readonly.readonly = true
	return readonly, nil
}

func (list *List) Static() Pattern {
	if list.generalElement != nil {
		return NewListPatternOf(&TypePattern{val: list.generalElement})
	}

	var elements []Value
	for _, e := range list.elements {
		elements = append(elements, getStatic(e).SymbolicValue())
	}

	if len(elements) == 0 {
		return NewListPatternOf(&TypePattern{val: ANY_SERIALIZABLE})
	}

	elem := AsSerializableChecked(joinValues(elements))
	return NewListPatternOf(&TypePattern{val: elem})
}

func (list *List) Prop(name string) Value {
	switch name {
	case "append":
		return WrapGoMethod(list.Append)
	default:
		panic(FormatErrPropertyDoesNotExist(name, list))
	}
}

func (list *List) PropertyNames() []string {
	return LIST_PROPNAMES
}

func (list *List) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if list.readonly {
		w.WriteName("readonly ")
	}

	if list.elements != nil {
		length := list.KnownLen()

		if w.Depth > config.MaxDepth && length > 0 {
			w.WriteString("[(...)]")
			return
		}

		w.WriteByte('[')

		indentCount := w.ParentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)
		printIndices := !config.Compact && length > 10

		for i := 0; i < length; i++ {
			v := list.elements[i]

			if !config.Compact {
				w.WriteLFCR()
				w.WriteBytes(indent)

				//index
				if printIndices {
					if config.Colorize {
						w.WriteBytes(config.Colors.DiscreteColor)
					}
					if i < 10 {
						w.WriteByte(' ')
					}
					w.WriteBytes(utils.StringAsBytes(strconv.FormatInt(int64(i), 10)))
					w.WriteBytes(COLON_SPACE)
					if config.Colorize {
						w.WriteBytes(ANSI_RESET_SEQUENCE)
					}
				}
			}

			//element
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == length-1

			if !isLastEntry {
				w.WriteBytes(COMMA_SPACE)
			}

		}

		var end []byte
		if !config.Compact && length > 0 {
			end = append(end, '\n', '\r')
			end = append(end, bytes.Repeat(config.Indent, w.Depth)...)
		}
		end = append(end, ']')

		w.WriteBytes(end)
		return
	}
	w.WriteString("[]")
	list.generalElement.PrettyPrint(w, config)
}

func (l *List) HasKnownLen() bool {
	return l.elements != nil
}

func (l *List) KnownLen() int {
	if l.elements == nil {
		panic("cannot get length of a symbolic list with no known length")
	}

	return len(l.elements)
}

func (l *List) element() Value {
	if l.elements != nil {
		if len(l.elements) == 0 {
			return ANY_SERIALIZABLE
		}
		return AsSerializableChecked(joinValues(SerializablesToValues(l.elements)))
	}
	return l.generalElement
}

func (l *List) elementAt(i int) Value {
	if l.elements != nil {
		if len(l.elements) == 0 || i >= len(l.elements) {
			return ANY // return "never" ?
		}
		return l.elements[i]
	}
	return l.generalElement
}

func (l *List) Contains(value Value) (bool, bool) {
	if l.elements == nil {
		if l.generalElement.Test(value, RecTestCallState{}) {
			return false, true
		}
		return false, false
	}

	possible := false

	for _, e := range l.elements {
		if e.Test(value, RecTestCallState{}) {
			possible = true
			if value.Test(e, RecTestCallState{}) {
				return true, true
			}
		}
	}
	return false, possible
}

func (l *List) IteratorElementKey() Value {
	return ANY_INT
}

func (l *List) IteratorElementValue() Value {
	return l.element()
}

func (l *List) WidestOfType() Value {
	return WIDEST_LIST_PATTERN
}

func (l *List) slice(start, end *Int) Sequence {
	if l.HasKnownLen() {
		return &List{generalElement: ANY_SERIALIZABLE}
	}
	return &List{
		generalElement: l.generalElement,
	}
}

func (l *List) set(ctx *Context, i *Int, v Value) {
	//TODO
}

func (l *List) SetSlice(ctx *Context, start, end *Int, v Sequence) {
	//TODO
}

func (l *List) insertElement(ctx *Context, v Value, i *Int) {
	//TODO
}

func (l *List) removePosition(ctx *Context, i *Int) {
	//TODO
}

func (l *List) insertSequence(ctx *Context, seq Sequence, i *Int) {
	if l.readonly {
		ctx.AddSymbolicGoFunctionError(ErrReadonlyValueCannotBeMutated.Error())
	}
	if seq.HasKnownLen() && seq.KnownLen() == 0 {
		return
	}
	if l.HasKnownLen() && l.KnownLen() == 0 {
		element := seq.element()
		if serializable, ok := element.(Serializable); ok {
			ctx.SetUpdatedSelf(NewList(serializable))
		} else {
			ctx.AddSymbolicGoFunctionError(NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE)
		}
		return
	}
	element := AsSerializable(widenToSameStaticTypeInMultivalue(joinValues([]Value{l.element(), seq.element()})))
	if serializable, ok := element.(Serializable); ok {
		ctx.SetUpdatedSelf(NewListOf(serializable))
	} else {
		ctx.AddSymbolicGoFunctionError(NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE)
	}
}

func (l *List) appendSequence(ctx *Context, seq Sequence) {
	if l.readonly {
		ctx.AddSymbolicGoFunctionError(ErrReadonlyValueCannotBeMutated.Error())
	}
	if seq.HasKnownLen() && seq.KnownLen() == 0 {
		return
	}
	if l.HasKnownLen() && l.KnownLen() == 0 {
		element := seq.element()
		if serializable, ok := element.(Serializable); ok {
			if seq.HasKnownLen() {
				length := seq.KnownLen()
				elements := make([]Serializable, length)

				for i := 0; i < length; i++ {
					elements[i] = seq.elementAt(i).(Serializable)
				}
				ctx.SetUpdatedSelf(NewList(elements...))
			} else {
				ctx.SetUpdatedSelf(NewListOf(serializable))
			}
		} else {
			ctx.AddSymbolicGoFunctionError(NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE)
		}
		return
	}
	element := AsSerializable(widenToSameStaticTypeInMultivalue(joinValues([]Value{l.element(), seq.element()})))
	if serializable, ok := element.(Serializable); ok {
		ctx.SetUpdatedSelf(NewListOf(serializable))
	} else {
		ctx.AddSymbolicGoFunctionError(NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE)
	}
}

func (l *List) Append(ctx *Context, elements ...Serializable) {
	if l.generalElement != nil {
		ctx.SetSymbolicGoFunctionParameters(&[]Value{l.element()}, []string{"values"})
	}
	l.appendSequence(ctx, NewList(elements...))
}

func (l *List) WatcherElement() Value {
	return ANY
}

// A Tuple represents a symbolic Tuple.
type Tuple struct {
	elements       []Serializable
	generalElement Serializable

	SerializableMixin
}

func NewTuple(elements ...Serializable) *Tuple {
	l := &Tuple{elements: make([]Serializable, 0)}
	for _, e := range elements {
		l.append(e)
	}
	return l
}

func NewTupleOf(generalElement Serializable) *Tuple {
	return &Tuple{generalElement: generalElement}
}

func (t *Tuple) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherList, ok := v.(*Tuple)
	if !ok {
		return false
	}

	if t.elements == nil {
		if otherList.elements == nil {
			return t.generalElement.Test(otherList.generalElement, state)
		}

		for _, elem := range otherList.elements {
			if !t.generalElement.Test(elem, state) {
				return false
			}
		}
		return true
	}

	if len(t.elements) != len(otherList.elements) || otherList.elements == nil {
		return false
	}

	for i, e := range t.elements {
		if !e.Test(otherList.elements[i], state) {
			return false
		}
	}
	return true
}

func (t *Tuple) IsConcretizable() bool {
	//TODO: support constraints

	if t.generalElement != nil {
		return false
	}

	for _, elem := range t.elements {
		if potentiallyConcretizable, ok := elem.(PotentiallyConcretizable); !ok || !potentiallyConcretizable.IsConcretizable() {
			return false
		}
	}

	return true
}

func (t *Tuple) Concretize(ctx ConcreteContext) any {
	if !t.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concreteElements := make([]any, len(t.elements))
	for i, e := range t.elements {
		concreteElements[i] = utils.Must(Concretize(e, ctx))
	}
	return extData.ConcreteValueFactories.CreateList(concreteElements)
}

func (t *Tuple) Static() Pattern {
	if t.generalElement != nil {
		return NewListPatternOf(&TypePattern{val: t.generalElement})
	}

	var elements []Value
	for _, e := range t.elements {
		elements = append(elements, getStatic(e).SymbolicValue())
	}

	if len(elements) == 0 {
		return NewListPatternOf(&TypePattern{val: ANY_SERIALIZABLE})
	}

	elem := AsSerializableChecked(joinValues(elements))
	return NewListPatternOf(&TypePattern{val: elem})
}

func (t *Tuple) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if t.elements != nil {
		lst := NewList(t.elements...)
		w.WriteByte('#')
		lst.PrettyPrint(w, config)
		return
	}
	w.WriteString("#[]")
	t.generalElement.PrettyPrint(w.ZeroDepthIndent(), config)
}

func (t *Tuple) append(element Value) {
	if t.elements == nil {
		t.elements = make([]Serializable, 0)
	}

	t.elements = append(t.elements, AsSerializableChecked(element))
}

func (t *Tuple) HasKnownLen() bool {
	return t.elements != nil
}

func (t *Tuple) KnownLen() int {
	if t.elements == nil {
		panic("cannot get length of a symbolic length with no known length")
	}

	return len(t.elements)
}

func (t *Tuple) element() Value {
	if t.elements != nil {
		if len(t.elements) == 0 {
			return ANY_SERIALIZABLE // return "never" ?
		}
		return AsSerializableChecked(joinValues(SerializablesToValues(t.elements)))
	}
	return t.generalElement
}

func (t *Tuple) elementAt(i int) Value {
	if t.elements != nil {
		if len(t.elements) == 0 || i >= len(t.elements) {
			return ANY // return "never" ?
		}
		return t.elements[i]
	}
	return t.generalElement
}

func (t *Tuple) Contains(value Value) (bool, bool) {
	if t.elements == nil {
		if t.generalElement.Test(value, RecTestCallState{}) {
			return false, true
		}
		return false, false
	}

	possible := false

	for _, e := range t.elements {
		if e.Test(value, RecTestCallState{}) {
			possible = true
			if value.Test(e, RecTestCallState{}) {
				return true, true
			}
		}
	}
	return false, possible
}

func (t *Tuple) IteratorElementKey() Value {
	return ANY_INT
}

func (t *Tuple) IteratorElementValue() Value {
	return t.element()
}

func (t *Tuple) WidestOfType() Value {
	return WIDEST_TUPLE_PATTERN
}

func (t *Tuple) slice(start, end *Int) Sequence {
	if t.HasKnownLen() {
		return &Tuple{generalElement: ANY_SERIALIZABLE}
	}
	return &Tuple{
		generalElement: t.generalElement,
	}
}

// A KeyList represents a symbolic KeyList.
type KeyList struct {
	Keys []string //if nil, matches any
	SerializableMixin
}

func NewAnyKeyList() *KeyList {
	return &KeyList{}
}

func (list *KeyList) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherList, ok := v.(*KeyList)
	if !ok {
		return false
	}
	if list.Keys == nil {
		return true
	}
	if len(list.Keys) != len(otherList.Keys) {
		return false
	}
	for i, k := range list.Keys {
		if otherList.Keys[i] != k {
			return false
		}
	}
	return true
}

func (list *KeyList) IsConcretizable() bool {
	if list.Keys == nil {
		return false
	}

	return true
}

func (list *KeyList) Concretize(ctx ConcreteContext) any {
	if !list.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	return extData.ConcreteValueFactories.CreateKeyList(utils.CopySlice(list.Keys))
}

func (list *KeyList) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if list.Keys != nil {
		if w.Depth > config.MaxDepth && len(list.Keys) > 0 {
			w.WriteString(".{(...)]}")
			return
		}

		w.WriteBytes(DOT_OPENING_CURLY_BRACKET)

		first := true

		for _, k := range list.Keys {
			if !first {
				w.WriteBytes(COMMA_SPACE)
			}
			first = false

			w.WriteBytes([]byte(k))
		}

		w.WriteByte(']')
		return
	}
	w.WriteName("key-list")
}

func (a *KeyList) append(key string) {
	a.Keys = append(a.Keys, key)
}

func (l *KeyList) WidestOfType() Value {
	return &KeyList{}
}

//

type Dictionary struct {
	//if nil, matches any dictionary, map (approximate key representation) -> value
	entries map[string]Serializable
	//map (approximate key representation) -> key
	keys map[string]Serializable

	SerializableMixin
	PseudoClonableMixin

	UnassignablePropsMixin
}

func NewAnyDictionary() *Dictionary {
	return &Dictionary{}
}

func NewUnitializedDictionary() *Dictionary {
	return &Dictionary{}
}

func NewDictionary(entries map[string]Serializable, keys map[string]Serializable) *Dictionary {
	if entries == nil {
		entries = map[string]Serializable{}
	}
	return &Dictionary{
		entries: entries,
		keys:    keys,
	}
}

func InitializeDictionary(d *Dictionary, entries map[string]Serializable, keys map[string]Serializable) {
	if d.entries != nil || d.keys != nil {
		panic(errors.New("dictionary is already initialized"))
	}
	if entries == nil {
		entries = map[string]Serializable{}
	}
	d.entries = entries
	d.keys = keys
}

func (dict *Dictionary) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherDict, ok := v.(*Dictionary)
	if !ok {
		return false
	}

	if dict.entries == nil {
		return true
	}

	if len(dict.entries) != len(otherDict.entries) || otherDict.entries == nil {
		return false
	}

	for i, e := range dict.entries {
		if !e.Test(otherDict.entries[i], state) {
			return false
		}
	}
	return true
}

func (dict *Dictionary) IsConcretizable() bool {
	//TODO: support constraints

	if dict.entries == nil {
		return false
	}

	for _, v := range dict.entries {
		if !IsConcretizable(v) {
			return false
		}
	}

	for _, key := range dict.entries {
		if !IsConcretizable(key) {
			return false
		}
	}

	return true
}

func (dict *Dictionary) Concretize(ctx ConcreteContext) any {
	if !dict.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concreteValues := make([]any, len(dict.entries))
	concreteKeys := make([]any, len(dict.entries))

	i := 0
	for keyRepr, value := range dict.entries {
		concreteValue := utils.Must(Concretize(value, ctx))
		concreteKey := utils.Must(Concretize(dict.keys[keyRepr], ctx))

		concreteValues[i] = concreteValue
		concreteKeys[i] = concreteKey
	}
	return extData.ConcreteValueFactories.CreateDictionary(concreteKeys, concreteValues, ctx)
}

func (dict *Dictionary) Entries() map[string]Serializable {
	return maps.Clone(dict.entries)
}

func (dict *Dictionary) Keys() map[string]Serializable {
	return maps.Clone(dict.keys)
}

func (dict *Dictionary) hasKey(keyRepr string) bool {
	if dict.entries == nil {
		return true
	}
	_, ok := dict.keys[keyRepr]
	return ok
}

func (dict *Dictionary) get(keyRepr string) (Value, bool) {
	if dict.entries == nil {
		return ANY, true
	}
	v, ok := dict.entries[keyRepr]
	return v, ok
}

func (dict *Dictionary) Get(ctx *Context, key Serializable) (Value, *Bool) {
	return ANY_SERIALIZABLE, ANY_BOOL
}

func (dict *Dictionary) SetValue(ctx *Context, key, value Serializable) {

}

func (dict *Dictionary) key() Value {
	if dict.entries != nil {
		if len(dict.entries) == 0 {
			return ANY
		}
		var keys []Value
		for _, k := range dict.keys {
			keys = append(keys, k)
		}
		return AsSerializableChecked(joinValues(keys))
	}
	return ANY
}

func (dict *Dictionary) ForEachEntry(fn func(k string, v Value) error) error {
	for k, v := range dict.entries {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (dict *Dictionary) Prop(name string) Value {
	switch name {
	case "get":
		return WrapGoMethod(dict.Get)
	case "set":
		return WrapGoMethod(dict.SetValue)
	default:
		panic(FormatErrPropertyDoesNotExist(name, dict))
	}
}

func (dict *Dictionary) PropertyNames() []string {
	return DICTIONARY_PROPNAMES
}

func (dict *Dictionary) IteratorElementKey() Value {
	return dict.key()
}

func (dict *Dictionary) IteratorElementValue() Value {
	return ANY
}

func (dict *Dictionary) WatcherElement() Value {
	return ANY
}

func (dict *Dictionary) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if dict.entries != nil {
		if w.Depth > config.MaxDepth && len(dict.entries) > 0 {
			w.WriteString(":{(...)}")
			return
		}

		indentCount := w.ParentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		w.WriteString(":{")

		var keys []string
		for k := range dict.entries {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for i, k := range keys {
			if !config.Compact {
				w.WriteLFCR()
				w.WriteBytes(indent)
			}

			//key
			if config.Colorize {
				w.WriteBytes(config.Colors.StringLiteral)

			}
			w.WriteString(k)

			if config.Colorize {
				w.WriteBytes(ANSI_RESET_SEQUENCE)
			}

			//colon
			w.WriteBytes(COLON_SPACE)

			//value
			v := dict.entries[k]

			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				w.WriteBytes([]byte{',', ' '})

			}

		}

		if !config.Compact && len(keys) > 0 {
			w.WriteLFCR()
		}
		w.WriteBytes(bytes.Repeat(config.Indent, w.Depth))
		w.WriteByte('}')
		return
	}
	w.WriteName("dictionary")
	return
}

func (d *Dictionary) WidestOfType() Value {
	return &Dictionary{}
}

//

type Object struct {
	entries                    map[string]Serializable //if nil, matches any object
	optionalEntries            map[string]struct{}
	dependencies               map[string]propertyDependencies
	static                     map[string]Pattern //key in .Static => key in .Entries, not reciprocal
	complexPropertyConstraints []*ComplexPropertyConstraint
	shared                     bool
	exact                      bool
	readonly                   bool

	SerializableMixin
	UrlHolderMixin
}

func NewAnyObject() *Object {
	return &Object{}
}

func NewEmptyObject() *Object {
	return &Object{entries: map[string]Serializable{}}
}

func NewEmptyReadonlyObject() *Object {
	obj := NewEmptyObject()
	obj.readonly = true
	return obj
}

func NewObject(exact bool, entries map[string]Serializable, optionalEntries map[string]struct{}, static map[string]Pattern) *Object {
	obj := &Object{
		entries:         entries,
		optionalEntries: optionalEntries,
		static:          static,
		exact:           exact,
	}
	return obj
}

func NewInexactObject(entries map[string]Serializable, optionalEntries map[string]struct{}, static map[string]Pattern) *Object {
	return NewObject(false, entries, optionalEntries, static)
}

func NewInexactObject2(entries map[string]Serializable) *Object {
	return NewObject(false, entries, nil, nil)
}

func NewExactObject(entries map[string]Serializable, optionalEntries map[string]struct{}, static map[string]Pattern) *Object {
	return NewObject(true, entries, optionalEntries, static)
}

func NewUnitializedObject() *Object {
	return &Object{}
}

func InitializeObject(obj *Object, entries map[string]Serializable, static map[string]Pattern, shared bool) {
	if obj.entries != nil {
		panic(errors.New("object is already initialized"))
	}
	obj.entries = entries
	obj.static = static
	obj.shared = shared
}

func (obj *Object) initNewProp(key string, value Serializable, static Pattern) {
	if obj.entries == nil {
		obj.entries = make(map[string]Serializable, 1)
	}
	obj.entries[key] = value

	if static == nil {
		static = getStatic(value)
	}

	if obj.static == nil {
		obj.static = make(map[string]Pattern, 1)
	}
	obj.static[key] = static
}

func (o *Object) ReadonlyObject() *Object {
	readonly := *o
	readonly.readonly = true
	return &readonly
}

func (obj *Object) TestExact(v Value) bool {
	return obj.test(v, true, RecTestCallState{})
}

func (obj *Object) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return obj.test(v, obj.exact, state)
}

func (obj *Object) test(v Value, exact bool, state RecTestCallState) bool {
	otherObj, ok := v.(*Object)
	if !ok || obj.readonly != otherObj.readonly {
		return false
	}

	if obj.entries == nil {
		return true
	}

	if obj.exact && otherObj.IsInexact() {
		return false
	}

	if (exact && len(obj.optionalEntries) == 0 && len(obj.entries) != len(otherObj.entries)) || otherObj.entries == nil {
		return false
	}

	//check dependencies
	for propName, deps := range obj.dependencies {
		counterPartDeps, ok := otherObj.dependencies[propName]
		if ok {
			for _, dep := range deps.requiredKeys {
				if !slices.Contains(counterPartDeps.requiredKeys, dep) {
					return false
				}
			}
			if deps.pattern != nil && (counterPartDeps.pattern == nil || !deps.pattern.Test(counterPartDeps.pattern, RecTestCallState{})) {
				return false
			}
		} else if !otherObj.hasRequiredProperty(propName) {
			//if the property does not exist or is optional in otherObj it's impossible
			//to known if the dependency constraint is fulfilled.
			return false
		}
	}

	for propName, propPattern := range obj.entries {
		_, isOptional := obj.optionalEntries[propName]
		_, isOptionalInOther := otherObj.optionalEntries[propName]

		other, isPresentInOther := otherObj.entries[propName]

		if isPresentInOther && !isOptional && isOptionalInOther {
			return false
		}

		if !isPresentInOther {
			if isOptional {
				continue
			}
			return false
		}

		if !propPattern.Test(other, state) {
			return false
		}

		if !isOptional || !isOptionalInOther {
			//check dependencies
			deps := obj.dependencies[propName]
			for _, requiredKey := range deps.requiredKeys {
				if !otherObj.hasRequiredProperty(requiredKey) {
					return false
				}
			}
			if deps.pattern != nil && !deps.pattern.TestValue(otherObj, state) {
				return false
			}
		}
	}

	//check there are no additional properties
	if exact {
		for k := range otherObj.entries {
			_, ok := obj.entries[k]
			if !ok {
				return false
			}
		}
	}

	return true
}

func (o *Object) SpecificIntersection(v Value, depth int) (Value, error) {
	if depth > MAX_INTERSECTION_COMPUTATION_DEPTH {
		return nil, ErrMaxIntersectionComputationDepthExceeded
	}

	other, ok := v.(*Object)

	if !ok || o.readonly != other.readonly {
		return NEVER, nil
	}

	if o.entries == nil {
		return v, nil
	}

	if other.entries == nil || other == o {
		return o, nil
	}

	// if at least one of the objects is inexact there are potentially properties we don't know of,
	// so inexactness wins.
	exact := o.exact && other.exact

	entries := map[string]Serializable{}
	var optionalEntries map[string]struct{}
	var static map[string]Pattern

	// add properties of self
	for propName, prop := range o.entries {

		var propInResult Value = prop
		propInOther, existsInOther := other.entries[propName]
		if existsInOther {
			val, err := getIntersection(depth+1, prop, propInOther)

			if err != nil {
				return nil, err
			}
			if val == NEVER {
				return NEVER, nil
			}

			propInResult = val
		}

		entries[propName] = AsSerializableChecked(propInResult)

		// if the property is optional in both objects then it is optional
		if existsInOther && o.IsExistingPropertyOptional(propName) && other.IsExistingPropertyOptional(propName) {
			if optionalEntries == nil {
				optionalEntries = map[string]struct{}{}
			}
			optionalEntries[propName] = struct{}{}
		}

		staticInSelf, haveStatic := o.static[propName]
		staticInOther, haveStaticInOther := other.static[propName]

		if haveStatic && haveStaticInOther {
			if static == nil {
				static = map[string]Pattern{}
			}

			//add narrowest
			if staticInSelf.Test(staticInOther, RecTestCallState{}) {
				static[propName] = staticInOther
			} else if staticInOther.Test(staticInSelf, RecTestCallState{}) {
				static[propName] = staticInSelf
			} else {
				return NEVER, nil
			}
		} else if haveStatic {
			if !staticInSelf.TestValue(propInResult, RecTestCallState{}) {
				return NEVER, nil
			}
			if static == nil {
				static = map[string]Pattern{}
			}
			static[propName] = staticInSelf
		} else if haveStaticInOther {
			if !staticInOther.TestValue(propInResult, RecTestCallState{}) {
				return NEVER, nil
			}
			if static == nil {
				static = map[string]Pattern{}
			}
			static[propName] = staticInOther
		}
	}

	// add properties of other
	for propName, prop := range other.entries {
		_, existsInSelf := o.entries[propName]
		if existsInSelf {
			continue
		}

		entries[propName] = prop

		if other.IsExistingPropertyOptional(propName) {
			if optionalEntries == nil {
				optionalEntries = map[string]struct{}{}
			}

			optionalEntries[propName] = struct{}{}
		}

		staticInOther, ok := other.static[propName]
		if ok {
			if static == nil {
				static = map[string]Pattern{}
			}
			static[propName] = staticInOther
		}
	}

	return NewObject(exact, entries, optionalEntries, static), nil
}

func (o *Object) IsInexact() bool {
	return !o.exact
}

func (o *Object) IsConcretizable() bool {
	//TODO: support constraints
	if o.entries == nil || len(o.optionalEntries) > 0 || o.shared {
		return false
	}

	for _, v := range o.entries {
		if potentiallyConcretizable, ok := v.(PotentiallyConcretizable); !ok || !potentiallyConcretizable.IsConcretizable() {
			return false
		}
	}

	return true
}

func (o *Object) Concretize(ctx ConcreteContext) any {
	if !o.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concreteProperties := make(map[string]any, len(o.entries))
	for k, v := range o.entries {
		concreteProperties[k] = utils.Must(Concretize(v, ctx))
	}
	return extData.ConcreteValueFactories.CreateObject(concreteProperties)
}

func (o *Object) IsReadonly() bool {
	return o.readonly
}

func (o *Object) ToReadonly() (PotentiallyReadonly, error) {
	if o.entries == nil {
		return nil, ErrNotConvertibleToReadonly
	}

	if o.readonly {
		return o, nil
	}

	properties := make(map[string]Serializable, len(o.entries))

	for k, v := range o.entries {
		if !v.IsMutable() {
			properties[k] = v
			continue
		}
		potentiallyReadonly, ok := v.(PotentiallyReadonly)
		if !ok {
			return nil, FmtPropertyError(k, ErrNotConvertibleToReadonly)
		}
		readonly, err := potentiallyReadonly.ToReadonly()
		if err != nil {
			return nil, FmtPropertyError(k, err)
		}
		properties[k] = readonly.(Serializable)
	}

	obj := NewObject(o.exact, properties, o.optionalEntries, o.static)
	obj.readonly = true
	return obj, nil
}

func (obj *Object) IsSharable() (bool, string) {
	if obj.shared {
		return true, ""
	}
	for k, v := range obj.entries {
		if ok, expl := IsSharableOrClonable(v); !ok {
			return false, commonfmt.FmtNotSharableBecausePropertyNotSharable(k, expl)
		}
	}
	return true, ""
}

func (obj *Object) Share(originState *State) PotentiallySharable {
	if obj.shared {
		return obj
	}
	shared := &Object{
		entries: maps.Clone(obj.entries),
		static:  obj.static,
		shared:  true,
	}

	for k, v := range obj.entries {
		newVal, err := ShareOrClone(v, originState)
		if err != nil {
			panic(err)
		}

		shared.entries[k] = newVal.(Serializable)
	}

	return shared
}

func (obj *Object) IsShared() bool {
	return obj.shared
}

func (obj *Object) Prop(name string) Value {
	v, ok := obj.entries[name]
	if !ok {
		panic(fmt.Errorf("object does not have a .%s property", name))
	}
	return v
}

func (obj *Object) MatchAnyObject() bool {
	return obj.entries == nil
}

func (obj *Object) ForEachEntry(fn func(propName string, propValue Value) error) error {
	for k, v := range obj.entries {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (obj *Object) ValueEntryMap() map[string]Value {
	entries := map[string]Value{}
	for k, v := range obj.entries {
		entries[k] = v
	}
	return entries
}

func (obj *Object) SetProp(name string, value Value) (IProps, error) {
	if obj.readonly {
		return nil, ErrReadonlyValueCannotBeMutated
	}

	if obj.entries == nil {
		return ANY_OBJ, nil
	}
	if _, ok := obj.entries[name]; ok { // update property

		if static, ok := obj.static[name]; ok {
			if !static.TestValue(value, RecTestCallState{}) {
				return nil, errors.New(fmtNotAssignableToPropOfType(value, static))
			}
		} else if prevValue, ok := obj.entries[name]; ok {
			if !prevValue.Test(value, RecTestCallState{}) {
				return nil, errors.New(fmtNotAssignableToPropOfType(value, &TypePattern{val: prevValue}))
			}
		}

		modified := *obj
		modified.entries = maps.Clone(obj.entries)
		modified.entries[name] = value.(Serializable)

		return &modified, nil
	}

	//new property

	if obj.exact {
		return nil, errors.New(CANNOT_ADD_NEW_PROPERTY_TO_AN_EXACT_OBJECT)
	}

	modified := *obj
	modified.entries = maps.Clone(obj.entries)
	modified.entries[name] = value.(Serializable)
	return &modified, nil
}

func (obj *Object) WithExistingPropReplaced(name string, value Value) (IProps, error) {
	if obj.readonly {
		return nil, ErrReadonlyValueCannotBeMutated
	}
	if obj.exact {
		return nil, errors.New(CANNOT_ADD_NEW_PROPERTY_TO_AN_EXACT_OBJECT)
	}

	modified := *obj
	modified.entries = maps.Clone(obj.entries)
	modified.optionalEntries = maps.Clone(obj.optionalEntries)
	modified.entries[name] = value.(Serializable)
	delete(modified.optionalEntries, name)

	return &modified, nil
}

// IsExistingPropertyOptional returns true if the property is part of the pattern and is optional
func (obj *Object) IsExistingPropertyOptional(name string) bool {
	_, ok := obj.optionalEntries[name]
	return ok
}

func (obj *Object) PropertyNames() []string {
	if obj.entries == nil {
		return nil
	}
	props := make([]string, len(obj.entries)-len(obj.optionalEntries))
	i := 0
	for k := range obj.entries {
		if _, isOptional := obj.optionalEntries[k]; isOptional {
			continue
		}
		props[i] = k
		i++
	}
	return props
}

func (obj *Object) OptionalPropertyNames() []string {
	return maps.Keys(obj.optionalEntries)
}

// func (obj *Object) SetNewProperty(name string, value SymbolicValue, static Pattern) {
// 	if obj.entries == nil {
// 		obj.entries = make(map[string]SymbolicValue, 1)
// 	}
// 	if static != nil {
// 		if obj.static == nil {
// 			obj.static = map[string]Pattern{name: static}
// 		} else {
// 			obj.static[name] = static
// 		}
// 	}

// 	obj.entries[name] = value
// }

func (obj *Object) hasProperty(name string) bool {
	if obj.entries == nil {
		return true
	}
	_, ok := obj.entries[name]
	return ok
}

func (obj *Object) hasRequiredProperty(name string) bool {
	_, ok := obj.optionalEntries[name]
	return !ok && obj.hasProperty(name)
}

func (obj *Object) hasDeps(name string) bool {
	_, ok := obj.dependencies[name]
	return ok
}

// result should not be modfied
func (obj *Object) GetProperty(name string) (Value, Pattern, bool) {
	if obj.entries == nil {
		return ANY, nil, true
	}
	v, ok := obj.entries[name]
	return v, obj.static[name], ok
}

func (obj *Object) AddStatic(pattern Pattern) (StaticDataHolder, error) {
	if objPatt, ok := pattern.(*ObjectPattern); ok {
		if obj.static == nil {
			obj.static = make(map[string]Pattern, len(objPatt.entries))
		}

		for k, v := range objPatt.entries {
			if _, ok := obj.entries[k]; !ok {
				//TODO
			}
			obj.static[k] = v
		}

		if !objPatt.inexact && len(obj.entries) != len(objPatt.entries) {
			//TODO
		}
	} else if _, ok := pattern.(*TypePattern); ok {
		//TODO
	} else if !pattern.TestValue(obj, RecTestCallState{}) {
		return nil, errors.New("cannot add static information of non object pattern")
	}
	return obj, nil
}

func (o *Object) HasKnownLen() bool {
	return false
}

func (o *Object) KnownLen() int {
	return -1
}

func (o *Object) element() Value {
	return ANY
}

func (*Object) elementAt(i int) Value {
	return ANY
}

func (o *Object) Contains(value Value) (bool, bool) {
	if o.entries == nil {
		return false, true
	}

	possible := false

	for _, e := range o.entries {
		if e.Test(value, RecTestCallState{}) {
			possible = true
			if value.Test(e, RecTestCallState{}) {
				return true, true
			}
		}
	}
	return false, possible
}

func (o *Object) IteratorElementKey() Value {
	return &String{}
}

func (o *Object) IteratorElementValue() Value {
	return o.element()
}

func (o *Object) WatcherElement() Value {
	return ANY
}

func (obj *Object) Static() Pattern {
	entries := map[string]Pattern{}

	for k, v := range obj.entries {
		static, ok := obj.static[k]
		if ok {
			entries[k] = static
		} else {
			entries[k] = getStatic(v)
		}
	}

	return NewInexactObjectPattern(entries, obj.optionalEntries)
}

func (obj *Object) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if obj.readonly {
		w.WriteName("readonly ")
	}

	if obj.entries != nil {
		if w.Depth > config.MaxDepth && len(obj.entries) > 0 {
			w.WriteString("{(...)}")
			return
		}

		indentCount := w.ParentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		w.WriteString("{")

		keys := maps.Keys(obj.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				w.WriteLFCR()
				w.WriteBytes(indent)
			}

			if config.Colorize {
				w.WriteBytes(config.Colors.IdentifierLiteral)
			}

			w.WriteBytes(utils.Must(utils.MarshalJsonNoHTMLEspace(k)))

			if config.Colorize {
				w.WriteBytes(ANSI_RESET_SEQUENCE)
			}

			if _, isOptional := obj.optionalEntries[k]; isOptional {
				w.WriteByte('?')
			}

			//colon
			w.WriteBytes(COLON_SPACE)

			//value
			v := obj.entries[k]
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				w.WriteBytes(COMMA_SPACE)
			}
		}

		if !config.Compact && len(keys) > 0 {
			w.WriteLFCR()
		}

		w.WriteManyBytes(bytes.Repeat(config.Indent, w.Depth), []byte{'}'})
		return
	}
	w.WriteName("object")
}

func (o *Object) WidestOfType() Value {
	return ANY_OBJ
}

// A Record represents a symbolic Record.
type Record struct {
	UnassignablePropsMixin
	entries         map[string]Serializable //if nil, matches any record
	optionalEntries map[string]struct{}
	valueOnly       Value
	exact           bool

	SerializableMixin
}

func NewAnyrecord() *Record {
	return &Record{}
}

func NewEmptyRecord() *Record {
	return &Record{entries: map[string]Serializable{}}
}

func NewInexactRecord(entries map[string]Serializable, optionalEntries map[string]struct{}) *Record {
	return &Record{
		entries:         entries,
		optionalEntries: optionalEntries,
		exact:           false,
	}
}

func NewExactRecord(entries map[string]Serializable, optionalEntries map[string]struct{}) *Record {
	return &Record{
		entries:         entries,
		optionalEntries: optionalEntries,
		exact:           true,
	}
}

func NewAnyKeyRecord(value Value) *Record {
	return &Record{valueOnly: value}
}

func NewBoundEntriesRecord(entries map[string]Serializable) *Record {
	return &Record{entries: entries}
}

func (rec *Record) TestExact(v Value) bool {
	return rec.test(v, true, RecTestCallState{})
}

func (rec *Record) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return rec.test(v, rec.exact, state)
}

func (rec *Record) test(v Value, exact bool, state RecTestCallState) bool {
	otherRec, ok := v.(*Record)
	if !ok {
		return false
	}

	if rec.entries == nil {
		return true
	}

	if rec.valueOnly != nil {
		value := rec.valueOnly
		if otherRec.valueOnly == nil {
			return false
		}
		return value.Test(otherRec.valueOnly, RecTestCallState{})
	}

	if (exact && len(rec.optionalEntries) == 0 && len(rec.entries) != len(otherRec.entries)) || otherRec.entries == nil {
		return false
	}

	for k, e := range rec.entries {
		_, isOptional := rec.optionalEntries[k]
		_, isOptionalInOther := otherRec.optionalEntries[k]

		other, ok := otherRec.entries[k]

		if ok && !isOptional && isOptionalInOther {
			return false
		}

		if !ok {
			if isOptional {
				continue
			}
			return false
		}
		if !e.Test(other, state) {
			return false
		}
	}

	if exact {
		for k := range otherRec.entries {
			_, ok := rec.entries[k]
			if !ok {
				return false
			}
		}
	}

	return true
}

func (r *Record) IsConcretizable() bool {
	//TODO: support constraints

	if r.entries == nil || len(r.optionalEntries) > 0 || r.valueOnly != nil {
		return false
	}

	for _, v := range r.entries {
		if potentiallyConcretizable, ok := v.(PotentiallyConcretizable); !ok || !potentiallyConcretizable.IsConcretizable() {
			return false
		}
	}

	return true
}

func (rec *Record) Concretize(ctx ConcreteContext) any {
	if !rec.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concreteProperties := make(map[string]any, len(rec.entries))
	for k, v := range rec.entries {
		concreteProperties[k] = utils.Must(Concretize(v, ctx))
	}
	return extData.ConcreteValueFactories.CreateObject(concreteProperties)
}

func (rec *Record) Prop(name string) Value {
	v, ok := rec.entries[name]
	if !ok {
		panic(fmt.Errorf("record does not have a .%s property", name))
	}
	return v
}

func (rec *Record) PropertyNames() []string {
	if rec.entries == nil {
		return nil
	}
	props := make([]string, len(rec.entries)-len(rec.optionalEntries))
	i := 0
	for k := range rec.entries {
		if _, isOptional := rec.optionalEntries[k]; isOptional {
			continue
		}
		props[i] = k
		i++
	}
	return props
}

func (rec *Record) OptionalPropertyNames() []string {
	return maps.Keys(rec.optionalEntries)
}

func (rec *Record) ValueEntryMap() map[string]Value {
	entries := map[string]Value{}
	for k, v := range rec.entries {
		entries[k] = v
	}
	return entries
}

func (rec *Record) hasProperty(name string) bool {
	if rec.entries == nil {
		return true
	}
	_, ok := rec.entries[name]
	return ok
}

func (rec *Record) getProperty(name string) (Value, bool) {
	if rec.entries == nil {
		return ANY, true
	}
	v, ok := rec.entries[name]
	return v, ok
}

func (rec *Record) ForEachEntry(fn func(k string, v Value) error) error {
	for k, v := range rec.entries {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (rec *Record) HasKnownLen() bool {
	return false
}

func (rec *Record) KnownLen() int {
	return -1
}

func (rec *Record) element() Value {
	return ANY
}

func (r *Record) Contains(value Value) (bool, bool) {
	if r.entries == nil {
		return false, true
	}

	possible := false

	for _, e := range r.entries {
		if e.Test(value, RecTestCallState{}) {
			possible = true
			if value.Test(e, RecTestCallState{}) {
				return true, true
			}
		}
	}
	return false, possible
}

func (rec *Record) IteratorElementKey() Value {
	return &String{}
}

func (rec *Record) IteratorElementValue() Value {
	return rec.element()
}

func (rec *Record) Static() Pattern {
	entries := map[string]Pattern{}

	for k, v := range rec.entries {
		entries[k] = getStatic(v)
	}

	return NewInexactObjectPattern(entries, rec.optionalEntries)
}

func (rec *Record) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if rec.entries != nil {
		if w.Depth > config.MaxDepth && len(rec.entries) > 0 {
			w.WriteString("#{(...)}")
			return
		}

		indentCount := w.ParentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		w.WriteString("#{")

		keys := maps.Keys(rec.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				w.WriteLFCR()
				w.WriteBytes(indent)
			}

			if config.Colorize {
				w.WriteBytes(config.Colors.IdentifierLiteral)
			}

			w.WriteBytes(utils.Must(utils.MarshalJsonNoHTMLEspace(k)))

			if config.Colorize {
				w.WriteBytes(ANSI_RESET_SEQUENCE)
			}

			if _, isOptional := rec.optionalEntries[k]; isOptional {
				w.WriteByte('?')
			}

			//colon
			w.WriteBytes(COLON_SPACE)

			//value
			v := rec.entries[k]
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				w.WriteBytes(COMMA_SPACE)
			}
		}

		if !config.Compact && len(keys) > 0 {
			w.WriteLFCR()
		}

		w.WriteManyBytes(bytes.Repeat(config.Indent, w.Depth), []byte{'}'})
		return
	}
	if rec.valueOnly == nil {
		w.WriteName("record")
		return
	}
	w.WriteString("#{ any -> ")
	rec.valueOnly.PrettyPrint(w.ZeroDepth(), config)
	w.WriteString("}")
}

func (r *Record) WidestOfType() Value {
	return ANY_REC
}

// An AnyIndexable represents a symbolic Indesable we do not know the concrete type.
type AnyIndexable struct {
	_ int
}

func (r *AnyIndexable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Indexable)

	return ok
}

func (i *AnyIndexable) IteratorElementKey() Value {
	return ANY
}

func (i *AnyIndexable) IteratorElementValue() Value {
	return ANY
}

func (i *AnyIndexable) element() Value {
	return ANY
}

func (i *AnyIndexable) elementAt(index int) Value {
	return ANY
}

func (i *AnyIndexable) KnownLen() int {
	return -1
}

func (i *AnyIndexable) HasKnownLen() bool {
	return false
}

func (r *AnyIndexable) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("indexable")
}

func (r *AnyIndexable) WidestOfType() Value {
	return ANY_INDEXABLE
}
