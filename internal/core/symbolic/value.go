package symbolic

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/commonfmt"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/slices"
)

const (
	MAX_RECURSIVE_TEST_CALL_DEPTH = 20
)

var (
	ErrWideSymbolicValue                   = errors.New("cannot create wide symbolic value")
	ErrNoSymbolicValue                     = errors.New("no symbolic value")
	ErrUnassignablePropsMixin              = errors.New("UnassignablePropsMixin")
	ErrMaximumSymbolicTestCallDepthReached = errors.New("maximum recursive Test() call depth reached, there is probably a cycle")

	ANY                = &Any{}
	NEVER              = &Never{}
	ANY_BOOL           = &Bool{}
	TRUE               = NewBool(true)
	FALSE              = NewBool(false)
	ANY_RES_NAME       = &AnyResourceName{}
	ANY_OPTION         = &Option{name: "", value: ANY_SERIALIZABLE}
	ANY_INT_RANGE      = &IntRange{}
	ANY_FLOAT_RANGE    = &FloatRange{}
	ANY_RUNE_RANGE     = &RuneRange{}
	ANY_QUANTITY_RANGE = &QuantityRange{element: ANY_SERIALIZABLE}
	ANY_FILEMODE       = &FileMode{}

	ANY_YEAR     = &Year{}
	ANY_DATE     = &Date{}
	ANY_DATETIME = &DateTime{}
	ANY_DURATION = &Duration{}

	ANY_BYTECOUNT  = &ByteCount{}
	ANY_LINECOUNT  = &LineCount{}
	ANY_RUNECOUNT  = &RuneCount{}
	ANY_BYTERATE   = &ByteRate{}
	ANY_SIMPLERATE = &SimpleRate{}
	ANY_IDENTIFIER = &Identifier{}
	ANY_PROPNAME   = &PropertyName{}
	ANY_EMAIL_ADDR = &EmailAddress{}
	ANY_FILEINFO   = &FileInfo{}
	ANY_MIMETYPE   = &Mimetype{}

	FILEINFO_PROPNAMES = []string{"name", "abs-path", "size", "mode", "mod-time", "is-dir"}
)

// A Value represents a Value during symbolic evaluation, its underlying data should be immutable.
type Value interface {
	Test(v Value, state RecTestCallState) bool

	IsMutable() bool

	WidestOfType() Value

	PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig)
}

func IsAny(val Value) bool {
	_, ok := val.(*Any)
	return ok
}

func IsAnySerializable(val Value) bool {
	_, ok := val.(*AnySerializable)
	return ok
}

func IsAnyOrAnySerializable(val Value) bool {
	switch val.(type) {
	case *Any, *AnySerializable:
		return true
	default:
		return false
	}
}

func isNever(val Value) bool {
	_, ok := val.(*Never)
	return ok
}

func deeplyMatch(v1, v2 Value) bool {
	return v1.Test(v2, RecTestCallState{}) && v2.Test(v1, RecTestCallState{})
}

type PseudoPropsValue interface {
	Value
	PropertyNames() []string
	Prop(name string) Value
}

type StaticDataHolder interface {
	Value

	//AddStatic returns a new StaticDataHolder with the added static data.
	AddStatic(Pattern) (StaticDataHolder, error)
}

var _ = []PseudoPropsValue{
	&Path{}, &PathPattern{}, &Host{}, &HostPattern{},
	&EmailAddress{}, &URLPattern{}, &CheckedString{}}

//symbolic value types with no data have a dummy field to avoid same address for empty structs

// An Any represents a SymbolicValue we do not know the concrete type.
type Any struct {
	_ int
}

func (a *Any) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return true
}

func (a *Any) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("any")
	return
}

func (a *Any) WidestOfType() Value {
	return ANY
}

// A Never represents a SymbolicValue that does not match against any value.
type Never struct {
	_ int
}

func (*Never) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Never)
	return ok
}

func (*Never) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("never")
}

func (*Never) WidestOfType() Value {
	return NEVER
}

var Nil = &NilT{}

// A NilT represents a symbolic NilT.
type NilT struct {
	SerializableMixin
}

func (n *NilT) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*NilT)
	return ok
}

