package symbolic

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/inoxlang/inox/internal/commonfmt"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	DICTIONARY_PROPNAMES = []string{"get", "set"}

	ANY_INDEXABLE = &AnyIndexable{}
	ANY_ARRAY     = NewArrayOf(ANY_SERIALIZABLE)
	ANY_TUPLE     = NewTupleOf(ANY_SERIALIZABLE)
	ANY_OBJ       = &Object{}
	ANY_REC       = &Record{}
	ANY_NAMESPACE = NewAnyNamespace()
	ANY_DICT      = NewAnyDictionary()
	ANY_KEYLIST   = NewAnyKeyList()

	_ = []Indexable{
		(*String)(nil), (*Array)(nil), (*List)(nil), (*Tuple)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil), (*Object)(nil), (*IntRange)(nil),
		(*AnyStringLike)(nil), (*AnyIndexable)(nil),
	}

	_ = []Sequence{
		(*String)(nil), (*Array)(nil), (*List)(nil), (*Tuple)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil),
	}

	_ = []IProps{(*Object)(nil), (*Record)(nil), (*Namespace)(nil), (*Dictionary)(nil)}
)

// An Indexable represents a symbolic Indexable.
type Indexable interface {
	Iterable
	element() SymbolicValue
	elementAt(i int) SymbolicValue
	KnownLen() int
	HasKnownLen() bool
}

// An Array represents a symbolic Array.
type Array struct {
	elements       []SymbolicValue
	generalElement SymbolicValue
}

func NewArray(elements ...SymbolicValue) *Array {
	if elements == nil {
		elements = []SymbolicValue{}
	}
	return &Array{elements: elements}
}

func NewArrayOf(generalElement SymbolicValue) *Array {
	return &Array{generalElement: generalElement}
}

func NewAnyArray() *Array {
	return &Array{}
}

func (a *Array) Test(v SymbolicValue) bool {
	otherArray, ok := v.(*Array)
	if !ok {
		return false
	}

	if a.elements == nil {
		if otherArray.elements == nil {
			return a.generalElement.Test(otherArray.generalElement)
		}

		for _, elem := range otherArray.elements {
			if !a.generalElement.Test(elem) {
				return false
			}
		}
		return true
	}

	if len(a.elements) != len(otherArray.elements) || otherArray.elements == nil {
		return false
	}

	for i, e := range a.elements {
		if !e.Test(otherArray.elements[i]) {
			return false
		}
	}
	return true
}

func (a *Array) Widen() (SymbolicValue, bool) {
	if a.elements == nil {
		if _, ok := a.generalElement.(*Any); ok {
			return nil, false
		}
		return &Array{generalElement: ANY}, true
	}

	allAny := true

	for _, elem := range a.elements {
		if _, ok := elem.(*Any); !ok {
			allAny = false
			break
		}
	}

	if allAny {
		return &Array{generalElement: ANY}, true
	}

	widenedElements := make([]SymbolicValue, 0)
	noWidening := true
	for _, elem := range a.elements {
		widened, ok := elem.Widen()
		if ok {
			noWidening = false
			widenedElements = append(widenedElements, AsSerializable(widened).(Serializable))
		} else {
			widenedElements = append(widenedElements, elem)
		}
	}
	if noWidening {
		return &Array{generalElement: joinValues(widenedElements)}, true
	}
	return &Array{elements: widenedElements}, true
}

func (a *Array) IsWidenable() bool {
	return a.elements != nil || !IsAny(a.generalElement)
}

