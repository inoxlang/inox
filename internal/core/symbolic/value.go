package symbolic

import (
	"bufio"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrWideSymbolicValue      = errors.New("cannot create wide symbolic value")
	ErrNoSymbolicValue        = errors.New("no symbolic value")
	ErrUnassignablePropsMixin = errors.New("UnassignablePropsMixin")

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
	ANY_DATE           = &Date{}
	ANY_DURATION       = &Duration{}
	ANY_BYTECOUNT      = &ByteCount{}
	ANY_LINECOUNT      = &LineCount{}
	ANY_RUNECOUNT      = &RuneCount{}
	ANY_BYTERATE       = &ByteRate{}
	ANY_SIMPLERATE     = &SimpleRate{}
	ANY_IDENTIFIER     = &Identifier{}
	ANY_PROPNAME       = &PropertyName{}
	ANY_EMAIL_ADDR     = &EmailAddress{}
	ANY_FILEINFO       = &FileInfo{}
	ANY_MIMETYPE       = &Mimetype{}

	FILEINFO_PROPNAMES = []string{"name", "abs-path", "size", "mode", "mod-time", "is-dir"}
)

// A SymbolicValue represents a Value during symbolic evaluation, its underlying data should be immutable.
type SymbolicValue interface {
	Test(v SymbolicValue) bool

	IsMutable() bool

	WidestOfType() SymbolicValue

	PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int)
}

func IsAny(val SymbolicValue) bool {
	_, ok := val.(*Any)
	return ok
}

func IsAnySerializable(val SymbolicValue) bool {
	_, ok := val.(*AnySerializable)
	return ok
}

func IsAnyOrAnySerializable(val SymbolicValue) bool {
	switch val.(type) {
	case *Any, *AnySerializable:
		return true
	default:
		return false
	}
}

func isNever(val SymbolicValue) bool {
	_, ok := val.(*Never)
	return ok
}

func deeplyEqual(v1, v2 SymbolicValue) bool {
	return v1.Test(v2) && v2.Test(v1)
}

type PseudoPropsValue interface {
	SymbolicValue
	PropertyNames() []string
	Prop(name string) SymbolicValue
}

type StaticDataHolder interface {
	SymbolicValue

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

func (a *Any) Test(v SymbolicValue) bool {
	return true
}

func (a *Any) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%any")))
	return
}

func (a *Any) WidestOfType() SymbolicValue {
	return ANY
}

// A Never represents a SymbolicValue that does not match against any value.
type Never struct {
	_ int
}

func (*Never) Test(v SymbolicValue) bool {
	_, ok := v.(*Never)
	return ok
}

func (*Never) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%never")))
}

func (*Never) WidestOfType() SymbolicValue {
	return NEVER
}

var Nil = &NilT{}

// A NilT represents a symbolic NilT.
type NilT struct {
	SerializableMixin
}

func (n *NilT) Test(v SymbolicValue) bool {
	_, ok := v.(*NilT)
	return ok
}

func (*NilT) IsConcretizable() bool {
	return true
}

func (*NilT) Concretize(ctx ConcreteContext) any {
	return extData.ConcreteValueFactories.CreateNil()
}

func (n *NilT) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("nil")))
}