func (*NilT) IsConcretizable() bool {
	return true
}

func (*NilT) Concretize(ctx ConcreteContext) any {
	return extData.ConcreteValueFactories.CreateNil()
}

func (n *NilT) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteString("nil")
}

func (n *NilT) WidestOfType() Value {
	return Nil
}

// A Bool represents a symbolic Bool.
type Bool struct {
	SerializableMixin
	value    bool
	hasValue bool
}

func NewBool(v bool) *Bool {
	return &Bool{
		value:    v,
		hasValue: true,
	}
}

func (b *Bool) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*Bool)
	if !ok {
		return false
	}
	if !b.hasValue {
		return true
	}
	return other.hasValue && b.value == other.value
}

func (b *Bool) IsConcretizable() bool {
	return b.hasValue
}

func (b *Bool) Concretize(ctx ConcreteContext) any {
	if !b.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateBool(b.value)
}

func (b *Bool) Static() Pattern {
	return &TypePattern{val: b.WidestOfType()}
}

func (b *Bool) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if b.hasValue {
		if b.value {
			w.WriteString("true")
		} else {
			w.WriteString("false")
		}
	} else {
		w.WriteName("boolean")
	}
}

func (b *Bool) WidestOfType() Value {
	return ANY_BOOL
}

// A EmailAddress represents a symbolic EmailAddress.
type EmailAddress struct {
	SerializableMixin
	value    string
	hasValue bool
}

func NewEmailAddress(v string) *EmailAddress {
	return &EmailAddress{
		value:    v,
		hasValue: true,
	}
}

func (e *EmailAddress) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*EmailAddress)
	if !ok {
		return false
	}
	if !e.hasValue {
		return true
	}
	return other.hasValue && e.value == other.value
}

func (e *EmailAddress) Static() Pattern {
	return &TypePattern{val: e.WidestOfType()}
}

func (e *EmailAddress) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("email-address")
	if e.hasValue {
		w.WriteByte('(')
		w.WriteString(e.value)
		w.WriteByte(')')
	}
}

func (e *EmailAddress) PropertyNames() []string {
	return []string{"username", "domain"}
}

func (*EmailAddress) Prop(name string) Value {
	switch name {
	case "username":
		return &String{}
	case "domain":
		return &Host{}
	default:
		return nil
	}
}

func (e *EmailAddress) WidestOfType() Value {
	return ANY_EMAIL_ADDR
}

// A Identifier represents a symbolic Identifier.
type Identifier struct {
	name string
	SerializableMixin
}

func NewIdentifier(name string) *Identifier {
	return &Identifier{name: name}
}

func (i *Identifier) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*Identifier)
	if !ok {
		return false
	}
	return i.name == "" || i.name == other.name
}

func (i *Identifier) IsConcretizable() bool {
	return i.HasConcreteName()
}

func (i *Identifier) Concretize(ctx ConcreteContext) any {
	if !i.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateIdentifier(i.name)
}

func (i *Identifier) HasConcreteName() bool {
	return i.name != ""
}

func (i *Identifier) Name() string {
	return i.name
}

func (i *Identifier) Static() Pattern {
	return &TypePattern{val: i.WidestOfType()}
}

func (i *Identifier) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if i.name == "" {
		w.WriteName("identifier")
		return
	}
	w.WriteStringF("#%s", i.name)
}

func (i *Identifier) underlyingString() *String {
	return &String{}
}

func (i *Identifier) WidestOfType() Value {
	return ANY_IDENTIFIER
}

// A PropertyName represents a symbolic PropertyName.
type PropertyName struct {
	name string
	SerializableMixin
}

func NewPropertyName(name string) *PropertyName {
	return &PropertyName{name: name}
}

func (n *PropertyName) Name() string {
	return n.name
}

func (p *PropertyName) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*PropertyName)
	if !ok {
		return false
	}
	return p.name == "" || p.name == other.name
}

func (n *PropertyName) IsConcretizable() bool {
	return n.name != ""
}

func (p *PropertyName) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreatePropertyName(p.name)
}

func (p *PropertyName) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *PropertyName) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.name == "" {
		w.WriteName("property-name")
		return
	}

	w.WriteNameF("property-name(#%s)", p.name)
}

