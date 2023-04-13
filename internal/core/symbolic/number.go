package internal

var (
	_            = []Integral{&Int{}, &Byte{}, &AnyIntegral{}}
	ANY_INTEGRAL = &AnyIntegral{}
	ANY_INT      = &Int{}
)

type Integral interface {
	Int64() (i *Int, signed bool)
}

// A Float represents a symbolic Float.
type Float struct {
	_ int
}

func (f *Float) Test(v SymbolicValue) bool {
	_, ok := v.(*Float)
	return ok
}

func (f *Float) IsWidenable() bool {
	return false
}

func (f *Float) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (f *Float) String() string {
	return "float"
}

func (f *Float) WidestOfType() SymbolicValue {
	return &Float{}
}

// An Int represents a symbolic Int.
type Int struct {
	_ int
}

func (i *Int) Test(v SymbolicValue) bool {
	_, ok := v.(*Int)
	return ok
}

func (a *Int) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (i *Int) IsWidenable() bool {
	return false
}

func (i *Int) String() string {
	return "%int"
}

func (i *Int) WidestOfType() SymbolicValue {
	return &Int{}
}

func (i *Int) Int64() (n *Int, signed bool) {
	return i, true
}

// An AnyIntegral represents a symbolic Integral we do not know the concrete type.
type AnyIntegral struct {
	_ int
}

func (*AnyIntegral) Test(v SymbolicValue) bool {
	_, ok := v.(Integral)

	return ok
}

func (*AnyIntegral) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (*AnyIntegral) IsWidenable() bool {
	return false
}

func (*AnyIntegral) String() string {
	return "integral"
}

func (*AnyIntegral) WidestOfType() SymbolicValue {
	return ANY_INTEGRAL
}

func (*AnyIntegral) IteratorElementKey() SymbolicValue {
	return ANY
}

func (*AnyIntegral) IteratorElementValue() SymbolicValue {
	return ANY
}

func (*AnyIntegral) Int64() (i *Int, signed bool) {
	return &Int{}, true
}
