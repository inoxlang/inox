package internal

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
	_ = []Indexable{
		&String{}, &List{}, &Tuple{}, &RuneSlice{}, &ByteSlice{}, &Object{}, &IntRange{},
		&AnyStringLike{},
	}

	ANY_INDEXABLE = &AnyIndexable{}
	ANY_TUPLE     = NewTupleOf(ANY)
	ANY_OBJ       = &Object{}
	ANY_REC       = &Record{}
)

// An Indexable represents a symbolic Indexable.
type Indexable interface {
	SymbolicValue
	element() SymbolicValue
	elementAt(i int) SymbolicValue
	knownLen() int
	HasKnownLen() bool
}

// A List represents a symbolic List.
type List struct {
	elements       []SymbolicValue
	generalElement SymbolicValue
}

func NewList(elements ...SymbolicValue) *List {
	if elements == nil {
		elements = []SymbolicValue{}
	}
	return &List{elements: elements}
}

func NewListOf(generalElement SymbolicValue) *List {
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
		if _, ok := list.generalElement.(*Any); ok {
			return nil, false
		}
		return &List{generalElement: ANY}, true
	}

	allAny := true

	for _, elem := range list.elements {
		if _, ok := elem.(*Any); !ok {
			allAny = false
			break
		}
	}

	if allAny {
		return &List{generalElement: ANY}, true
	}

	widenedElements := make([]SymbolicValue, 0)
	noWidening := true
	for _, elem := range list.elements {
		widened, ok := elem.Widen()
		if ok {
			noWidening = false
			widenedElements = append(widenedElements, widened)
		} else {
			widenedElements = append(widenedElements, elem)
		}
	}
	if noWidening {
		return &List{generalElement: joinValues(widenedElements)}, true
	}
	return &List{elements: widenedElements}, true
}

func (list *List) IsWidenable() bool {
	return list.elements != nil || !IsAny(list.generalElement)
}

func (list *List) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if list.elements != nil {
		length := list.knownLen()

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
		}
		end = append(end, bytes.Repeat(config.Indent, depth)...)
		end = append(end, ']')

		utils.Must(w.Write(end))
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("[]")))
	list.generalElement.PrettyPrint(w, config, 0, 0)
}

func (a *List) HasKnownLen() bool {
	return a.elements != nil
}

func (a *List) knownLen() int {
	if a.elements == nil {
		panic("cannot get length of a symbolic list with no known length")
	}

	return len(a.elements)
}

func (a *List) element() SymbolicValue {
	if a.elements != nil {
		if len(a.elements) == 0 {
			return ANY
		}
		return joinValues(a.elements)
	}
	return a.generalElement
}

func (t *List) elementAt(i int) SymbolicValue {
	if t.elements != nil {
		if len(t.elements) == 0 || i >= len(t.elements) {
			return ANY // return "never" ?
		}
		return t.elements[i]
	}
	return t.generalElement
}

func (l *List) set(i *Int, v SymbolicValue) {

}

func (a *List) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (a *List) IteratorElementValue() SymbolicValue {
	return a.element()
}

func (a *List) WidestOfType() SymbolicValue {
	return &List{generalElement: ANY}
}

func (l *List) slice(start, end *Int) Sequence {
	if l.HasKnownLen() {
		return &List{generalElement: ANY}
	}
	return &List{
		generalElement: l.generalElement,
	}
}

func (l *List) setSlice(start, end *Int, v SymbolicValue) {

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
	elements       []SymbolicValue
	generalElement SymbolicValue
}

func NewTuple(elements ...SymbolicValue) *Tuple {
	l := &Tuple{elements: make([]SymbolicValue, 0)}
	for _, e := range elements {
		l.append(e)
	}
	return l
}

func NewTupleOf(generalElement SymbolicValue) *Tuple {
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
		if _, ok := t.generalElement.(*Any); ok {
			return nil, false
		}
		return &Tuple{generalElement: ANY}, true
	}

	allAny := true

	for _, elem := range t.elements {
		if _, ok := elem.(*Any); !ok {
			allAny = false
			break
		}
	}

	if allAny {
		return &Tuple{generalElement: ANY}, true
	}

	widenedElements := make([]SymbolicValue, 0)
	noWidening := true
	for _, elem := range t.elements {
		widened, ok := elem.Widen()
		if ok {
			noWidening = false
			widenedElements = append(widenedElements, widened)
		} else {
			widenedElements = append(widenedElements, elem)
		}
	}
	if noWidening {
		return &Tuple{generalElement: joinValues(widenedElements)}, true
	}
	return &Tuple{elements: widenedElements}, true
}