func (a *Array) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if a.elements != nil {
		length := a.KnownLen()

		if depth > config.MaxDepth && length > 0 {
			utils.Must(w.Write(utils.StringAsBytes("Array(...)")))
			return
		}

		utils.Must(w.Write(utils.StringAsBytes("Array(")))

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)
		printIndices := !config.Compact && length > 10

		for i := 0; i < length; i++ {
			v := a.elements[i]

			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))

				//index
				if printIndices {
					if config.Colorize {
						utils.Must(w.Write(config.Colors.DiscreteColor))
					}
					if i < 10 {
						utils.PanicIfErr(w.WriteByte(' '))
					}
					utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(int64(i), 10))))
					utils.Must(w.Write(COLON_SPACE))
					if config.Colorize {
						utils.Must(w.Write(ANSI_RESET_SEQUENCE))
					}
				}
			}

			//element
			v.PrettyPrint(w, config, depth+1, indentCount)

			//comma & indent
			isLastEntry := i == length-1

			if !isLastEntry {
				utils.Must(w.Write(COMMA_SPACE))
			}

		}

		var end []byte
		if !config.Compact && length > 0 {
			end = append(end, '\n', '\r')
			end = append(end, bytes.Repeat(config.Indent, depth)...)
		}
		end = append(end, ')')

		utils.Must(w.Write(end))
		return
	}

	utils.Must(w.Write(utils.StringAsBytes("%array(")))
	a.generalElement.PrettyPrint(w, config, depth, parentIndentCount)
	utils.Must(w.Write(utils.StringAsBytes(")")))
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

func (a *Array) element() SymbolicValue {
	if a.elements != nil {
		if len(a.elements) == 0 {
			return ANY
		}
		return joinValues(a.elements)
	}
	return ANY
}

func (a *Array) elementAt(i int) SymbolicValue {
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

func (a *Array) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (a *Array) IteratorElementValue() SymbolicValue {
	return a.element()
}

func (*Array) WidestOfType() SymbolicValue {
	return ANY_ARRAY
}

// A List represents a symbolic List.
type List struct {
	elements       []Serializable
	generalElement Serializable

	SerializableMixin
	PseudoClonableMixin
}

func NewList(elements ...Serializable) *List {
	if elements == nil {
		elements = []Serializable{}
	}
	return &List{elements: elements}
}

func NewListOf(generalElement Serializable) *List {
	return &List{generalElement: generalElement}
}

func (list *List) Test(v SymbolicValue) bool {
	otherList, ok := v.(*List)
	if !ok {
		return false
	}

	if list.elements == nil {
		if otherList.elements == nil {
			return list.generalElement.Test(otherList.generalElement)
		}

		for _, elem := range otherList.elements {
			if !list.generalElement.Test(elem) {
				return false
			}
		}
		return true
	}

	if len(list.elements) != len(otherList.elements) || otherList.elements == nil {
		return false
	}

	for i, e := range list.elements {
		if !e.Test(otherList.elements[i]) {
			return false
		}
	}
	return true
}

func (list *List) Widen() (SymbolicValue, bool) {
	if list.elements == nil {
		if _, ok := list.generalElement.(*AnySerializable); ok {
			return nil, false
		}
		return &List{generalElement: ANY_SERIALIZABLE}, true
	}

	allAny := true

	for _, elem := range list.elements {
		if _, ok := elem.(*AnySerializable); !ok {
			allAny = false
			break
		}
	}

	if allAny {
		return &List{generalElement: ANY_SERIALIZABLE}, true
	}

	widenedElements := make([]Serializable, 0)
	noWidening := true
	for _, elem := range list.elements {
		widened, ok := elem.Widen()
		if ok {
			noWidening = false
			widenedElements = append(widenedElements, AsSerializable(widened).(Serializable))
		} else {
			widenedElements = append(widenedElements, elem)
		}
	}
	if noWidening {
		return &List{generalElement: AsSerializable(joinValues(SerializablesToValues(widenedElements))).(Serializable)}, true
	}
	return &List{elements: widenedElements}, true
}

func (list *List) IsWidenable() bool {
	return list.elements != nil || !IsAnySerializable(list.generalElement)
}

func (list *List) Static() Pattern {
	if list.generalElement != nil {
		return NewListPatternOf(&TypePattern{val: list.generalElement})
	}

	var elements []SymbolicValue
	for _, e := range list.elements {
		elements = append(elements, getStatic(e).SymbolicValue())
	}

	if len(elements) == 0 {
		return NewListPatternOf(&TypePattern{val: ANY_SERIALIZABLE})
	}

	elem := AsSerializable(joinValues(elements))
	return NewListPatternOf(&TypePattern{val: elem})
}

func (list *List) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if list.elements != nil {
		length := list.KnownLen()

		if depth > config.MaxDepth && length > 0 {
			utils.Must(w.Write(utils.StringAsBytes("[(...)]")))
			return
		}

		utils.PanicIfErr(w.WriteByte('['))

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)
		printIndices := !config.Compact && length > 10

		for i := 0; i < length; i++ {
			v := list.elements[i]

			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))

				//index
				if printIndices {
					if config.Colorize {
						utils.Must(w.Write(config.Colors.DiscreteColor))
					}
					if i < 10 {
						utils.PanicIfErr(w.WriteByte(' '))
					}
					utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(int64(i), 10))))
					utils.Must(w.Write(COLON_SPACE))
					if config.Colorize {
						utils.Must(w.Write(ANSI_RESET_SEQUENCE))
					}
				}
			}

			//element
			v.PrettyPrint(w, config, depth+1, indentCount)

			//comma & indent
			isLastEntry := i == length-1

			if !isLastEntry {
				utils.Must(w.Write(COMMA_SPACE))
			}

		}

		var end []byte
		if !config.Compact && length > 0 {
			end = append(end, '\n', '\r')
			end = append(end, bytes.Repeat(config.Indent, depth)...)
		}
		end = append(end, ']')

		utils.Must(w.Write(end))
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("[]")))
	list.generalElement.PrettyPrint(w, config, depth, parentIndentCount)
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

