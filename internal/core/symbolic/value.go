package internal

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrWideSymbolicValue      = errors.New("cannot create wide symbolic value")
	ErrNoSymbolicValue        = errors.New("no symbolic value")
	ErrUnassignablePropsMixin = errors.New("UnassignablePropsMixin")

	ANY           = &Any{}
	ANY_BOOL      = &Bool{}
	ANY_RES_NAME  = &AnyResourceName{}
	ANY_OPTION    = &Option{}
	ANY_INT_RANGE = &IntRange{}
)

func isAny(val SymbolicValue) bool {
	_, ok := val.(*Any)
	return ok
}

// A SymbolicValue represents a Value during symbolic evaluation, its underyling data should be immutable.
type SymbolicValue interface {
	Test(v SymbolicValue) bool

	IsWidenable() bool
	Widen() (SymbolicValue, bool)
	String() string

	IsMutable() bool

	WidestOfType() SymbolicValue
}

type PseudoPropsValue interface {
	SymbolicValue
	PropertyNames() []string
	Prop(name string) SymbolicValue
}

type StaticDataHolder interface {
	SymbolicValue
	AddStatic(Pattern)
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

func (a *Any) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Any) IsWidenable() bool {
	return false
}

func (a *Any) String() string {
	return "%any"
}

func (a *Any) WidestOfType() SymbolicValue {
	return ANY
}

//

var Nil = &NilT{}

// A NilT represents a symbolic NilT.
type NilT struct {
	_ int
}

func (b *NilT) Test(v SymbolicValue) bool {
	_, ok := v.(*NilT)
	return ok
}

func (a *NilT) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *NilT) IsWidenable() bool {
	return false
}

func (a *NilT) String() string {
	return "nil"
}

func (a *NilT) WidestOfType() SymbolicValue {
	return &NilT{}
}

// A Bool represents a symbolic Bool.
type Bool struct {
	_ int
}

func (b *Bool) Test(v SymbolicValue) bool {
	_, ok := v.(*Bool)
	return ok
}

func (a *Bool) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Bool) IsWidenable() bool {
	return false
}

func (a *Bool) String() string {
	return "%boolean"
}

func (a *Bool) WidestOfType() SymbolicValue {
	return ANY_BOOL
}

// A EmailAddress represents a symbolic EmailAddress.
type EmailAddress struct {
	UnassignablePropsMixin
	_ int
}

func (s *EmailAddress) Test(v SymbolicValue) bool {
	_, ok := v.(*EmailAddress)
	return ok
}

func (s *EmailAddress) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *EmailAddress) IsWidenable() bool {
	return false
}

func (s *EmailAddress) String() string {
	return "%email-address"
}

func (s *EmailAddress) PropertyNames() []string {
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

func (s *EmailAddress) WidestOfType() SymbolicValue {
	return &Host{}
}

// A Identifier represents a symbolic Identifier.
type Identifier struct {
	name string
}

func (i *Identifier) Test(v SymbolicValue) bool {
	other, ok := v.(*Identifier)
	if !ok {
		return false
	}
	return i.name == "" || i.name == other.name
}

func (i *Identifier) Name() string {
	return i.name
}

func (i *Identifier) Widen() (SymbolicValue, bool) {
	if i.name == "" {
		return nil, false
	}
	return &Identifier{}, true
}

func (i *Identifier) IsWidenable() bool {
	return i.name != ""
}

func (i *Identifier) String() string {
	if i.name == "" {
		return "%identifier"
	}
	return fmt.Sprintf("#%s", i.name)
}

func (s *Identifier) underylingString() *String {
	return &String{}
}

func (s *Identifier) WidestOfType() SymbolicValue {
	return &Identifier{}
}

// A PropertyName represents a symbolic PropertyName.
type PropertyName struct {
	name string
}

func (p *PropertyName) Test(v SymbolicValue) bool {
	other, ok := v.(*PropertyName)
	if !ok {
		return false
	}
	return p.name == "" || p.name == other.name
}

func (p *PropertyName) Widen() (SymbolicValue, bool) {
	if p.name == "" {
		return nil, false
	}
	return &PropertyName{}, true
}

func (p *PropertyName) IsWidenable() bool {
	return p.name != ""
}

func (p *PropertyName) String() string {
	if p.name == "" {
		return "%property-name"
	}
	return fmt.Sprintf("%%property-name(#%s)", p.name)
}

func (s *PropertyName) underylingString() *String {
	return &String{}
}

func (s *PropertyName) WidestOfType() SymbolicValue {
	return &Identifier{}
}

// A Mimetype represents a symbolic Mimetype.
type Mimetype struct {
	_ int
}

func (p *Mimetype) Test(v SymbolicValue) bool {
	_, ok := v.(*Mimetype)
	return ok
}

func (a *Mimetype) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Mimetype) IsWidenable() bool {
	return false
}

func (p *Mimetype) String() string {
	return "%mimetype"
}

