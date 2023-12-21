package symbolic

import (
	"github.com/google/uuid"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/oklog/ulid/v2"
)

var (
	ANY_ULID   = &ULID{}
	ANY_UUIDv4 = &UUIDv4{}

	_ = []Value{(*ULID)(nil), (*UUIDv4)(nil)}
)

// An ULID represents a symbolic ULID.
type ULID struct {
	SerializableMixin
	value    ulid.ULID
	hasValue bool
}

func NewULID(v ulid.ULID) *ULID {
	return &ULID{
		value:    v,
		hasValue: true,
	}
}

func (i *ULID) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherULID, ok := v.(*ULID)
	if !ok {
		return false
	}
	if !i.hasValue {
		return true
	}
	return otherULID.hasValue && i.value == otherULID.value
}

func (i *ULID) IsConcretizable() bool {
	return i.hasValue
}

func (i *ULID) Concretize(ctx ConcreteContext) any {
	if !i.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateULID(i.value)
}

func (i *ULID) HasValue() bool {
	return i.IsConcretizable()
}

func (i *ULID) Value() ulid.ULID {
	if !i.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return i.value
}

func (i *ULID) Static() Pattern {
	return &TypePattern{val: ANY_ULID}
}

func (i *ULID) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("ulid")
	if i.hasValue {
		w.WriteByte('(')
		w.WriteString(i.value.String())
		w.WriteByte(')')
	}
}

func (i *ULID) WidestOfType() Value {
	return ANY_ULID
}

// An UUIDv4 represents a symbolic UUIDv4.
type UUIDv4 struct {
	SerializableMixin
	value    uuid.UUID
	hasValue bool
}

func NewUUID(v uuid.UUID) *UUIDv4 {
	return &UUIDv4{
		value:    v,
		hasValue: true,
	}
}

func (i *UUIDv4) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherUUID, ok := v.(*UUIDv4)
	if !ok {
		return false
	}
	if !i.hasValue {
		return true
	}
	return otherUUID.hasValue && i.value == otherUUID.value
}

func (i *UUIDv4) IsConcretizable() bool {
	return i.hasValue
}

func (i *UUIDv4) Concretize(ctx ConcreteContext) any {
	if !i.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateUUID(i.value)
}

func (i *UUIDv4) HasValue() bool {
	return i.IsConcretizable()
}

func (i *UUIDv4) Value() uuid.UUID {
	if !i.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return i.value
}

func (i *UUIDv4) Static() Pattern {
	return &TypePattern{val: ANY_UUIDv4}
}

func (i *UUIDv4) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("uuidv4")
	if i.hasValue {
		w.WriteByte('(')
		w.WriteString(i.value.String())
		w.WriteByte(')')
	}
}

func (i *UUIDv4) WidestOfType() Value {
	return ANY_UUIDv4
}