func (l *List) element() SymbolicValue {
	if l.elements != nil {
		if len(l.elements) == 0 {
			return ANY_SERIALIZABLE
		}
		return AsSerializable(joinValues(SerializablesToValues(l.elements)))
	}
	return l.generalElement
}

func (l *List) elementAt(i int) SymbolicValue {
	if l.elements != nil {
		if len(l.elements) == 0 || i >= len(l.elements) {
			return ANY // return "never" ?
		}
		return l.elements[i]
	}
	return l.generalElement
}

func (l *List) Contains(ctx *Context, value SymbolicValue) (bool, bool) {
	if l.elements == nil {
		if l.generalElement.Test(value) {
			return false, true
		}
		return false, false
	}

	possible := false

	for _, e := range l.elements {
		if e.Test(value) {
			possible = true
			if value.Test(e) {
				return true, true
			}
		}
	}
	return false, possible
}

func (l *List) set(i *Int, v SymbolicValue) {

}

func (l *List) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (l *List) IteratorElementValue() SymbolicValue {
	return l.element()
}

func (l *List) WidestOfType() SymbolicValue {
	return &List{generalElement: ANY_SERIALIZABLE}
}

func (l *List) slice(start, end *Int) Sequence {
	if l.HasKnownLen() {
		return &List{generalElement: ANY_SERIALIZABLE}
	}
	return &List{
		generalElement: l.generalElement,
	}
}

func (l *List) SetSlice(start, end *Int, v Sequence) {

}

func (l *List) insertElement(v SymbolicValue, i *Int) *Error {
	panic(ErrNotImplementedYet)
}

func (l *List) removePosition(i *Int) *Error {
	panic(ErrNotImplementedYet)
}

func (l *List) insertSequence(seq Sequence, i *Int) *Error {
	panic(ErrNotImplementedYet)

}
func (l *List) appendSequence(seq Sequence) *Error {
	panic(ErrNotImplementedYet)
}