func (t *Tuple) IsWidenable() bool {
	return t.elements != nil || !IsAny(t.generalElement)
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
		t.elements = make([]SymbolicValue, 0)
	}

	t.elements = append(t.elements, element)
}

func (t *Tuple) HasKnownLen() bool {
	return t.elements != nil
}

func (t *Tuple) knownLen() int {
	if t.elements == nil {
		panic("cannot get length of a symbolic length with no known length")
	}

	return len(t.elements)
}

func (t *Tuple) element() SymbolicValue {
	if t.elements != nil {
		if len(t.elements) == 0 {
			return ANY // return "never" ?
		}
		return joinValues(t.elements)
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

func (t *Tuple) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (t *Tuple) IteratorElementValue() SymbolicValue {
	return t.element()
}

func (t *Tuple) WidestOfType() SymbolicValue {
	return &Tuple{generalElement: ANY}
}

func (t *Tuple) slice(start, end *Int) Sequence {
	if t.HasKnownLen() {
		return &Tuple{generalElement: ANY}
	}
	return &Tuple{
		generalElement: t.generalElement,
	}
}

//

type KeyList struct {
	Keys []string //if nil, matches any
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
	Entries map[string]SymbolicValue //if nil, matches any dictionary
	Keys    map[string]SymbolicValue
}

func NewAnyDictionary() *Dictionary {
	return &Dictionary{}
}

func NewDictionary(entries map[string]SymbolicValue, keys map[string]SymbolicValue) *Dictionary {
	return &Dictionary{
		Entries: entries,
		Keys:    keys,
	}
}

func (dict *Dictionary) Test(v SymbolicValue) bool {
	otherDict, ok := v.(*Dictionary)
	if !ok {
		return false
	}

	if dict.Entries == nil {
		return true
	}

	if len(dict.Entries) != len(otherDict.Entries) || otherDict.Entries == nil {
		return false
	}

	for i, e := range dict.Entries {
		if !e.Test(otherDict.Entries[i]) {
			return false
		}
	}
	return true
}

func (dict *Dictionary) hasKey(keyRepr string) bool {
	if dict.Entries == nil {
		return true
	}
	_, ok := dict.Keys[keyRepr]
	return ok
}

func (dict *Dictionary) get(keyRepr string) (SymbolicValue, bool) {
	if dict.Entries == nil {
		return ANY, true
	}
	v, ok := dict.Entries[keyRepr]
	return v, ok
}

func (dict *Dictionary) key() SymbolicValue {
	if dict.Entries != nil {
		if len(dict.Entries) == 0 {
			return ANY
		}
		var keys []SymbolicValue
		for _, k := range dict.Keys {
			keys = append(keys, k)
		}
		return joinValues(keys)
	}
	return ANY
}

func (dict *Dictionary) IteratorElementKey() SymbolicValue {
	return dict.key()
}

func (dict *Dictionary) IteratorElementValue() SymbolicValue {
	return ANY
}

func (dict *Dictionary) Widen() (SymbolicValue, bool) {
	if dict.Entries == nil {
		return nil, false
	}

	widenedEntries := map[string]SymbolicValue{}
	keys := map[string]SymbolicValue{}
	allAlreadyWidened := true

	for keyRepr, v := range dict.Entries {
		widened, ok := v.Widen()
		if ok {
			allAlreadyWidened = false
			v = widened
		}
		widenedEntries[keyRepr] = v
		keys[keyRepr] = dict.Keys[keyRepr]
	}

	if allAlreadyWidened {
		return &Dictionary{}, true
	}

	return &Dictionary{Entries: widenedEntries, Keys: keys}, true
}

func (dict *Dictionary) IsWidenable() bool {
	_, ok := dict.Widen()
	return ok
}

func (dict *Dictionary) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if dict.Entries != nil {
		if depth > config.MaxDepth && len(dict.Entries) > 0 {
			utils.Must(w.Write(utils.StringAsBytes(":{(...)}")))
			return
		}

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		utils.Must(w.Write(utils.StringAsBytes(":{")))

		var keys []string
		for k := range dict.Entries {
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
			v := dict.Entries[k]

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
		utils.PanicIfErr(w.WriteByte(']'))
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
	entries                    map[string]SymbolicValue //if nil, matches any object
	static                     map[string]Pattern       //key in .Static => key in .Entries, not reciprocal
	complexPropertyConstraints []*ComplexPropertyConstraint
	shared                     bool
}

func NewAnyObject() *Object {
	return &Object{}
}

func NewEmptyObject() *Object {
	return &Object{entries: map[string]SymbolicValue{}}
}

func NewObject(entries map[string]SymbolicValue, static map[string]Pattern) *Object {
	obj := &Object{
		entries: entries,
		static:  static,
	}
	return obj
}

func NewUnitializedObject() *Object {
	return &Object{}
}

func InitializeObject(obj *Object, entries map[string]SymbolicValue, static map[string]Pattern) {
	if obj.entries != nil {
		panic(errors.New("object is already initialized"))
	}
	obj.entries = entries
	obj.static = static
}

func (obj *Object) initNewProp(key string, value SymbolicValue, static Pattern) {
	if obj.entries == nil {
		obj.entries = make(map[string]SymbolicValue, 1)
	}
	obj.entries[key] = value
	if static != nil {
		if obj.static == nil {
			obj.static = make(map[string]Pattern, 1)
		}
		obj.static[key] = static
	}

}

func (obj *Object) Test(v SymbolicValue) bool {
	otherObj, ok := v.(*Object)
	if !ok {
		return false
	}

	if obj.entries == nil {
		return true
	}

	if len(obj.entries) != len(otherObj.entries) || otherObj.entries == nil {
		return false
	}

	for i, e := range obj.entries {
		if !e.Test(otherObj.entries[i]) {
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
		if ok, expl := IsSharable(v); !ok {
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

		shared.entries[k] = newVal
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
		modified.entries[name] = value

		return &modified, nil
	}

	//new property

	modified := *obj
	modified.entries = utils.CopyMap(obj.entries)
	modified.entries[name] = value
	return &modified, nil
}

func (obj *Object) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	modified := *obj
	modified.entries = utils.CopyMap(obj.entries)
	modified.entries[name] = value
	return &modified, nil
}

func (obj *Object) PropertyNames() []string {
	if obj.entries == nil {
		return nil
	}
	props := make([]string, len(obj.entries))
	i := 0
	for k := range obj.entries {
		props[i] = k
		i++
	}
	return props
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

func (o *Object) knownLen() int {
	return -1
}

func (o *Object) element() SymbolicValue {
	return ANY
}

func (*Object) elementAt(i int) SymbolicValue {
	return ANY
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

	widenedEntries := map[string]SymbolicValue{}
	allAlreadyWidened := true

	for k, v := range obj.entries {
		widened, ok := v.Widen()
		if ok {
			allAlreadyWidened = false
			v = widened
		}
		widenedEntries[k] = v
	}

	if allAlreadyWidened {
		return &Object{}, true
	}

	return &Object{entries: widenedEntries}, true
}

func (obj *Object) IsWidenable() bool {
	_, ok := obj.Widen()
	return ok
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
	return
}

func (o *Object) WidestOfType() SymbolicValue {
	return ANY_OBJ
}

//

//

type Record struct {
	UnassignablePropsMixin
	entries   map[string]SymbolicValue //if nil, matches any record
	valueOnly SymbolicValue
}

func NewAnyrecord() *Record {
	return &Record{}
}

func NewEmptyRecord() *Record {
	return &Record{entries: map[string]SymbolicValue{}}
}

func NewRecord(entries map[string]SymbolicValue) *Record {
	return &Record{entries: entries}
}

func NewAnyKeyRecord(value SymbolicValue) *Record {
	return &Record{valueOnly: value}
}

func NewBoundEntriesRecord(entries map[string]SymbolicValue) *Record {
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

	if len(rec.entries) != len(otherRec.entries) || otherRec.entries == nil {
		return false
	}

	for i, e := range rec.entries {
		if !e.Test(otherRec.entries[i]) {
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
	props := make([]string, len(rec.entries))
	i := 0
	for k := range rec.entries {
		props[i] = k
		i++
	}
	return props
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

func (rec *Record) knownLen() int {
	return -1
}

func (rec *Record) element() SymbolicValue {
	return ANY
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

	widenedEntries := map[string]SymbolicValue{}
	allAlreadyWidened := true

	for k, v := range rec.entries {
		widened, ok := v.Widen()
		if ok {
			allAlreadyWidened = false
			v = widened
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

func (r *AnyIndexable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnyIndexable) IsWidenable() bool {
	return false
}

func (r *AnyIndexable) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%indexable")))
	return
}

func (r *AnyIndexable) WidestOfType() SymbolicValue {
	return ANY_INDEXABLE
}

func (r *AnyIndexable) IteratorElementValue() SymbolicValue {
	return ANY
}