func (s *PropertyName) underlyingString() *String {
	return &String{}
}

func (s *PropertyName) WidestOfType() Value {
	return ANY_PROPNAME
}

// A Mimetype represents a symbolic Mimetype.
type Mimetype struct {
	SerializableMixin
	value    string
	hasValue bool
}

func NewMimetype(v string) *Mimetype {
	return &Mimetype{
		value:    v,
		hasValue: true,
	}
}

func (m *Mimetype) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*Mimetype)
	if !ok {
		return false
	}
	if !m.hasValue {
		return true
	}
	return other.hasValue && m.value == other.value
}

func (m *Mimetype) Static() Pattern {
	return &TypePattern{val: m.WidestOfType()}
}

func (m *Mimetype) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("mimetype")
	if m.hasValue {
		w.WriteByte('(')
		w.WriteString(m.value)
		w.WriteByte(')')
	}
}

func (m *Mimetype) WidestOfType() Value {
	return ANY_MIMETYPE
}

// An Option represents a symbolic Option.
type Option struct {
	name  string //if "", any name is matched
	value Value
	SerializableMixin
	PseudoClonableMixin
}

func NewOption(name string, value Value) *Option {
	if name == "" {
		panic(errors.New("name should not be empty"))
	}
	return &Option{name: name, value: value}
}

func NewAnyNameOption(value Value) *Option {
	return &Option{value: value}
}

func (o *Option) Name() (string, bool) {
	if o.name == "" {
		return "", false
	}
	return o.name, true
}

func (o *Option) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherOpt, ok := v.(*Option)
	if !ok || (o.name != "" && o.name != otherOpt.name) {
		return false
	}

	return o.value.Test(otherOpt.value, RecTestCallState{})
}

func (o *Option) IsConcretizable() bool {
	if o.name == "" {
		return false
	}
	potentiallyConcretizable, ok := o.value.(PotentiallyConcretizable)

	return ok && potentiallyConcretizable.IsConcretizable()
}

func (o *Option) Concretize(ctx ConcreteContext) any {
	if !o.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concreteValue := utils.Must(Concretize(o.value, ctx))
	return extData.ConcreteValueFactories.CreateOption(o.name, concreteValue)
}

func (o *Option) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("option(")
	if o.name != "" {
		NewString(o.name).PrettyPrint(w.ZeroIndent(), config)
		w.WriteString(", ")
	}
	o.value.PrettyPrint(w.ZeroIndent(), config)
	w.WriteByte(')')
}

func (o *Option) WidestOfType() Value {
	return ANY_OPTION
}

// A FileMode represents a symbolic FileMode.
type FileMode struct {
	_ int
}

func (m *FileMode) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*FileMode)
	return ok
}

func (m *FileMode) Static() Pattern {
	return &TypePattern{val: m.WidestOfType()}
}

func (m *FileMode) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("filemode")
}

func (m *FileMode) WidestOfType() Value {
	return ANY_FILEMODE
}

// A FileInfo represents a symbolic FileInfo.
type FileInfo struct {
	UnassignablePropsMixin
	SerializableMixin
}

func (f *FileInfo) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*FileInfo)
	return ok
}

func (f *FileInfo) IsConcretizable() bool {
	return false
}

func (f *FileInfo) Concretize(ctx ConcreteContext) any {
	panic(ErrNotConcretizable)
}

func (f FileInfo) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (f *FileInfo) Prop(name string) Value {
	switch name {
	case "name":
		return ANY_STR
	case "abs-path":
		return ANY_PATH
	case "size":
		return ANY_BYTECOUNT
	case "mode":
		return ANY_FILEMODE
	case "mod-time":
		return ANY_DATETIME
	case "is-dir":
		return ANY_BOOL
	}
	method, ok := f.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, f))
	}
	return method
}

func (*FileInfo) PropertyNames() []string {
	return FILEINFO_PROPNAMES
}

func (f *FileInfo) Static() Pattern {
	return &TypePattern{val: f.WidestOfType()}
}

func (f *FileInfo) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("file-info")
}