func (l *List) WatcherElement() SymbolicValue {
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

func (t *Tuple) Test(v SymbolicValue) bool {
	otherList, ok := v.(*Tuple)
	if !ok {
		return false
	}

	if t.elements == nil {
		if otherList.elements == nil {
			return t.generalElement.Test(otherList.generalElement)
		}

		for _, elem := range otherList.elements {
			if !t.generalElement.Test(elem) {
				return false
			}
		}
		return true
	}

	if len(t.elements) != len(otherList.elements) || otherList.elements == nil {
		return false
	}

	for i, e := range t.elements {
		if !e.Test(otherList.elements[i]) {
			return false
		}
	}
	return true
}

func (t *Tuple) Widen() (SymbolicValue, bool) {
	if t.elements == nil {
		if _, ok := t.generalElement.(*AnySerializable); ok {
			return nil, false
		}
		return &Tuple{generalElement: ANY_SERIALIZABLE}, true
	}

	allAny := true

	for _, elem := range t.elements {
		if _, ok := elem.(*AnySerializable); !ok {
			allAny = false
			break
		}
	}

	if allAny {
		return &Tuple{generalElement: ANY_SERIALIZABLE}, true
	}

	widenedElements := make([]Serializable, 0)
	noWidening := true
	for _, elem := range t.elements {
		widened, ok := elem.Widen()
		if ok {
			noWidening = false
			widenedElements = append(widenedElements, AsSerializable(widened).(Serializable))
		} else {
			widenedElements = append(widenedElements, elem)
		}
	}
	if noWidening {
		return &Tuple{generalElement: AsSerializable(joinValues(SerializablesToValues(widenedElements))).(Serializable)}, true
	}
	return &Tuple{elements: widenedElements}, true
}

func (t *Tuple) IsWidenable() bool {
	return t.elements != nil || !IsAnySerializable(t.generalElement)
}

func (t *Tuple) Static() Pattern {
	if t.generalElement != nil {
		return NewListPatternOf(&TypePattern{val: t.generalElement})
	}

	var elements []SymbolicValue
	for _, e := range t.elements {
		elements = append(elements, getStatic(e).SymbolicValue())
	}

	if len(elements) == 0 {
		return NewListPatternOf(&TypePattern{val: ANY_SERIALIZABLE})
	}

	elem := AsSerializable(joinValues(elements))
	return NewListPatternOf(&TypePattern{val: elem})
}

func (t *Tuple) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if t.elements != nil {
		lst := NewList(t.elements...)
		utils.Must(w.Write([]byte{'#'}))
		lst.PrettyPrint(w, config, depth, parentIndentCount)
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("#[]")))
	t.generalElement.PrettyPrint(w, config, 0, 0)
}

