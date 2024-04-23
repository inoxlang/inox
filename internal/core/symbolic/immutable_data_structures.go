package symbolic

import (
	"fmt"
	"sort"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

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
	return extData.ConcreteValueFactories.CreateTuple(concreteElements)
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

func (t *Tuple) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w = w.IncrDepth()

	if t.elements != nil {
		lst := NewList(t.elements...)
		w.WriteByte('#')
		lst.PrettyPrint(w, config)
		return
	}
	w.WriteString("#[]")
	t.generalElement.PrettyPrint(w.ZeroIndent(), config)
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

func (t *Tuple) Element() Value {
	if t.elements != nil {
		if len(t.elements) == 0 {
			return ANY_SERIALIZABLE // return "never" ?
		}
		return AsSerializableChecked(joinValues(SerializablesToValues(t.elements)))
	}
	return t.generalElement
}

func (t *Tuple) ElementAt(i int) Value {
	if t.elements != nil {
		if len(t.elements) == 0 || i >= len(t.elements) {
			return ANY // return "never" ?
		}
		return t.elements[i]
	}
	return t.generalElement
}

func (t *Tuple) Contains(value Serializable) (bool, bool) {
	if t.elements == nil {
		if t.generalElement.Test(value, RecTestCallState{}) {
			return false, true
		}
		return false, false
	}

	possible := false
	isValueConcretizable := IsConcretizable(value)

	for _, e := range t.elements {
		if e.Test(value, RecTestCallState{}) {
			possible = true
			if isValueConcretizable && value.Test(e, RecTestCallState{}) {
				return true, true
			}
		} else if !possible && value.Test(e, RecTestCallState{}) {
			possible = true
		}
	}
	return false, possible
}

func (t *Tuple) IteratorElementKey() Value {
	return ANY_INT
}

func (t *Tuple) IteratorElementValue() Value {
	return t.Element()
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

// A OrderedPair represents a symbolic OrderedPair.
type OrderedPair struct {
	elements [2]Serializable

	SerializableMixin
}

func NewOrderedPair(first, second Serializable) *OrderedPair {
	return &OrderedPair{
		elements: [2]Serializable{first, second},
	}
}

func NewUnitializedOrderedPair() *OrderedPair {
	return &OrderedPair{}
}

func InitializeOrderedPair(pair *OrderedPair, first, second Serializable) {
	pair.elements[0] = first
	pair.elements[1] = second
}

func (p *OrderedPair) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPair, ok := v.(*OrderedPair)
	if !ok {
		return false
	}

	return p.elements[0].Test(otherPair.elements[0], state) &&
		p.elements[1].Test(otherPair.elements[1], state)
}

func (p *OrderedPair) IsConcretizable() bool {
	return IsConcretizable(p.elements[0]) && IsConcretizable(p.elements[1])
}

func (t *OrderedPair) Concretize(ctx ConcreteContext) any {
	if !t.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	first := utils.Must(Concretize(t.elements[0], ctx))
	second := utils.Must(Concretize(t.elements[1], ctx))

	return extData.ConcreteValueFactories.CreateOrderedPair(first, second)
}

func (t *OrderedPair) Static() Pattern {
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

func (t *OrderedPair) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w = w.IncrDepth()
	w.WriteName("ordered-pair")

	if t.elements[0] == nil || t.elements[1] == nil {
		return
	}

	w.WriteString("(\n")
	t.elements[0].PrettyPrint(w.IncrIndent(), config)
	t.elements[1].PrettyPrint(w.IncrIndent(), config)
}

func (t *OrderedPair) HasKnownLen() bool {
	return true
}

func (t *OrderedPair) KnownLen() int {
	return 2
}

func (t *OrderedPair) Element() Value {
	return joinValues([]Value{t.elements[0], t.elements[1]})
}

func (t *OrderedPair) ElementAt(i int) Value {
	switch i {
	case 0:
		return t.elements[0]
	case 1:
		return t.elements[1]
	default:
		return NEVER
	}
}

func (p *OrderedPair) Contains(value Serializable) (bool, bool) {
	possible := false
	isValueConcretizable := IsConcretizable(value)

	for _, e := range p.elements {
		if e.Test(value, RecTestCallState{}) {
			possible = true
			if isValueConcretizable && value.Test(e, RecTestCallState{}) {
				return true, true
			}
		} else if !possible && value.Test(e, RecTestCallState{}) {
			possible = true
		}
	}
	return false, possible
}

func (t *OrderedPair) IteratorElementKey() Value {
	return ANY_INT
}

func (t *OrderedPair) IteratorElementValue() Value {
	return t.Element()
}

func (t *OrderedPair) WidestOfType() Value {
	return ANY_ORDERED_PAIR
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

	return extData.ConcreteValueFactories.CreateKeyList(slices.Clone(list.Keys))
}

func (list *KeyList) HasKnownLen() bool {
	return list.IsConcretizable()
}

func (list *KeyList) KnownLen() int {
	if !list.HasKnownLen() {
		panic("cannot get the length of a symbolic keylist with no known length")
	}
	return len(list.Keys)
}

func (list *KeyList) Element() Value {
	return ANY_STR_LIKE
}

func (list *KeyList) ElementAt(i int) Value {
	return ANY_STR_LIKE
}

func (list *KeyList) IteratorElementKey() Value {
	return ANY_INT
}

func (list *KeyList) IteratorElementValue() Value {
	return list.Element()
}

func (list *KeyList) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w = w.IncrDepth()

	if list.Keys != nil {
		if w.Depth > config.MaxDepth && len(list.Keys) > 0 {
			w.WriteString(".{(...)]}")
			return
		}

		w.WriteString(".{")

		first := true

		for _, k := range list.Keys {
			if !first {
				w.WriteString(", ")
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
	return extData.ConcreteValueFactories.CreateRecord(concreteProperties)
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
	sort.Strings(props)
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

// result should not be modfied
func (rec *Record) GetProperty(name string) (Value, Pattern, bool) {
	v, ok := rec.getProperty(name)
	return v, nil, ok
}

func (rec *Record) IsExistingPropertyOptional(name string) bool {
	_, ok := rec.optionalEntries[name]
	return ok
}

// HasPropertyOptionalOrNot returns if the property $name is listed in the object's entries,
// the property may be optional.
func (rec *Record) HasPropertyOptionalOrNot(name string) bool {
	if rec.entries == nil {
		return true
	}
	_, ok := rec.entries[name]
	return ok
}

func (rec *Record) ForEachEntry(fn func(k string, v Value) error) error {
	for k, v := range rec.entries {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (r *Record) Contains(value Serializable) (bool, bool) {
	if r.entries == nil {
		return false, true
	}

	possible := false
	isValueConcretizable := IsConcretizable(value)

	for _, e := range r.entries {
		if e.Test(value, RecTestCallState{}) {
			possible = true
			if isValueConcretizable && value.Test(e, RecTestCallState{}) {
				return true, true
			}
		} else if !possible && value.Test(e, RecTestCallState{}) {
			possible = true
		}
	}
	return false, possible
}

func (rec *Record) IteratorElementKey() Value {
	return &String{}
}

func (rec *Record) IteratorElementValue() Value {
	//TODO: properly implement for exact records.
	return ANY_SERIALIZABLE
}

func (rec *Record) Static() Pattern {
	entries := map[string]Pattern{}

	for k, v := range rec.entries {
		entries[k] = getStatic(v)
	}

	return NewInexactObjectPattern(entries, rec.optionalEntries)
}

func (rec *Record) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w = w.IncrDepth()

	if rec.entries != nil {
		if w.Depth > config.MaxDepth && len(rec.entries) > 0 {
			w.WriteString("#{(...)}")
			return
		}

		w.WriteString("#{")

		keys := maps.Keys(rec.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				w.WriteEndOfLine()
				w.WriteInnerIndent()
			}

			if config.Colorize {
				w.WriteBytes(config.Colors.IdentifierLiteral)
			}

			w.WriteBytes(utils.Must(utils.MarshalJsonNoHTMLEspace(k)))

			if config.Colorize {
				w.WriteAnsiReset()
			}

			if _, isOptional := rec.optionalEntries[k]; isOptional {
				w.WriteByte('?')
			}

			//colon
			w.WriteString(": ")

			//value
			v := rec.entries[k]
			v.PrettyPrint(w.IncrIndent(), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				w.WriteString(", ")
			}
		}

		if !config.Compact && len(keys) > 0 {
			w.WriteEndOfLine()
			w.WriteOuterIndent()
		}

		w.WriteByte('}')
		return
	}
	if rec.valueOnly == nil {
		w.WriteName("record")
		return
	}
	w.WriteString("#{ any -> ")
	rec.valueOnly.PrettyPrint(w.ZeroIndent(), config)
	w.WriteString("}")
}

func (r *Record) WidestOfType() Value {
	return ANY_REC
}
