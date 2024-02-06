package symbolic

import (
	"bytes"
	"strconv"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	LIST_APPEND_PARAM_NAMES = []string{"values"}

	LIST_OF_SERIALIZABLES = NewListOf(ANY_SERIALIZABLE)
)

// An Indexable represents a symbolic Indexable.
type Indexable interface {
	Iterable
	Element() Value
	ElementAt(i int) Value
	KnownLen() int
	HasKnownLen() bool
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

func (a *Array) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
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
					w.WriteColonSpace()
					if config.Colorize {
						w.WriteAnsiReset()
					}
				}
			}

			//element
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == length-1

			if !isLastEntry {
				w.WriteCommaSpace()
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

func (a *Array) Element() Value {
	if a.elements != nil {
		if len(a.elements) == 0 {
			return ANY
		}
		return joinValues(a.elements)
	}
	return a.generalElement
}

func (a *Array) ElementAt(i int) Value {
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
	return a.Element()
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
	ClonableSerializableMixin
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
	case "dequeue":
		return WrapGoMethod(list.Dequeue)
	case "pop":
		return WrapGoMethod(list.Pop)
	case "sorted":
		return WrapGoMethod(list.Sorted)
	case "sort_by":
		return WrapGoMethod(list.SortBy)
	case "len":
		return ANY_INT
	default:
		panic(FormatErrPropertyDoesNotExist(name, list))
	}
}

func (list *List) PropertyNames() []string {
	return LIST_PROPNAMES
}