func (n *NilT) WidestOfType() SymbolicValue {
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

func (b *Bool) Test(v SymbolicValue) bool {
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

func (b *Bool) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if b.hasValue {
		if b.value {
			utils.Must(w.Write(utils.StringAsBytes("true")))
		} else {
			utils.Must(w.Write(utils.StringAsBytes("false")))
		}
	} else {
		utils.Must(w.Write(utils.StringAsBytes("%boolean")))
	}
}

func (b *Bool) WidestOfType() SymbolicValue {
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

func (e *EmailAddress) Test(v SymbolicValue) bool {
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

func (e *EmailAddress) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%email-address")))
	if e.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(e.value)))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (e *EmailAddress) PropertyNames() []string {
	return []string{"username", "domain"}
}

func (*EmailAddress) Prop(name string) SymbolicValue {
	switch name {
	case "username":
		return &String{}
	case "domain":
		return &Host{}
	default:
		return nil
	}
}

func (e *EmailAddress) WidestOfType() SymbolicValue {
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

func (i *Identifier) Test(v SymbolicValue) bool {
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

func (i *Identifier) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if i.name == "" {
		utils.Must(w.Write(utils.StringAsBytes("%identifier")))
		return
	}
	utils.Must(fmt.Fprintf(w, "#%s", i.name))
}

func (i *Identifier) underlyingString() *String {
	return &String{}
}

func (i *Identifier) WidestOfType() SymbolicValue {
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

func (p *PropertyName) Test(v SymbolicValue) bool {
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

func (p *PropertyName) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.name == "" {
		utils.Must(w.Write(utils.StringAsBytes("%property-name")))
		return
	}

	utils.Must(fmt.Fprintf(w, "%%property-name(#%s)", p.name))
}

func (s *PropertyName) underlyingString() *String {
	return &String{}
}

func (s *PropertyName) WidestOfType() SymbolicValue {
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

func (m *Mimetype) Test(v SymbolicValue) bool {
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

func (m *Mimetype) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%mimetype")))
	if m.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(m.value)))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (m *Mimetype) WidestOfType() SymbolicValue {
	return ANY_MIMETYPE
}

// An Option represents a symbolic Option.
type Option struct {
	name  string //if "", any name is matched
	value SymbolicValue
	SerializableMixin
	PseudoClonableMixin
}

func NewOption(name string, value SymbolicValue) *Option {
	if name == "" {
		panic(errors.New("name should not be empty"))
	}
	return &Option{name: name, value: value}
}

func NewAnyNameOption(value SymbolicValue) *Option {
	return &Option{value: value}
}

func (o *Option) Name() (string, bool) {
	if o.name == "" {
		return "", false
	}
	return o.name, true
}

func (o *Option) Test(v SymbolicValue) bool {
	otherOpt, ok := v.(*Option)
	if !ok || (o.name != "" && o.name != otherOpt.name) {
		return false
	}

	return o.value.Test(otherOpt.value)
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

func (o *Option) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%option(")))
	if o.name != "" {
		NewString(o.name).PrettyPrint(w, config, depth, 0)
		utils.Must(w.Write(utils.StringAsBytes(", ")))
	}
	o.value.PrettyPrint(w, config, depth, 0)
	utils.PanicIfErr(w.WriteByte(')'))
}

func (o *Option) WidestOfType() SymbolicValue {
	return ANY_OPTION
}

// A Date represents a symbolic Date.
type Date struct {
	SerializableMixin
	value    time.Time
	hasValue bool
}

func NewDate(v time.Time) *Date {
	return &Date{
		value:    v,
		hasValue: true,
	}
}

func (d *Date) Test(v SymbolicValue) bool {
	other, ok := v.(*Date)
	if !ok {
		return false
	}
	if !d.hasValue {
		return true
	}
	return other.hasValue && d.value == other.value
}

func (d *Date) IsConcretizable() bool {
	return d.hasValue
}

func (d *Date) Concretize(ctx ConcreteContext) any {
	if !d.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateDate(d.value)
}

func (d *Date) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%date")))
	if d.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(commonfmt.FmtInoxDate(d.value))))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (d *Date) WidestOfType() SymbolicValue {
	return ANY_DATE
}

// A Duration represents a symbolic Duration.
type Duration struct {
	SerializableMixin
	value    time.Duration
	hasValue bool
}

func NewDuration(v time.Duration) *Duration {
	return &Duration{
		value:    v,
		hasValue: true,
	}
}

func (d *Duration) Test(v SymbolicValue) bool {
	other, ok := v.(*Duration)
	if !ok {
		return false
	}
	if !d.hasValue {
		return true
	}
	return other.hasValue && d.value == other.value
}

func (d *Duration) IsConcretizable() bool {
	return d.hasValue
}

func (d *Duration) Concretize(ctx ConcreteContext) any {
	if !d.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateDuration(d.value)
}

func (d *Duration) Static() Pattern {
	return &TypePattern{val: d.WidestOfType()}
}

func (d *Duration) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%duration")))
	if d.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(commonfmt.FmtInoxDuration(d.value))))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (d *Duration) WidestOfType() SymbolicValue {
	return ANY_DURATION
}

// A FileMode represents a symbolic FileMode.
type FileMode struct {
	_ int
}

func (m *FileMode) Test(v SymbolicValue) bool {
	_, ok := v.(*FileMode)
	return ok
}

func (m *FileMode) Static() Pattern {
	return &TypePattern{val: m.WidestOfType()}
}

func (m *FileMode) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%filemode")))
}

func (m *FileMode) WidestOfType() SymbolicValue {
	return ANY_FILEMODE
}

// A FileInfo represents a symbolic FileInfo.
type FileInfo struct {
	UnassignablePropsMixin
	SerializableMixin
}