func (t *Tuple) append(element SymbolicValue) {
	if t.elements == nil {
		t.elements = make([]Serializable, 0)
	}

	t.elements = append(t.elements, AsSerializable(element).(Serializable))
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

func (t *Tuple) element() SymbolicValue {
	if t.elements != nil {
		if len(t.elements) == 0 {
			return ANY_SERIALIZABLE // return "never" ?
		}
		return AsSerializable(joinValues(SerializablesToValues(t.elements)))
	}
	return t.generalElement
}

func (t *Tuple) elementAt(i int) SymbolicValue {
	if t.elements != nil {
		if len(t.elements) == 0 || i >= len(t.elements) {
			return ANY // return "never" ?
		}
		return t.elements[i]
	}
	return t.generalElement
}

func (t *Tuple) Contains(ctx *Context, value SymbolicValue) (bool, bool) {
	if t.elements == nil {
		if t.generalElement.Test(value) {
			return false, true
		}
		return false, false
	}

	possible := false

	for _, e := range t.elements {
		if e.Test(value) {
			possible = true
			if value.Test(e) {
				return true, true
			}
		}
	}
	return false, possible
}

func (t *Tuple) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (t *Tuple) IteratorElementValue() SymbolicValue {
	return t.element()
}

func (t *Tuple) WidestOfType() SymbolicValue {
	return &Tuple{generalElement: ANY_SERIALIZABLE}
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

func (list *KeyList) Test(v SymbolicValue) bool {
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

func (a *KeyList) Widen() (SymbolicValue, bool) {
	if a.Keys == nil {
		return nil, false
	}
	return &KeyList{}, true
}

func (list *KeyList) IsWidenable() bool {
	return list.Keys != nil
}

func (list *KeyList) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if list.Keys != nil {
		if depth > config.MaxDepth && len(list.Keys) > 0 {
			utils.Must(w.Write(utils.StringAsBytes(".{(...)]}")))
			return
		}

		utils.Must(w.Write(DOT_OPENING_CURLY_BRACKET))

		first := true

		for _, k := range list.Keys {
			if !first {
				utils.Must(w.Write(COMMA_SPACE))
			}
			first = false

			utils.Must(w.Write([]byte(k)))
		}

		utils.PanicIfErr(w.WriteByte(']'))
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%key-list")))
}

func (a *KeyList) append(key string) {
	a.Keys = append(a.Keys, key)
}

func (l *KeyList) WidestOfType() SymbolicValue {
	return &KeyList{}
}

//

type Dictionary struct {
	entries map[string]Serializable //if nil, matches any dictionary
	keys    map[string]Serializable

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

func (dict *Dictionary) Test(v SymbolicValue) bool {
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
		if !e.Test(otherDict.entries[i]) {
			return false
		}
	}
	return true
}

func (dict *Dictionary) Entries() map[string]Serializable {
	return utils.CopyMap(dict.entries)
}

func (dict *Dictionary) Keys() map[string]Serializable {
	return utils.CopyMap(dict.keys)
}

func (dict *Dictionary) hasKey(keyRepr string) bool {
	if dict.entries == nil {
		return true
	}
	_, ok := dict.keys[keyRepr]
	return ok
}

func (dict *Dictionary) get(keyRepr string) (SymbolicValue, bool) {
	if dict.entries == nil {
		return ANY, true
	}
	v, ok := dict.entries[keyRepr]
	return v, ok
}

func (dict *Dictionary) Get(ctx *Context, key Serializable) (SymbolicValue, *Bool) {
	return ANY_SERIALIZABLE, ANY_BOOL
}

func (dict *Dictionary) SetValue(ctx *Context, key, value Serializable) {

}

func (dict *Dictionary) key() SymbolicValue {
	if dict.entries != nil {
		if len(dict.entries) == 0 {
			return ANY
		}
		var keys []SymbolicValue
		for _, k := range dict.keys {
			keys = append(keys, k)
		}
		return AsSerializable(joinValues(keys))
	}
	return ANY
}

func (dict *Dictionary) Prop(name string) SymbolicValue {
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

func (dict *Dictionary) IteratorElementKey() SymbolicValue {
	return dict.key()
}

func (dict *Dictionary) IteratorElementValue() SymbolicValue {
	return ANY
}

func (dict *Dictionary) WatcherElement() SymbolicValue {
	return ANY
}

func (dict *Dictionary) Widen() (SymbolicValue, bool) {
	if dict.entries == nil {
		return nil, false
	}

	widenedEntries := map[string]Serializable{}
	keys := map[string]Serializable{}
	allAlreadyWidened := true

	for keyRepr, v := range dict.entries {
		widened, ok := v.Widen()
		if ok {
			allAlreadyWidened = false
			v = widened.(Serializable)
		}
		widenedEntries[keyRepr] = v
		keys[keyRepr] = dict.keys[keyRepr]
	}

	if allAlreadyWidened {
		return &Dictionary{}, true
	}

	return &Dictionary{entries: widenedEntries, keys: keys}, true
}

func (dict *Dictionary) IsWidenable() bool {
	_, ok := dict.Widen()
	return ok
}

func (dict *Dictionary) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if dict.entries != nil {
		if depth > config.MaxDepth && len(dict.entries) > 0 {
			utils.Must(w.Write(utils.StringAsBytes(":{(...)}")))
			return
		}

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		utils.Must(w.Write(utils.StringAsBytes(":{")))

		var keys []string
		for k := range dict.entries {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for i, k := range keys {
			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))
			}

			//key
			if config.Colorize {
				utils.Must(w.Write(config.Colors.StringLiteral))

			}
			utils.Must(w.Write(utils.StringAsBytes(k)))

			if config.Colorize {
				utils.Must(w.Write(ANSI_RESET_SEQUENCE))
			}

			//colon
			utils.Must(w.Write(COLON_SPACE))

			//value
			v := dict.entries[k]

			v.PrettyPrint(w, config, depth+1, indentCount)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				utils.Must(w.Write([]byte{',', ' '}))

			}

		}

		if !config.Compact && len(keys) > 0 {
			utils.Must(w.Write(LF_CR))
		}
		utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
		utils.PanicIfErr(w.WriteByte('}'))
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%dictionary")))
	return
}

func (d *Dictionary) WidestOfType() SymbolicValue {
	return &Dictionary{}
}

//

type Object struct {
	entries                    map[string]Serializable //if nil, matches any object
	optionalEntries            map[string]struct{}
	static                     map[string]Pattern //key in .Static => key in .Entries, not reciprocal
	complexPropertyConstraints []*ComplexPropertyConstraint
	shared                     bool

	SerializableMixin
}

func NewAnyObject() *Object {
	return &Object{}
}

func NewEmptyObject() *Object {
	return &Object{entries: map[string]Serializable{}}
}

func NewObject(entries map[string]Serializable, optionalEntries map[string]struct{}, static map[string]Pattern) *Object {
	obj := &Object{
		entries:         entries,
		optionalEntries: optionalEntries,
		static:          static,
	}
	return obj
}

func NewUnitializedObject() *Object {
	return &Object{}
}

func InitializeObject(obj *Object, entries map[string]Serializable, static map[string]Pattern) {
	if obj.entries != nil {
		panic(errors.New("object is already initialized"))
	}
	obj.entries = entries
	obj.static = static
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

func (obj *Object) Test(v SymbolicValue) bool {
	otherObj, ok := v.(*Object)
	if !ok {
		return false
	}

	if obj.entries == nil {
		return true
	}

	if (len(obj.optionalEntries) == 0 && len(obj.entries) != len(otherObj.entries)) || otherObj.entries == nil {
		return false
	}

	for k, e := range obj.entries {
		_, isOptional := obj.optionalEntries[k]
		_, isOptionalInOther := otherObj.optionalEntries[k]

		other, ok := otherObj.entries[k]

		if ok && !isOptional && isOptionalInOther {
			return false
		}

		if !ok {
			if isOptional {
				continue
			}
			return false
		}
		if !e.Test(other) {
			return false
		}
	}

	return true
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
		entries: utils.CopyMap(obj.entries),
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

func (obj *Object) Prop(name string) SymbolicValue {
	v, ok := obj.entries[name]
	if !ok {
		panic(fmt.Errorf("object does not have a .%s property", name))
	}
	return v
}

func (obj *Object) MatchAnyObject() bool {
	return obj.entries == nil
}

func (obj *Object) ForEachEntry(fn func(k string, v SymbolicValue) error) error {
	for k, v := range obj.entries {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (obj *Object) ValueEntryMap() map[string]SymbolicValue {
	entries := map[string]SymbolicValue{}
	for k, v := range obj.entries {
		entries[k] = v
	}
	return entries
}

func (obj *Object) SetProp(name string, value SymbolicValue) (IProps, error) {
	if obj.entries == nil {
		return ANY_OBJ, nil
	}
	if _, ok := obj.entries[name]; ok { // update property

		if static, ok := obj.static[name]; ok {
			if !static.TestValue(value) {
				return nil, errors.New(fmtNotAssignableToPropOfType(value, static))
			}
		} else if prevValue, ok := obj.entries[name]; ok {
			if !prevValue.Test(value) {
				return nil, errors.New(fmtNotAssignableToPropOfType(value, &TypePattern{val: prevValue}))
			}
		}

		modified := *obj
		modified.entries = utils.CopyMap(obj.entries)
		modified.entries[name] = value.(Serializable)

		return &modified, nil
	}

	//new property

	modified := *obj
	modified.entries = utils.CopyMap(obj.entries)
	modified.entries[name] = value.(Serializable)
	return &modified, nil
}

func (obj *Object) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	modified := *obj
	modified.entries = utils.CopyMap(obj.entries)
	modified.optionalEntries = utils.CopyMap(obj.optionalEntries)
	modified.entries[name] = value.(Serializable)
	delete(modified.optionalEntries, name)

	return &modified, nil
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
	return utils.GetMapKeys(obj.optionalEntries)
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

// result should not be modfied
func (obj *Object) GetProperty(name string) (SymbolicValue, Pattern, bool) {
	if obj.entries == nil {
		return ANY, nil, true
	}
	v, ok := obj.entries[name]
	return v, obj.static[name], ok
}

func (obj *Object) AddStatic(pattern Pattern) {
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
	} else {
		panic(errors.New("cannot add static information of non object pattern"))
	}
}

func (o *Object) HasKnownLen() bool {
	return false
}

func (o *Object) KnownLen() int {
	return -1
}

func (o *Object) element() SymbolicValue {
	return ANY
}

func (*Object) elementAt(i int) SymbolicValue {
	return ANY
}

func (o *Object) Contains(ctx *Context, value SymbolicValue) (bool, bool) {
	if o.entries == nil {
		return false, true
	}

	possible := false

	for _, e := range o.entries {
		if e.Test(value) {
			possible = true
			if value.Test(e) {
				return true, true
			}
		}
	}
	return false, possible
}

func (o *Object) IteratorElementKey() SymbolicValue {
	return &String{}
}

func (o *Object) IteratorElementValue() SymbolicValue {
	return o.element()
}

func (o *Object) WatcherElement() SymbolicValue {
	return ANY
}

func (obj *Object) Widen() (SymbolicValue, bool) {
	if obj.entries == nil {
		return nil, false
	}

	widenedEntries := map[string]Serializable{}
	allAlreadyWidened := true

	for k, v := range obj.entries {
		widened, ok := v.Widen()
		if ok {
			allAlreadyWidened = false
			v = widened.(Serializable)
		}
		widenedEntries[k] = v
	}

	if allAlreadyWidened {
		return &Object{}, true
	}

	return &Object{
		entries:         widenedEntries,
		optionalEntries: obj.optionalEntries,
	}, true
}

func (obj *Object) IsWidenable() bool {
	_, ok := obj.Widen()
	return ok
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

func (obj *Object) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if obj.entries != nil {
		if depth > config.MaxDepth && len(obj.entries) > 0 {
			utils.Must(w.Write(utils.StringAsBytes("{(...)}")))
			return
		}

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		utils.Must(w.Write(utils.StringAsBytes("{")))

		keys := utils.GetMapKeys(obj.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))
			}

			if config.Colorize {
				utils.Must(w.Write(config.Colors.IdentifierLiteral))
			}

			utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(k))))

			if config.Colorize {
				utils.Must(w.Write(ANSI_RESET_SEQUENCE))
			}

			if _, isOptional := obj.optionalEntries[k]; isOptional {
				utils.PanicIfErr(w.WriteByte('?'))
			}

			//colon
			utils.Must(w.Write(COLON_SPACE))

			//value
			v := obj.entries[k]
			v.PrettyPrint(w, config, depth+1, indentCount)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				utils.Must(w.Write(COMMA_SPACE))
			}
		}

		if !config.Compact && len(keys) > 0 {
			utils.Must(w.Write(LF_CR))
		}

		utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%object")))
}

