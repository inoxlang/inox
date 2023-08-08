package symbolic

import (
	"errors"
	"time"
)

var (
	_ = []PotentiallyConcretizable{
		(*Bool)(nil), (*Int)(nil), (*Float)(nil), Nil,

		(*ByteCount)(nil), (*LineCount)(nil), (*ByteRate)(nil), (*SimpleRate)(nil), (*Duration)(nil), (*Date)(nil),

		(*Rune)(nil), (*String)(nil), (*Path)(nil), (*URL)(nil), (*Host)(nil), (*Scheme)(nil),
		(*Identifier)(nil),
		(*PropertyName)(nil),

		(*StringConcatenation)(nil),

		(*RuneSlice)(nil), (*ByteSlice)(nil),

		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil), (*KeyList)(nil), (*Dictionary)(nil),

		(*ObjectPattern)(nil), (*RecordPattern)(nil), (*ListPattern)(nil), (*TuplePattern)(nil),
		(*TypePattern)(nil),

		(*InoxFunction)(nil),

		(*FileInfo)(nil),

		(*Option)(nil),
	}

	ErrNotConcretizable = errors.New("not concretizable")
)

type ConcreteValueFactories struct {
	CreateNil  func() any
	CreateBool func(bool) any

	CreateFloat func(float64) any
	CreateInt   func(int64) any

	CreateByteCount          func(int64) any
	CreateLineCount          func(int64) any
	CreateRuneCount          func(int64) any
	CreateSimpleRate         func(int64) any
	CreateByteRate           func(int64) any
	CreateDuration           func(time.Duration) any
	CreateDate               func(time.Time) any
	CreateByte               func(uint8) any
	CreateRune               func(rune) any
	CreateString             func(string) any
	CreateStringConctenation func(elements []any) any

	CreatePath   func(string) any
	CreateURL    func(string) any
	CreateHost   func(string) any
	CreateScheme func(string) any

	CreateIdentifier   func(string) any
	CreatePropertyName func(string) any

	CreateByteSlice func([]byte) any
	CreateRuneSlice func([]rune) any

	CreateObject     func(concreteProperties map[string]any) any
	CreateRecord     func(concreteProperties map[string]any) any
	CreateList       func(elements []any) any
	CreateTuple      func(elements []any) any
	CreateKeyList    func(names []string) any
	CreateDictionary func(keys, values []any) Any

	CreatePathPattern func(string) any
	CreateURLPattern  func(string) any
	CreateHostPattern func(string) any

	CreateObjectPattern func(inexact bool, concretePropertyPatterns map[string]any, optionalProperties map[string]struct{}) any
	CreateRecordPattern func(inexact bool, concretePropertyPatterns map[string]any, optionalProperties map[string]struct{}) any
	CreateListPattern   func(generalElementPattern any, elementPatterns []any) any
	CreateTuplePattern  func(generalElementPattern any, elementPatterns []any) any

	//CreateFileInfo func() any

	CreateOption func(name string, value any) any
}

type PotentiallyConcretizable interface {
	IsConcretizable() bool
	Concretize() any
}

func IsConcretizable(v SymbolicValue) bool {
	potentiallyConcretizable, ok := v.(PotentiallyConcretizable)
	return ok && potentiallyConcretizable.IsConcretizable()
}

func Concretize(v SymbolicValue) (any, error) {
	potentiallyConcretizable, ok := v.(PotentiallyConcretizable)
	if !ok || !potentiallyConcretizable.IsConcretizable() {
		return nil, ErrNotConcretizable
	}
	return potentiallyConcretizable.Concretize(), nil
}