func (s *Mimetype) WidestOfType() SymbolicValue {
	return &Mimetype{}
}

// An Option represents a symbolic Option.
type Option struct {
	name string //if "", any name is matched
	_    int
}

func NewOption(name string) *Option {
	return &Option{name: name}
}

func (o *Option) Name() (string, bool) {
	if o.name == "" {
		return "%", false
	}
	return o.name, true
}

func (o *Option) Test(v SymbolicValue) bool {
	otherOpt, ok := v.(*Option)
	if !ok {
		return false
	}
	return o.name == "" || o.name == otherOpt.name
}

func (opt *Option) IsWidenable() bool {
	return opt.name != ""
}

func (o *Option) Widen() (SymbolicValue, bool) {
	if o.IsWidenable() {
		return ANY_OPTION, true
	}
	return nil, false
}

func (o *Option) String() string {
	if !o.IsWidenable() {
		return "%option"
	}
	return "%--" + o.name + "(...)"
}

func (o *Option) WidestOfType() SymbolicValue {
	return ANY_OPTION
}

// A Date represents a symbolic Date.
type Date struct {
	_ int
}

func (f *Date) Test(v SymbolicValue) bool {
	_, ok := v.(*Date)
	return ok
}

func (a *Date) IsWidenable() bool {
	return false
}

func (a *Date) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Date) String() string {
	return "%date"
}

func (a *Date) WidestOfType() SymbolicValue {
	return &Date{}
}

//

type Duration struct {
	_ int
}

func (f *Duration) Test(v SymbolicValue) bool {
	_, ok := v.(*Duration)
	return ok
}

func (a *Duration) IsWidenable() bool {
	return false
}

func (a *Duration) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Duration) String() string {
	return "%duration"
}

func (a *Duration) WidestOfType() SymbolicValue {
	return &Duration{}
}

//

type FileMode struct {
	_ int
}

func (f *FileMode) Test(v SymbolicValue) bool {
	_, ok := v.(*FileMode)
	return ok
}

func (a *FileMode) IsWidenable() bool {
	return false
}

func (a *FileMode) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *FileMode) String() string {
	return "%filemode"
}

func (a *FileMode) WidestOfType() SymbolicValue {
	return &FileMode{}
}

//

type FileInfo struct {
	_ int
}

func (f *FileInfo) Test(v SymbolicValue) bool {
	_, ok := v.(*FileInfo)
	return ok
}

func (a *FileInfo) IsWidenable() bool {
	return false
}

func (a *FileInfo) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *FileInfo) String() string {
	return "%file-info"
}

func (a *FileInfo) WidestOfType() SymbolicValue {
	return &FileInfo{}
}

// A Type represents a symbolic Type.
type Type struct {
	Type reflect.Type //if nil, any type is matched
}

func (b *Type) Test(v SymbolicValue) bool {
	other, ok := v.(*Type)
	if !ok {
		return false
	}
	if b.Type == nil {
		return true
	}

	if other.Type == nil {
		return false
	}

	return utils.SamePointer(b.Type, other.Type)
}

func (b *Type) Widen() (SymbolicValue, bool) {
	if b.Type == nil {
		return nil, false
	}
	return &Type{}, true
}

func (b *Type) IsWidenable() bool {
	return b.Type != nil
}

func (b *Type) String() string {
	if b.Type == nil {
		return "%t"
	}
	return fmt.Sprintf("%%type(%v)", b.Type)
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

// An Function represents a symbolic function we do not know the concrete type.
type Function struct {
	pattern *FunctionPattern
}

func (r *Function) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *Function, *GoFunction, *InoxFunction:
		return r.pattern.TestValue(v)
	default:
		return false
	}
}

func (r *Function) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *Function) IsWidenable() bool {
	return false
}

func (r *Function) String() string {
	return "%function"
}

func (r *Function) WidestOfType() SymbolicValue {
	return &Function{
		pattern: (&FunctionPattern{}).WidestOfType().(*FunctionPattern),
	}
}

//

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

func (b *Bytecode) Widen() (SymbolicValue, bool) {
	if b.Bytecode == nil {
		return nil, false
	}
	return &Bytecode{}, true
}

func (b *Bytecode) IsWidenable() bool {
	return b.Bytecode != nil
}

func (b *Bytecode) String() string {
	if b.Bytecode == nil {
		return "%bytecode"
	}
	return fmt.Sprintf("%%bytecode(%v)", b.Bytecode)
}

func (b *Bytecode) WidestOfType() SymbolicValue {
	return &Bytecode{}
}

// A QuantityRange represents a symbolic QuantityRange.
type QuantityRange struct {
	_ int
}

func (r *QuantityRange) Test(v SymbolicValue) bool {
	_, ok := v.(*QuantityRange)
	return ok
}

func (r *QuantityRange) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *QuantityRange) IsWidenable() bool {
	return false
}

func (r *QuantityRange) String() string {
	return "%quantity-range"
}