func (o *Object) WidestOfType() SymbolicValue {
	return ANY_OBJ
}

// A Record represents a symbolic Record.
type Record struct {
	UnassignablePropsMixin
	entries         map[string]Serializable //if nil, matches any record
	optionalEntries map[string]struct{}
	valueOnly       SymbolicValue

	SerializableMixin
}

func NewAnyrecord() *Record {
	return &Record{}
}

func NewEmptyRecord() *Record {
	return &Record{entries: map[string]Serializable{}}
}

func NewRecord(entries map[string]Serializable) *Record {
	return &Record{entries: entries}
}

func NewAnyKeyRecord(value SymbolicValue) *Record {
	return &Record{valueOnly: value}
}

func NewBoundEntriesRecord(entries map[string]Serializable) *Record {
	return &Record{entries: entries}
}

func (rec *Record) Test(v SymbolicValue) bool {
	otherRec, ok := v.(*Record)
	if !ok {
		return false
	}

	if rec.entries == nil {
		if rec.valueOnly == nil {
			return true
		}
		value := rec.valueOnly
		if otherRec.valueOnly == nil {
			return false
		}
		return value.Test(otherRec.valueOnly)
	}

	if (len(rec.optionalEntries) == 0 && len(rec.entries) != len(otherRec.entries)) || otherRec.entries == nil {
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
		if !e.Test(other) {
			return false
		}
	}

	return true
}