func (f *FileInfo) WidestOfType() Value {
	return ANY_FILEINFO
}

// A Type represents a symbolic Type.
type Type struct {
	Type reflect.Type //if nil, any type is matched
}

func (t *Type) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*Type)
	if !ok {
		return false
	}
	if t.Type == nil {
		return true
	}

	if other.Type == nil {
		return false
	}

	return utils.SamePointer(t.Type, other.Type)
}

func (t *Type) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if t.Type == nil {
		w.WriteName("t")
		return
	}
	w.WriteNameF("type(%v)", t.Type)
}

func (t *Type) WidestOfType() Value {
	return &Type{}
}

type IProps interface {
	Value
	Prop(name string) Value

	// SetProp should be equivalent to .SetProp of a concrete IProps, the difference being that the original IProps should
	// not be modified since all symbolic values are immutable, an IProps with the modification should be returned.
	SetProp(name string, value Value) (IProps, error)

	// WithExistingPropReplaced should return a version of the Iprops with the replacement value of the given property.
	WithExistingPropReplaced(name string, value Value) (IProps, error)

	// returned slice should never be modified
	PropertyNames() []string
}

type UnassignablePropsMixin struct {
}

func (UnassignablePropsMixin) SetProp(name string, value Value) (IProps, error) {
	return nil, errors.New("unassignable properties")
}

func (UnassignablePropsMixin) WithExistingPropReplaced(name string, value Value) (IProps, error) {
	return nil, ErrUnassignablePropsMixin
}

type OptionalIProps interface {
	IProps
	OptionalPropertyNames() []string
}

func GetAllPropertyNames(v IProps) []string {
	names := slices.Clone(v.PropertyNames())
	if optIprops, ok := v.(OptionalIProps); ok {
		names = append(names, optIprops.OptionalPropertyNames()...)
	}
	return names
}

func IsPropertyOptional(v IProps, name string) bool {
	optIprops, ok := v.(OptionalIProps)
	if !ok {
		return false
	}
	for _, current := range optIprops.OptionalPropertyNames() {
		if name == current {
			return true
		}
	}
	return false
}

func HasRequiredOrOptionalProperty(v IProps, name string) bool {
	for _, current := range GetAllPropertyNames(v) {
		if name == current {
			return true
		}
	}
	return false
}

func HasRequiredProperty(v IProps, name string) bool {
	for _, current := range v.PropertyNames() {
		if name == current {
			return true
		}
	}
	return false
}

//

type GoValue interface {
	Value
	Prop(name string) Value
	PropertyNames() []string
	GetGoMethod(name string) (*GoFunction, bool)
}

func GetGoMethodOrPanic(name string, v GoValue) Value {
	method, ok := v.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, v))
	}
	return method
}

type Bytecode struct {
	Bytecode any //if nil, any function is matched
}

func (b *Bytecode) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*Bytecode)
	if !ok {
		return false
	}
	if b.Bytecode == nil {
		return true
	}

	if other.Bytecode == nil {
		return false
	}

	return utils.SamePointer(b.Bytecode, other.Bytecode)
}

func (b *Bytecode) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if b.Bytecode == nil {
		w.WriteName("bytecode")
		return
	}

	w.WriteNameF("bytecode(%v)", b.Bytecode)
}

func (b *Bytecode) WidestOfType() Value {
	return &Bytecode{}
}

// A QuantityRange represents a symbolic QuantityRange.
type QuantityRange struct {
	element Serializable
	SerializableMixin
}

func NewQuantityRange(element Serializable) *QuantityRange {
	return &QuantityRange{element: element}
}

func (r *QuantityRange) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*QuantityRange)
	return ok && r.element.Test(other.element, RecTestCallState{})
}

func (r *QuantityRange) IteratorElementKey() Value {
	return ANY_INT
}

func (r *QuantityRange) IteratorElementValue() Value {
	return r.element
}

func (r QuantityRange) Contains(value Serializable) (yes bool, possible bool) {
	if !r.element.Test(value, RecTestCallState{}) {
		return false, false
	}

	return false, true
}

func (r *QuantityRange) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("quantity-range")
}

func (r *QuantityRange) WidestOfType() Value {
	return &QuantityRange{}
}