func (r *QuantityRange) WidestOfType() SymbolicValue {
	return &QuantityRange{}
}

//

type IntRange struct {
	_ int
}

func (r *IntRange) Test(v SymbolicValue) bool {
	_, ok := v.(*IntRange)
	return ok
}

func (s *IntRange) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *IntRange) IsWidenable() bool {
	return false
}

func (r *IntRange) String() string {
	return "%int-range"
}

func (r *IntRange) knownLen() int {
	return -1
}

func (r *IntRange) element() SymbolicValue {
	return &Int{}
}

func (*IntRange) elementAt(i int) SymbolicValue {
	return &Int{}
}

func (r *IntRange) HasKnownLen() bool {
	return false
}

func (r *IntRange) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (r *IntRange) IteratorElementValue() SymbolicValue {
	return &Int{}
}

func (r *IntRange) WidestOfType() SymbolicValue {
	return &IntRange{}
}

//

type RuneRange struct {
	_ int
}

func (r *RuneRange) Test(v SymbolicValue) bool {
	_, ok := v.(*RuneRange)
	return ok
}

func (s *RuneRange) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *RuneRange) IsWidenable() bool {
	return false
}

func (r *RuneRange) String() string {
	return "%rune-range"
}

func (r *RuneRange) knownLen() int {
	return -1
}

func (r *RuneRange) element() SymbolicValue {
	return &Rune{}
}

func (r *RuneRange) HasKnownLen() bool {
	return false
}

func (r *RuneRange) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (r *RuneRange) IteratorElementValue() SymbolicValue {
	return &Rune{}
}

func (r *RuneRange) WidestOfType() SymbolicValue {
	return &RuneRange{}
}

//

type ByteCount struct {
	_ int
}

func (r *ByteCount) Test(v SymbolicValue) bool {
	_, ok := v.(*ByteCount)

	return ok
}

func (r *ByteCount) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *ByteCount) IsWidenable() bool {
	return false
}

func (r *ByteCount) String() string {
	return "%byte-count"
}

func (r *ByteCount) WidestOfType() SymbolicValue {
	return &ByteCount{}
}

//

//

type ByteRate struct {
	_ int
}

func (r *ByteRate) Test(v SymbolicValue) bool {
	_, ok := v.(*ByteRate)

	return ok
}

func (r *ByteRate) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *ByteRate) IsWidenable() bool {
	return false
}

func (r *ByteRate) String() string {
	return "%byte-rate"
}

func (r *ByteRate) WidestOfType() SymbolicValue {
	return &ByteRate{}
}

// A LineCount represents a symbolic LineCount.
type LineCount struct {
	_ int
}

func (r *LineCount) Test(v SymbolicValue) bool {
	_, ok := v.(*LineCount)

	return ok
}

func (r *LineCount) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *LineCount) IsWidenable() bool {
	return false
}

func (r *LineCount) String() string {
	return "%line-count"
}

func (r *LineCount) WidestOfType() SymbolicValue {
	return &LineCount{}
}

// A RuneCount represents a symbolic RuneCount.
type RuneCount struct {
	_ int
}

func (r *RuneCount) Test(v SymbolicValue) bool {
	_, ok := v.(*RuneCount)

	return ok
}

func (r *RuneCount) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *RuneCount) IsWidenable() bool {
	return false
}

func (r *RuneCount) String() string {
	return "%rune-count"
}

func (r *RuneCount) WidestOfType() SymbolicValue {
	return &RuneCount{}
}

//

type SimpleRate struct {
	_ int
}

func (r *SimpleRate) Test(v SymbolicValue) bool {
	_, ok := v.(*SimpleRate)

	return ok
}

func (r *SimpleRate) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *SimpleRate) IsWidenable() bool {
	return false
}

func (r *SimpleRate) String() string {
	return "%simple-rate"
}

func (r *SimpleRate) WidestOfType() SymbolicValue {
	return &SimpleRate{}
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

func (r *AnyResourceName) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *AnyResourceName) IsWidenable() bool {
	return false
}

func (r *AnyResourceName) String() string {
	return "%resource-name"
}

func (r *AnyResourceName) underylingString() *String {
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

func (i *Port) Test(v SymbolicValue) bool {
	_, ok := v.(*Port)
	return ok
}

func (a *Port) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Port) IsWidenable() bool {
	return false
}

func (i *Port) String() string {
	return "%port"
}

func (i *Port) WidestOfType() SymbolicValue {
	return &Port{}
}

// A Udata represents a symbolic UData.
type UData struct {
	_ int
}

func (i *UData) Test(v SymbolicValue) bool {
	_, ok := v.(*UData)
	return ok
}

func (a *UData) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *UData) IsWidenable() bool {
	return false
}

func (i *UData) String() string {
	return "%udata"
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

func (a *UDataHiearchyEntry) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *UDataHiearchyEntry) IsWidenable() bool {
	return false
}

func (i *UDataHiearchyEntry) String() string {
	return "%udata-hiearchy-entry"
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
