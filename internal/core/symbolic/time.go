package symbolic

import (
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

// A Year represents a symbolic Year.
type Year struct {
	SerializableMixin
	value    time.Time
	hasValue bool
}

func NewYear(v time.Time) *Year {
	return &Year{
		value:    v,
		hasValue: true,
	}
}

func (d *Year) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*Year)
	if !ok {
		return false
	}
	if !d.hasValue {
		return true
	}
	return other.hasValue && d.value == other.value
}

func (d *Year) IsConcretizable() bool {
	return d.hasValue
}

func (d *Year) Concretize(ctx ConcreteContext) any {
	if !d.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateYear(d.value)
}

func (d *Year) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if d.hasValue {
		w.WriteString(commonfmt.FmtInoxYear(d.value))
	} else {
		w.WriteName("year")
	}
}

func (d *Year) WidestOfType() Value {
	return ANY_YEAR
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

func (d *Date) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

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

func (d *Date) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("date")
	if d.hasValue {
		w.WriteByte('(')
		w.WriteString(commonfmt.FmtInoxDate(d.value))
		w.WriteByte(')')
	}
}

func (d *Date) WidestOfType() Value {
	return ANY_DATE
}

// A DateTime represents a symbolic DateTime.
type DateTime struct {
	SerializableMixin
	value    time.Time
	hasValue bool
}

func NewDateTime(v time.Time) *DateTime {
	return &DateTime{
		value:    v,
		hasValue: true,
	}
}

func (d *DateTime) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*DateTime)
	if !ok {
		return false
	}
	if !d.hasValue {
		return true
	}
	return other.hasValue && d.value == other.value
}

func (d *DateTime) IsConcretizable() bool {
	return d.hasValue
}

func (d *DateTime) Concretize(ctx ConcreteContext) any {
	if !d.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateDateTime(d.value)
}

func (d *DateTime) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("datetime")
	if d.hasValue {
		w.WriteByte('(')
		w.WriteString(commonfmt.FmtInoxDateTime(d.value))
		w.WriteByte(')')
	}
}

func (d *DateTime) WidestOfType() Value {
	return ANY_DATETIME
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

func (d *Duration) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

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

func (d *Duration) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("duration")
	if d.hasValue {
		w.WriteByte('(')
		w.WriteString(commonfmt.FmtInoxDuration(d.value))
		w.WriteByte(')')
	}
}

func (d *Duration) WidestOfType() Value {
	return ANY_DURATION
}