// An IntRange represents a symbolic IntRange.
type IntRange struct {
	SerializableMixin
	hasValue bool

	//fields set if .hasValue is true

	inclusiveEnd bool
	start        *Int
	end          *Int
	isStepNotOne bool //only symbolic int ranges with a step of 1 are fully supported
}

func NewIncludedEndIntRange(start, end *Int) *IntRange {
	if !start.hasValue {
		panic(errors.New("lower bound has no value"))
	}
	if !end.hasValue {
		panic(errors.New("lower bound has no value"))
	}

	return &IntRange{
		hasValue:     true,
		inclusiveEnd: true,
		start:        start,
		end:          end,
	}
}

func NewExcludedEndIntRange(start, end *Int) *IntRange {
	if !start.hasValue {
		panic(errors.New("lower bound has no value"))
	}
	if !end.hasValue {
		panic(errors.New("lower bound has no value"))
	}

	return &IntRange{
		hasValue:     true,
		inclusiveEnd: false,
		start:        start,
		end:          end,
	}
}

func NewIntRange(start, end *Int, inclusiveEnd, isStepNotOne bool) *IntRange {
	if !start.hasValue {
		panic(errors.New("lower bound has no value"))
	}
	if !end.hasValue {
		panic(errors.New("lower bound has no value"))
	}

	return &IntRange{
		hasValue:     true,
		inclusiveEnd: inclusiveEnd,
		isStepNotOne: isStepNotOne,
		start:        start,
		end:          end,
	}
}

func (r *IntRange) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherRange, ok := v.(*IntRange)
	if !ok {
		return false
	}

	if !r.hasValue {
		return true
	}
	if !otherRange.hasValue {
		return false
	} //else boh ranges have a value

	return r.isStepNotOne == otherRange.isStepNotOne &&
		r.start == otherRange.start &&
		r.end == otherRange.end &&
		r.inclusiveEnd == otherRange.inclusiveEnd
}

func (r *IntRange) Static() Pattern {
	return &TypePattern{val: r.WidestOfType()}
}

func (r *IntRange) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if !r.hasValue {
		w.WriteName("int-range")
		return
	}

	if r.inclusiveEnd {
		w.WriteStringF("%d..%d", r.start.value, r.end.value)
	} else {
		w.WriteStringF("%d..<%d", r.start.value, r.end.value)
	}

	if r.isStepNotOne {
		w.WriteString("(step?)")
	}
}

func (r *IntRange) KnownLen() int {
	return -1
}

func (r *IntRange) element() Value {
	return &Int{
		hasValue:        false,
		matchingPattern: &IntRangePattern{intRange: r},
	}
}

func (*IntRange) elementAt(i int) Value {
	return ANY_INT
}

func (r *IntRange) InclusiveEnd() int64 {
	if r.inclusiveEnd {
		return r.end.value
	}
	return r.end.value - 1
}

func (r *IntRange) Contains(value Serializable) (yes bool, possible bool) {
	int, ok := value.(*Int)
	if !ok {
		return false, false
	}

	if int.matchingPattern != nil && r.Test(int.matchingPattern.intRange, RecTestCallState{}) {
		return true, true
	}

	if !r.hasValue || !int.hasValue {
		return false, true
	}

	contained := int.value >= r.start.value && int.value <= r.InclusiveEnd()

	if contained && r.isStepNotOne {
		return false, true
	}

	return contained, contained
}

func (r *IntRange) HasKnownLen() bool {
	return false
}

func (r *IntRange) IteratorElementKey() Value {
	return ANY_INT
}

func (r *IntRange) IteratorElementValue() Value {
	return r.element()
}

func (r *IntRange) WidestOfType() Value {
	return ANY_INT_RANGE
}

// An FloatRange represents a symbolic FloatRange.
type FloatRange struct {
	SerializableMixin
	hasValue bool

	//fields set if .hasValue is true

	inclusiveEnd bool
	start        *Float
	end          *Float
}

func NewIncludedEndFloatRange(start, end *Float) *FloatRange {
	if !start.hasValue {
		panic(errors.New("lower bound has no value"))
	}
	if !end.hasValue {
		panic(errors.New("lower bound has no value"))
	}

	return &FloatRange{
		hasValue:     true,
		inclusiveEnd: true,
		start:        start,
		end:          end,
	}
}

