package symbolic

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/inoxlang/inox/internal/commonfmt"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type Object struct {
	entries                    map[string]Serializable //if nil, matches any object
	optionalEntries            map[string]struct{}
	dependencies               map[string]propertyDependencies
	static                     map[string]Pattern //key in .Static => key in .Entries, not reciprocal
	complexPropertyConstraints []*ComplexPropertyConstraint
	shared                     bool
	exact                      bool
	readonly                   bool

	url *URL //can be nil

	SerializableMixin
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

func NewExactObject2(entries map[string]Serializable) *Object {
	return NewObject(true, entries, nil, nil)
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

	if obj.url != nil {
		if urlHolder, ok := v.(UrlHolder); ok {
			return urlHolder.WithURL(obj.url.WithAdditionalPathSegment(name))
		}
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

func (obj *Object) SerializableEntryMap() map[string]Serializable {
	entries := map[string]Serializable{}
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
	sort.Strings(props)
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

func (o *Object) Element() Value {
	return ANY
}

func (*Object) ElementAt(i int) Value {
	return ANY
}

func (o *Object) Contains(value Serializable) (bool, bool) {
	if o.entries == nil {
		return false, true
	}

	if deeplyMatch(value, ANY_SERIALIZABLE) {
		return false, true
	}

	possible := o.IsInexact() //if the object can have additional properties its always possible.
	isValueConcretizable := IsConcretizable(value)

	for _, e := range o.entries {
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

func (o *Object) IteratorElementKey() Value {
	return &String{}
}

func (o *Object) IteratorElementValue() Value {
	return o.Element()
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

func (obj *Object) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
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
				w.WriteAnsiReset()
			}

			if _, isOptional := obj.optionalEntries[k]; isOptional {
				w.WriteByte('?')
			}

			//colon
			w.WriteColonSpace()

			//value
			v := obj.entries[k]
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				w.WriteCommaSpace()
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