func (f *FileInfo) Test(v SymbolicValue) bool {
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

func (f *FileInfo) Prop(name string) SymbolicValue {
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
		return ANY_DATE
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

func (f *FileInfo) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%file-info")))
}

func (f *FileInfo) WidestOfType() SymbolicValue {
	return ANY_FILEINFO
}

// A Type represents a symbolic Type.
type Type struct {
	Type reflect.Type //if nil, any type is matched
}

func (t *Type) Test(v SymbolicValue) bool {
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

func (t *Type) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if t.Type == nil {
		utils.Must(w.Write(utils.StringAsBytes("%t")))
		return
	}
	utils.Must(fmt.Fprintf(w, "%%type(%v)", t.Type))
}

func (t *Type) WidestOfType() SymbolicValue {
	return &Type{}
}

type IProps interface {
	SymbolicValue
	Prop(name string) SymbolicValue

	// SetProp should be equivalent to .SetProp of a concrete IProps, the difference being that the original IProps should
	// not be modified since all symbolic values are immutable, an IProps with the modification should be returned.
	SetProp(name string, value SymbolicValue) (IProps, error)

	// WithExistingPropReplaced should return a version of the Iprops with the replacement value of the given property.
	WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error)

	// returned slice should never be modified
	PropertyNames() []string
}

type UnassignablePropsMixin struct {
}

func (UnassignablePropsMixin) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New("unassignable properties")
}

func (UnassignablePropsMixin) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, ErrUnassignablePropsMixin
}

type OptionalIProps interface {
	IProps
	OptionalPropertyNames() []string
}

func GetAllPropertyNames(v IProps) []string {
	names := utils.CopySlice(v.PropertyNames())
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
	SymbolicValue
	Prop(name string) SymbolicValue
	PropertyNames() []string
	GetGoMethod(name string) (*GoFunction, bool)
}

func GetGoMethodOrPanic(name string, v GoValue) SymbolicValue {
	method, ok := v.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, v))
	}
	return method
}

type Bytecode struct {
	Bytecode any //if nil, any function is matched
}

func (b *Bytecode) Test(v SymbolicValue) bool {
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

func (b *Bytecode) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if b.Bytecode == nil {
		utils.Must(w.Write(utils.StringAsBytes("%bytecode")))
		return
	}
	utils.Must(fmt.Fprintf(w, "%%bytecode(%v)", b.Bytecode))
}

func (b *Bytecode) WidestOfType() SymbolicValue {
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

func (r *QuantityRange) Test(v SymbolicValue) bool {
	other, ok := v.(*QuantityRange)
	return ok && r.element.Test(other.element)
}

func (r *QuantityRange) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (r *QuantityRange) IteratorElementValue() SymbolicValue {
	return r.element
}

func (r QuantityRange) Contains(value SymbolicValue) (yes bool, possible bool) {
	if !r.element.Test(value) {
		return false, false
	}

	return false, true
}

func (r *QuantityRange) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%quantity-range")))
}

func (r *QuantityRange) WidestOfType() SymbolicValue {
	return &QuantityRange{}
}

// An IntRange represents a symbolic IntRange.
type IntRange struct {
	_ int
	SerializableMixin
}

func (r *IntRange) Test(v SymbolicValue) bool {
	_, ok := v.(*IntRange)
	return ok
}

func (r *IntRange) Static() Pattern {
	return &TypePattern{val: r.WidestOfType()}
}

func (r *IntRange) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%int-range")))
}

func (r *IntRange) KnownLen() int {
	return -1
}

func (r *IntRange) element() SymbolicValue {
	return ANY_INT
}

func (*IntRange) elementAt(i int) SymbolicValue {
	return ANY_INT
}

func (r *IntRange) Contains(value SymbolicValue) (bool, bool) {
	if _, ok := value.(*Int); ok {
		return false, true
	}

	return false, false
}

func (r *IntRange) HasKnownLen() bool {
	return false
}

func (r *IntRange) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (r *IntRange) IteratorElementValue() SymbolicValue {
	return ANY_INT
}

func (r *IntRange) WidestOfType() SymbolicValue {
	return ANY_INT_RANGE
}

// An FloatRange represents a symbolic FloatRange.
type FloatRange struct {
	_ int
	SerializableMixin
}

func (r *FloatRange) Test(v SymbolicValue) bool {
	_, ok := v.(*FloatRange)
	return ok
}

func (r *FloatRange) Static() Pattern {
	return &TypePattern{val: r.WidestOfType()}
}

func (r *FloatRange) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%float-range")))
}

func (r *FloatRange) Contains(value SymbolicValue) (bool, bool) {
	if _, ok := value.(*Float); ok {
		return false, true
	}

	return false, false
}