func NewExcludedEndFloatRange(start, end *Float) *FloatRange {
	if !start.hasValue {
		panic(errors.New("lower bound has no value"))
	}
	if !end.hasValue {
		panic(errors.New("lower bound has no value"))
	}

	return &FloatRange{
		hasValue:     true,
		inclusiveEnd: false,
		start:        start,
		end:          end,
	}
}

func NewFloatRange(start, end *Float, inclusiveEnd bool) *FloatRange {
	if !start.hasValue {
		panic(errors.New("lower bound has no value"))
	}
	if !end.hasValue {
		panic(errors.New("lower bound has no value"))
	}

	return &FloatRange{
		hasValue:     true,
		inclusiveEnd: inclusiveEnd,
		start:        start,
		end:          end,
	}
}

func (r *FloatRange) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherRange, ok := v.(*FloatRange)
	if !ok {
		return false
	}

	if !r.hasValue {
		return true
	}
	if !otherRange.hasValue {
		return false
	} //else boh ranges have a value

	return r.start == otherRange.start &&
		r.end == otherRange.end &&
		r.inclusiveEnd == otherRange.inclusiveEnd
}

func (r *FloatRange) Static() Pattern {
	return &TypePattern{val: r.WidestOfType()}
}

func (r *FloatRange) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if !r.hasValue {
		w.WriteName("float-range")
		return
	}

	//print start
	s := strconv.FormatFloat(r.start.value, 'g', -1, 64)
	w.WriteString(s)
	if !strings.ContainsAny(s, ".e") {
		w.WriteString(".0")
	}

	if r.inclusiveEnd {
		w.WriteString("..")
	} else {
		w.WriteString("..<")
	}

	//print end
	s = strconv.FormatFloat(r.end.value, 'g', -1, 64)
	w.WriteString(s)
	if !strings.ContainsAny(s, ".e") {
		w.WriteString(".0")
	}
}

func (r *FloatRange) InclusiveEnd() float64 {
	if r.inclusiveEnd || math.IsInf(r.end.value, 1) {
		return r.end.value
	}
	return math.Nextafter(r.end.value, math.Inf(-1))
}

func (r *FloatRange) Contains(value Serializable) (yes bool, possible bool) {
	float, ok := value.(*Float)
	if !ok {
		return false, false
	}

	if float.matchingPattern != nil && r.Test(float.matchingPattern.floatRange, RecTestCallState{}) {
		return true, true
	}

	if !r.hasValue || !float.hasValue {
		return false, true
	}

	contained := float.value >= r.start.value && float.value <= r.InclusiveEnd()
	return contained, contained
}

func (r *FloatRange) IteratorElementKey() Value {
	return ANY_INT
}

func (r *FloatRange) IteratorElementValue() Value {
	return ANY_FLOAT
}

func (r *FloatRange) WidestOfType() Value {
	return ANY_FLOAT_RANGE
}

// A RuneRange represents a symbolic RuneRange.
type RuneRange struct {
	_ int
	SerializableMixin
}

func (r *RuneRange) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*RuneRange)
	return ok
}

func (r *RuneRange) Static() Pattern {
	return &TypePattern{val: r.WidestOfType()}
}

func (r *RuneRange) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("rune-range")
}

func (r *RuneRange) KnownLen() int {
	return -1
}

func (r *RuneRange) element() Value {
	return &Rune{}
}

func (r *RuneRange) Contains(value Serializable) (bool, bool) {
	if _, ok := value.(*Rune); ok {
		return false, true
	}

	return false, false
}

func (r *RuneRange) HasKnownLen() bool {
	return false
}

func (r *RuneRange) IteratorElementKey() Value {
	return ANY_INT
}

func (r *RuneRange) IteratorElementValue() Value {
	return ANY_RUNE
}

func (r *RuneRange) WidestOfType() Value {
	return ANY_RUNE_RANGE
}

// A ByteCount represents a symbolic ByteCount.
type ByteCount struct {
	hasValue bool
	value    int64
	SerializableMixin
}

