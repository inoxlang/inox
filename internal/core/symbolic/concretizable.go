package symbolic

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

var (
	_ = []PotentiallyConcretizable{
		(*Bool)(nil), (*Int)(nil), (*Float)(nil), Nil,

		(*ByteCount)(nil), (*LineCount)(nil), (*ByteRate)(nil), (*Frequency)(nil), (*Duration)(nil), (*DateTime)(nil),

		(*Rune)(nil), (*String)(nil), (*Path)(nil), (*URL)(nil), (*Host)(nil), (*Scheme)(nil),
		(*Identifier)(nil),
		(*PropertyName)(nil),

		(*StringConcatenation)(nil),

		(*RuneSlice)(nil), (*ByteSlice)(nil),

		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil), (*KeyList)(nil), (*Dictionary)(nil),

		(*ObjectPattern)(nil), (*RecordPattern)(nil), (*ListPattern)(nil), (*TuplePattern)(nil),
		(*TypePattern)(nil),

		(*ExactValuePattern)(nil), (*ExactStringPattern)(nil), (*URLPattern)(nil), (*PathPattern)(nil),

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

	CreateByteCount func(int64) any
	CreateLineCount func(int64) any
	CreateRuneCount func(int64) any
	CreateFrequency func(float64) any
	CreateByteRate  func(int64) any

	CreateDuration func(time.Duration) any
	CreateYear     func(time.Time) any
	CreateDate     func(time.Time) any
	CreateDateTime func(time.Time) any

	CreateByte                func(byte) any
	CreateRune                func(rune) any
	CreateString              func(string) any
	CreateStringConcatenation func(elements []any) any

	CreatePath   func(string) any
	CreateURL    func(string) any
	CreateHost   func(string) any
	CreateScheme func(string) any

	CreateIdentifier   func(string) any
	CreatePropertyName func(string) any

	CreateByteSlice func([]byte) any
	CreateRuneSlice func([]rune) any

	CreateObject      func(concreteProperties map[string]any) any
	CreateRecord      func(concreteProperties map[string]any) any
	CreateList        func(elements []any) any
	CreateTuple       func(elements []any) any
	CreateOrderedPair func(first, second any) any
	CreateKeyList     func(names []string) any
	CreateDictionary  func(keys, values []any, ctx ConcreteContext) any

	CreatePathPattern func(string) any
	CreateURLPattern  func(string) any
	CreateHostPattern func(string) any

	CreateObjectPattern func(inexact bool, concretePropertyPatterns map[string]any, optionalProperties map[string]struct{}) any
	CreateRecordPattern func(inexact bool, concretePropertyPatterns map[string]any, optionalProperties map[string]struct{}) any
	CreateListPattern   func(generalElementPattern any, elementPatterns []any) any
	CreateTuplePattern  func(generalElementPattern any, elementPatterns []any) any

	CreateExactValuePattern  func(value any) any
	CreateExactStringPattern func(value any) any

	//CreateFileInfo func() any

	CreateOption func(name string, value any) any

	CreateULID func(ulid.ULID) any
	CreateUUID func(uuid.UUID) any
}

type PotentiallyConcretizable interface {
	IsConcretizable() bool
	Concretize(ctx ConcreteContext) any
}

func IsConcretizable(v Value) bool {
	potentiallyConcretizable, ok := v.(PotentiallyConcretizable)
	return ok && potentiallyConcretizable.IsConcretizable()
}

func Concretize(v Value, ctx ConcreteContext) (any, error) {
	potentiallyConcretizable, ok := v.(PotentiallyConcretizable)
	if !ok || !potentiallyConcretizable.IsConcretizable() {
		return nil, ErrNotConcretizable
	}
	return potentiallyConcretizable.Concretize(ctx), nil
}
