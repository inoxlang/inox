package internal

import "errors"

// An DynamicValue represents a symbolic DynamicValue.
type DynamicValue struct {
	val SymbolicValue
}

func NewAnyDynamicValue() *DynamicValue {
	return &DynamicValue{val: ANY}
}

func NewDynamicValue(val SymbolicValue) *DynamicValue {
	return &DynamicValue{val: val}
}

func (d *DynamicValue) Test(v SymbolicValue) bool {
	return d.val.Test(v)
}

func (d *DynamicValue) Prop(memberName string) SymbolicValue {
	return &DynamicValue{d.val.(IProps).Prop(memberName)}
}

func (d *DynamicValue) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (d *DynamicValue) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (d *DynamicValue) PropertyNames() []string {
	return d.val.(IProps).PropertyNames()
}

func (d *DynamicValue) Widen() (SymbolicValue, bool) {
	if d.IsWidenable() {
		return nil, false
	}
	widened, _ := d.val.Widen()
	return &DynamicValue{val: widened}, true
}

func (d *DynamicValue) IsWidenable() bool {
	return d.val.IsWidenable()
}

func (d *DynamicValue) String() string {
	return "dyn"
}

func (d *DynamicValue) WidestOfType() SymbolicValue {
	return NewAnyDynamicValue()
}

func (d *DynamicValue) IteratorElementKey() SymbolicValue {
	return ANY
}

func (d *DynamicValue) IteratorElementValue() SymbolicValue {
	return ANY
}

func (d *DynamicValue) WatcherElement() SymbolicValue {
	return ANY
}

func (d *DynamicValue) TakeInMemorySnapshot() (*Snapshot, error) {
	if v, ok := d.val.(InMemorySnapshotable); ok {
		return v.TakeInMemorySnapshot()
	}
	return nil, ErrFailedToSnapshot
}