func NewByteCount(v int64) *ByteCount {
	return &ByteCount{
		hasValue: true,
		value:    v,
	}
}

func (c *ByteCount) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherCount, ok := v.(*ByteCount)
	if !ok {
		return false
	}

	if !c.hasValue {
		return true
	}

	return otherCount.hasValue && c.value == otherCount.value
}

func (c *ByteCount) IsConcretizable() bool {
	return c.hasValue
}

func (c *ByteCount) Concretize(ctx ConcreteContext) any {
	if !c.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateByteCount(c.value)
}

func (c *ByteCount) Static() Pattern {
	return &TypePattern{val: c.WidestOfType()}
}

func (c *ByteCount) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("byte-count")
	if c.hasValue {
		w.WriteByte('(')
		w.WriteString(utils.Must(commonfmt.FmtByteCount(c.value, -1)))
		w.WriteByte(')')
	}
}

func (c *ByteCount) WidestOfType() Value {
	return ANY_BYTECOUNT
}

// A ByteRate represents a symbolic ByteRate.
type ByteRate struct {
	hasValue bool
	value    int64
	SerializableMixin
}

func NewByteRate(v int64) *ByteRate {
	return &ByteRate{
		hasValue: true,
		value:    v,
	}
}

func (c *ByteRate) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherRate, ok := v.(*ByteRate)
	if !ok {
		return false
	}

	if !c.hasValue {
		return true
	}

	return otherRate.hasValue && c.value == otherRate.value
}

func (r *ByteRate) IsConcretizable() bool {
	return r.hasValue
}

func (r *ByteRate) Concretize(ctx ConcreteContext) any {
	if !r.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateByteRate(r.value)
}

func (r *ByteRate) Static() Pattern {
	return &TypePattern{val: r.WidestOfType()}
}

func (r *ByteRate) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("byte-rate")
	if r.hasValue {
		w.WriteByte('(')
		w.WriteString(utils.Must(commonfmt.FmtByteCount(r.value, -1)))
		w.WriteString("/s")
		w.WriteByte(')')
	}
}

func (r *ByteRate) WidestOfType() Value {
	return ANY_BYTERATE
}

type LineCount struct {
	hasValue bool
	value    int64
	SerializableMixin
}

func NewLineCount(v int64) *LineCount {
	return &LineCount{
		hasValue: true,
		value:    v,
	}
}

func (c *LineCount) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherCount, ok := v.(*LineCount)
	if !ok {
		return false
	}

	if !c.hasValue {
		return true
	}

	return otherCount.hasValue && c.value == otherCount.value
}

func (c *LineCount) IsConcretizable() bool {
	return c.hasValue
}

func (c *LineCount) Concretize(ctx ConcreteContext) any {
	if !c.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateLineCount(c.value)
}

func (c *LineCount) Static() Pattern {
	return &TypePattern{val: c.WidestOfType()}
}

func (c *LineCount) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("line-count")
	if c.hasValue {
		w.WriteByte('(')
		w.WriteString(strconv.FormatInt(c.value, 10))
		w.WriteByte(')')
	}
}

func (c *LineCount) WidestOfType() Value {
	return ANY_LINECOUNT
}

// A RuneCount represents a symbolic RuneCount.
type RuneCount struct {
	hasValue bool
	value    int64
	SerializableMixin
}

func NewRuneCount(v int64) *RuneCount {
	return &RuneCount{
		hasValue: true,
		value:    v,
	}
}

func (c *RuneCount) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherCount, ok := v.(*RuneCount)
	if !ok {
		return false
	}

	if !c.hasValue {
		return true
	}

	return otherCount.hasValue && c.value == otherCount.value
}

func (c *RuneCount) IsConcretizable() bool {
	return c.hasValue
}

func (c *RuneCount) Static() Pattern {
	return &TypePattern{val: c.WidestOfType()}
}

func (c *RuneCount) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("rune-count")
	if c.hasValue {
		w.WriteByte('(')
		w.WriteString(strconv.FormatInt(c.value, 10))
		w.WriteByte(')')
	}
}

func (c *RuneCount) WidestOfType() Value {
	return ANY_RUNECOUNT
}