func (rec *Record) Prop(name string) SymbolicValue {
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
	return utils.GetMapKeys(rec.optionalEntries)
}

func (rec *Record) ValueEntryMap() map[string]SymbolicValue {
	entries := map[string]SymbolicValue{}
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

func (rec *Record) getProperty(name string) (SymbolicValue, bool) {
	if rec.entries == nil {
		return ANY, true
	}
	v, ok := rec.entries[name]
	return v, ok
}

func (rec *Record) HasKnownLen() bool {
	return false
}

func (rec *Record) KnownLen() int {
	return -1
}

func (rec *Record) element() SymbolicValue {
	return ANY
}

func (r *Record) Contains(ctx *Context, value SymbolicValue) (bool, bool) {
	if r.entries == nil {
		return false, true
	}

	possible := false

	for _, e := range r.entries {
		if e.Test(value) {
			possible = true
			if value.Test(e) {
				return true, true
			}
		}
	}
	return false, possible
}

func (rec *Record) IteratorElementKey() SymbolicValue {
	return &String{}
}

func (rec *Record) IteratorElementValue() SymbolicValue {
	return rec.element()
}

func (rec *Record) Widen() (SymbolicValue, bool) {
	if rec.entries == nil {
		if rec.valueOnly != nil {
			return &Record{}, true
		}
		return nil, false
	}

	widenedEntries := map[string]Serializable{}
	allAlreadyWidened := true

	for k, v := range rec.entries {
		widened, ok := v.Widen()
		if ok {
			allAlreadyWidened = false
			v = widened.(Serializable)
		}
		widenedEntries[k] = v
	}

	if allAlreadyWidened {
		return &Record{}, true
	}

	return &Record{entries: widenedEntries}, true
}

func (rec *Record) IsWidenable() bool {
	_, ok := rec.Widen()
	return ok
}

func (rec *Record) Static() Pattern {
	entries := map[string]Pattern{}

	for k, v := range rec.entries {
		entries[k] = getStatic(v)
	}

	return NewInexactObjectPattern(entries, rec.optionalEntries)
}

func (rec *Record) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if rec.entries != nil {
		if depth > config.MaxDepth && len(rec.entries) > 0 {
			utils.Must(w.Write(utils.StringAsBytes("#{(...)}")))
			return
		}

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		utils.Must(w.Write(utils.StringAsBytes("#{")))

		keys := utils.GetMapKeys(rec.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))
			}

			if config.Colorize {
				utils.Must(w.Write(config.Colors.IdentifierLiteral))
			}

			utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(k))))

			if config.Colorize {
				utils.Must(w.Write(ANSI_RESET_SEQUENCE))
			}

			if _, isOptional := rec.optionalEntries[k]; isOptional {
				utils.PanicIfErr(w.WriteByte('?'))
			}

			//colon
			utils.Must(w.Write(COLON_SPACE))

			//value
			v := rec.entries[k]
			v.PrettyPrint(w, config, depth+1, indentCount)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				utils.Must(w.Write(COMMA_SPACE))
			}
		}

		if !config.Compact && len(keys) > 0 {
			utils.Must(w.Write(LF_CR))
		}

		utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
		return
	}
	if rec.valueOnly == nil {
		utils.Must(w.Write(utils.StringAsBytes("%record")))
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("#{ any -> ")))
	rec.valueOnly.PrettyPrint(w, config, 0, parentIndentCount)
	utils.Must(w.Write(utils.StringAsBytes("}")))
}

func (r *Record) WidestOfType() SymbolicValue {
	return ANY_REC
}

// An AnyIndexable represents a symbolic Indesable we do not know the concrete type.
type AnyIndexable struct {
	_ int
}

func (r *AnyIndexable) Test(v SymbolicValue) bool {
	_, ok := v.(Indexable)

	return ok
}

func (i *AnyIndexable) IteratorElementKey() SymbolicValue {
	return ANY
}

func (i *AnyIndexable) IteratorElementValue() SymbolicValue {
	return ANY
}

func (i *AnyIndexable) element() SymbolicValue {
	return ANY
}

func (i *AnyIndexable) elementAt(index int) SymbolicValue {
	return ANY
}

func (i *AnyIndexable) KnownLen() int {
	return -1
}

func (i *AnyIndexable) HasKnownLen() bool {
	return false
}

func (r *AnyIndexable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnyIndexable) IsWidenable() bool {
	return false
}

func (r *AnyIndexable) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%indexable")))
}

func (r *AnyIndexable) WidestOfType() SymbolicValue {
	return ANY_INDEXABLE
}