func (list *List) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
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
					w.WriteColonSpace()
					if config.Colorize {
						w.WriteAnsiReset()
					}
				}
			}

			//element
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == length-1

			if !isLastEntry {
				w.WriteCommaSpace()
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

func (l *List) Element() Value {
	if l.elements != nil {
		if len(l.elements) == 0 {
			return ANY_SERIALIZABLE
		}
		return AsSerializableChecked(joinValues(SerializablesToValues(l.elements)))
	}
	return l.generalElement
}

func (l *List) ElementAt(i int) Value {
	if l.elements != nil {
		if len(l.elements) == 0 || i >= len(l.elements) {
			return ANY // return "never" ?
		}
		return l.elements[i]
	}
	return l.generalElement
}

func (l *List) Contains(value Serializable) (bool, bool) {
	isValueConcretizable := IsConcretizable(value)

	if l.elements == nil {
		if l.generalElement.Test(value, RecTestCallState{}) {
			if isValueConcretizable && value.Test(l.generalElement, RecTestCallState{}) {
				return true, true
			}
			return false, true
		}
		return false, value.Test(l.generalElement, RecTestCallState{})
	}

	if deeplyMatch(value, ANY_SERIALIZABLE) {
		return false, true
	}

	possible := false

	for _, e := range l.elements {
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

func (l *List) IteratorElementKey() Value {
	return ANY_INT
}

func (l *List) IteratorElementValue() Value {
	return l.Element()
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
	//do nothing if no elements are inserted.
	if seq.HasKnownLen() && seq.KnownLen() == 0 {
		return
	}

	if l.HasKnownLen() && l.KnownLen() == 0 {
		element := seq.Element()
		if serializable, ok := element.(Serializable); ok {
			//we could pass a list with a known length but we don't know how many times
			//the mutation can ocurr (e.g. in for loops).
			ctx.SetUpdatedSelf(NewListOf(serializable))
		} else {
			ctx.AddSymbolicGoFunctionError(NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE)
		}
		return
	}

	element := AsSerializable(MergeValuesWithSameStaticTypeInMultivalue(joinValues([]Value{l.Element(), seq.Element()})))
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

	//do nothing if no elements are appended.
	if seq.HasKnownLen() && seq.KnownLen() == 0 {
		return
	}

	if l.HasKnownLen() && l.KnownLen() == 0 {
		element := seq.Element()
		if serializable, ok := element.(Serializable); ok {
			//we could pass a list with a known length but we don't know how many times
			//the mutation can ocurr (e.g. in for loops).
			ctx.SetUpdatedSelf(NewListOf(serializable))
		} else {
			ctx.AddSymbolicGoFunctionError(NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE)
		}
		return
	}

	element := AsSerializable(MergeValuesWithSameStaticTypeInMultivalue(joinValues([]Value{l.Element(), seq.Element()})))
	if serializable, ok := element.(Serializable); ok {
		ctx.SetUpdatedSelf(NewListOf(serializable))
	} else {
		ctx.AddSymbolicGoFunctionError(NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE)
	}
}

func (l *List) Append(ctx *Context, elements ...Serializable) {
	if l.generalElement != nil {
		ctx.SetSymbolicGoFunctionParameters(&[]Value{l.Element()}, LIST_APPEND_PARAM_NAMES)
	}
	l.appendSequence(ctx, NewList(elements...))
}

func (l *List) Dequeue(ctx *Context) Serializable {
	if l.generalElement == nil && l.HasKnownLen() {
		if l.KnownLen() == 0 {
			ctx.AddSymbolicGoFunctionError(CANNOT_DEQUEUE_FROM_EMPTY_LIST)
			return ANY_SERIALIZABLE
		} else {
			elements := l.elements[1:len(l.elements)]
			ctx.SetUpdatedSelf(NewList(elements...))
		}
		return l.ElementAt(0).(Serializable)
	}
	return l.Element().(Serializable)
}

func (l *List) Pop(ctx *Context) Serializable {
	if l.generalElement == nil && l.HasKnownLen() {
		if l.KnownLen() == 0 {
			ctx.AddSymbolicGoFunctionError(CANNOT_POP_FROM_EMPTY_LIST)
			return ANY_SERIALIZABLE
		} else {
			elements := l.elements[:len(l.elements)-1]
			ctx.SetUpdatedSelf(NewList(elements...))
		}
		return l.ElementAt(l.KnownLen() - 1).(Serializable)
	}
	return l.Element().(Serializable)
}

func (l *List) Sorted(ctx *Context, orderIdent *Identifier) *List {
	if l.HasKnownLen() && l.KnownLen() == 0 {
		return l
	}

	if !orderIdent.HasConcreteName() {
		ctx.AddSymbolicGoFunctionError("invalid order identifier")
		return l
	}

	order, ok := OrderFromString(orderIdent.Name())
	if !ok {
		ctx.AddSymbolicGoFunctionErrorf("unknown order %q", orderIdent.Name())
		return l
	}

	switch MergeValuesWithSameStaticTypeInMultivalue(l.IteratorElementValue()).(type) {
	case *Int:
		switch order {
		case AscendingOrder, DescendingOrder:
		default:
			ctx.AddFormattedSymbolicGoFunctionError("invalid order '%s' for integers, use #asc or #desc", orderIdent.Name())
		}

	case *Float:
		switch order {
		case AscendingOrder, DescendingOrder:
		default:
			ctx.AddFormattedSymbolicGoFunctionError("invalid order '%s' for floats, use #asc or #desc", orderIdent.Name())
		}
	case StringLike:
		switch order {
		case LexicographicOrder, ReverseLexicographicOrder:
		default:
			ctx.AddFormattedSymbolicGoFunctionError("invalid order '%s' for strings, use #lex or #revlex", orderIdent.Name())
		}
	default:
		ctx.AddSymbolicGoFunctionError("list should contain only integers, only floats or only strings")
	}

	if l.HasKnownLen() {
		return NewListOf(l.Element().(Serializable))
	}

	return l
}

func (l *List) SortBy(ctx *Context, valuePath ValuePath, orderIdent *Identifier) {
	if l.HasKnownLen() && l.KnownLen() == 0 {
		return
	}

	if !orderIdent.HasConcreteName() {
		ctx.AddSymbolicGoFunctionError("invalid order identifier")
		return
	}

	order, ok := OrderFromString(orderIdent.Name())
	if !ok {
		ctx.AddSymbolicGoFunctionErrorf("unknown order %q", orderIdent.Name())
		return
	}

	elem := MergeValuesWithSameStaticTypeInMultivalue(l.Element())

	v, alwaysPresent, err := valuePath.GetFrom(elem)
	if err != nil {
		ctx.AddSymbolicGoFunctionError("invalid value path")
		return
	}

	if !alwaysPresent {
		ctx.AddSymbolicGoFunctionError("sorting value is not necessarily present for all elements")
	}

	switch MergeValuesWithSameStaticTypeInMultivalue(v).(type) {
	case *Int:
		switch order {
		case AscendingOrder, DescendingOrder:
		default:
			ctx.AddFormattedSymbolicGoFunctionError("invalid order '%s' for integers, use #asc or #desc", orderIdent.Name())
		}
	case *Float:
		switch order {
		case AscendingOrder, DescendingOrder:
		default:
			ctx.AddFormattedSymbolicGoFunctionError("invalid order '%s' for floats, use #asc or #desc", orderIdent.Name())
		}
	case StringLike:
		ctx.AddSymbolicGoFunctionError("sorting by a nested string is not supported yet")
		// switch order {
		// case LexicographicOrder, ReverseLexicographicOrder:
		// default:
		// 	ctx.AddFormattedSymbolicGoFunctionError("invalid order '%s' for strings, use #lex or #revlex", orderIdent.Name())
		// }
	default:
		ctx.AddSymbolicGoFunctionError("sorting values should be only integers, only floats or only strings")
	}

	ctx.SetUpdatedSelf(NewListOf(l.Element().(Serializable)))
}

func (l *List) WatcherElement() Value {
	return ANY
}