// A SimpleRate represents a symbolic SimpleRate.
type SimpleRate struct {
	hasValue bool
	value    int64
	SerializableMixin
}

func NewSimpleRate(v int64) *SimpleRate {
	return &SimpleRate{
		hasValue: true,
		value:    v,
	}
}

func (c *SimpleRate) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherRate, ok := v.(*SimpleRate)
	if !ok {
		return false
	}

	if !c.hasValue {
		return true
	}

	return otherRate.hasValue && c.value == otherRate.value
}

func (r *SimpleRate) IsConcretizable() bool {
	return r.hasValue
}

func (r *SimpleRate) Concretize(ctx ConcreteContext) any {
	if !r.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateSimpleRate(r.value)
}

func (r *SimpleRate) Static() Pattern {
	return &TypePattern{val: r.WidestOfType()}
}

func (r *SimpleRate) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("simple-rate")
	if r.hasValue {
		w.WriteByte('(')
		w.WriteString(strconv.FormatInt(r.value, 10))
		w.WriteName("x/s")
		w.WriteByte(')')
	}
}

func (r *SimpleRate) WidestOfType() Value {
	return ANY_SIMPLERATE
}

type ResourceName interface {
	WrappedString
	ResourceName() *String
}

type AnyResourceName struct {
	_ int
}

func (r *AnyResourceName) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case ResourceName:
		return true
	default:
		return false
	}
}

func (r *AnyResourceName) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("resource-name")
}

func (r *AnyResourceName) underlyingString() *String {
	return &String{}
}

func (r *AnyResourceName) ResourceName() *String {
	return &String{}
}

func (r *AnyResourceName) WidestOfType() Value {
	return ANY_RES_NAME
}

//
//

// A Port represents a symbolic Port.
type Port struct {
	_ int
}

func (p *Port) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Port)
	return ok
}

func (p *Port) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *Port) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("port")
}

func (p *Port) WidestOfType() Value {
	return ANY_PORT
}

// A Udata represents a symbolic UData.
type UData struct {
	_ int
	SerializableMixin
}

func (i *UData) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*UData)
	return ok
}

func (*UData) WalkerElement() Value {
	return ANY
}

func (*UData) WalkerNodeMeta() Value {
	return Nil
}

func (i *UData) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("udata")
}

func (i *UData) WidestOfType() Value {
	return &UData{}
}

// A UDataHiearchyEntry represents a symbolic UData.
type UDataHiearchyEntry struct {
	_ int
}

func (i *UDataHiearchyEntry) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*UDataHiearchyEntry)
	return ok
}

func (i *UDataHiearchyEntry) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("udata-hiearchy-entry")
}

func (i *UDataHiearchyEntry) WidestOfType() Value {
	return &UDataHiearchyEntry{}
}

func IsSimpleSymbolicInoxVal(v Value) bool {
	switch v.(type) {
	case *NilT, *Rune, *Byte, *Bool, *Int, *Float, WrappedString, *Port:
		return true
	default:
		return false
	}
}

// func valueOfSymbolic(v any) SymbolicValue {
// 	if isSymbolicInoxValue(v) {
// 		return v.(SymbolicValue)
// 	}
// 	switch val := v.(type) {
// 	case *SymbolicGoValue:

// 		if val.hasVal && isSymbolicInoxValue(val.wrapped) {
// 			return val.wrapped.(SymbolicValue)
// 		}

// 		return cloneSymbolicValue(val, nil)
// 	default:
// 		if rVal, ok := v.(GoValue); ok {
// 			v = rVal.Interface()
// 		}
// 		return &SymbolicGoValue{
// 			hasVal:  true,
// 			wrapped: v,
// 		}
// 	}
// }

type RecTestCallState struct {
	depth int64
}

func (s *RecTestCallState) StartCall() {
	s.depth++
	s.check()
}
func (s *RecTestCallState) FinishCall() {
	s.depth--
}

func (s RecTestCallState) check() {
	if s.depth > MAX_RECURSIVE_TEST_CALL_DEPTH {
		panic(ErrMaximumSymbolicTestCallDepthReached)
	}
}