func (r *FloatRange) WidestOfType() SymbolicValue {
	return ANY_FLOAT_RANGE
}

// A RuneRange represents a symbolic RuneRange.
type RuneRange struct {
	_ int
	SerializableMixin
}

func (r *RuneRange) Test(v SymbolicValue) bool {
	_, ok := v.(*RuneRange)
	return ok
}

func (r *RuneRange) Static() Pattern {
	return &TypePattern{val: r.WidestOfType()}
}

func (r *RuneRange) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%rune-range")))
}

func (r *RuneRange) KnownLen() int {
	return -1
}

func (r *RuneRange) element() SymbolicValue {
	return &Rune{}
}

func (r *RuneRange) Contains(value SymbolicValue) (bool, bool) {
	if _, ok := value.(*Rune); ok {
		return false, true
	}

	return false, false
}

func (r *RuneRange) HasKnownLen() bool {
	return false
}

func (r *RuneRange) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (r *RuneRange) IteratorElementValue() SymbolicValue {
	return ANY_RUNE
}

func (r *RuneRange) WidestOfType() SymbolicValue {
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

func (c *ByteCount) Test(v SymbolicValue) bool {
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

func (c *ByteCount) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%byte-count")))
	if c.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(utils.Must(commonfmt.FmtByteCount(c.value, -1)))))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (c *ByteCount) WidestOfType() SymbolicValue {
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

func (c *ByteRate) Test(v SymbolicValue) bool {
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

func (r *ByteRate) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%byte-rate")))
	if r.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(utils.Must(commonfmt.FmtByteCount(r.value, -1)))))
		utils.Must(w.Write(utils.StringAsBytes("/s")))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (r *ByteRate) WidestOfType() SymbolicValue {
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

func (c *LineCount) Test(v SymbolicValue) bool {
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

func (c *LineCount) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%line-count")))
	if c.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(c.value, 10))))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (c *LineCount) WidestOfType() SymbolicValue {
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

func (c *RuneCount) Test(v SymbolicValue) bool {
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

func (c *RuneCount) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%rune-count")))
	if c.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(c.value, 10))))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (c *RuneCount) WidestOfType() SymbolicValue {
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

func (c *SimpleRate) Test(v SymbolicValue) bool {
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

func (r *SimpleRate) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%simple-rate")))
	if r.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(r.value, 10))))
		utils.Must(w.Write(utils.StringAsBytes("x/s")))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (r *SimpleRate) WidestOfType() SymbolicValue {
	return ANY_SIMPLERATE
}

type ResourceName interface {
	WrappedString
	ResourceName() *String
}

type AnyResourceName struct {
	_ int
}

func (r *AnyResourceName) Test(v SymbolicValue) bool {
	switch v.(type) {
	case ResourceName:
		return true
	default:
		return false
	}
}

func (r *AnyResourceName) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%resource-name")))
}

func (r *AnyResourceName) underlyingString() *String {
	return &String{}
}

func (r *AnyResourceName) ResourceName() *String {
	return &String{}
}

func (r *AnyResourceName) WidestOfType() SymbolicValue {
	return ANY_RES_NAME
}

//
//

// A Port represents a symbolic Port.
type Port struct {
	_ int
}

func (p *Port) Test(v SymbolicValue) bool {
	_, ok := v.(*Port)
	return ok
}

func (p *Port) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *Port) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%port")))
}

func (p *Port) WidestOfType() SymbolicValue {
	return ANY_PORT
}

// A Udata represents a symbolic UData.
type UData struct {
	_ int
}

func (i *UData) Test(v SymbolicValue) bool {
	_, ok := v.(*UData)
	return ok
}

func (*UData) WalkerElement() SymbolicValue {
	return ANY
}

func (*UData) WalkerNodeMeta() SymbolicValue {
	return Nil
}

func (i *UData) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%udata")))
}

func (i *UData) WidestOfType() SymbolicValue {
	return &UData{}
}

// A UDataHiearchyEntry represents a symbolic UData.
type UDataHiearchyEntry struct {
	_ int
}

func (i *UDataHiearchyEntry) Test(v SymbolicValue) bool {
	_, ok := v.(*UDataHiearchyEntry)
	return ok
}

func (i *UDataHiearchyEntry) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%udata-hiearchy-entry")))
}

func (i *UDataHiearchyEntry) WidestOfType() SymbolicValue {
	return &UDataHiearchyEntry{}
}

func IsSimpleSymbolicInoxVal(v SymbolicValue) bool {
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
