package core

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	LINE_COUNT_UNIT = "ln"
	RUNE_COUNT_UNIT = "rn"
	BYTE_COUNT_UNIT = "B"
)

// ByteCount implements Value.
type ByteCount int64

// LineCount implements Value.
type LineCount int

// RuneCount implements Value.
type RuneCount int

type Rate interface {
	Value
	QuantityPerSecond() Value
}

// A ByteRate represents a number of bytes per second, it implements Value.
type ByteRate int

func (r ByteRate) QuantityPerSecond() Value {
	return ByteCount(r)
}

// A SimpleRate represents a number of 'X' per second implements Value.
type SimpleRate int

func (r SimpleRate) QuantityPerSecond() Value {
	return Float(r)
}

// QuantityRange implements Value.
type QuantityRange struct {
	unknownStart bool
	inclusiveEnd bool
	start        Serializable
	end          Serializable
}

func (r QuantityRange) KnownStart() Serializable {
	if r.unknownStart {
		panic(errors.New("range has unknown start"))
	}
	return r.start
}

func (r QuantityRange) InclusiveEnd() Serializable {
	if r.inclusiveEnd {
		return r.end
	}
	return nextInt64Float64(reflect.ValueOf(r.end)).Interface().(Serializable)
}

// evalQuantity computes a quantity value (Duration, ByteCount, ...).
func evalQuantity(values []float64, units []string) (Serializable, error) {

	if len(values) != len(units) {
		return nil, ErrInvalidQuantity
	}

	var totalResultF float64
	var totalResult Serializable

	for partIndex := 0; partIndex < len(units); partIndex++ {

		var (
			partValue  = values[partIndex]
			unit       = units[partIndex]
			i          = 0
			multiplier = 1.0

			partResult float64
			isInt      bool
		)

		if partValue < 0 {
			return nil, ErrNegQuantityNotSupported
		}

		switch unit[i] {
		case 'k':
			multiplier = 1_000.0
			i++
		case 'M':
			multiplier = 1_000_000.0
			i++
		case 'G':
			multiplier = 1_000_000_000.0
			i++
		case 'T':
			multiplier = 1_000_000_000_000.0
			i++
		}

		//multiplier not followed by a unit
		if multiplier != 1.0 && len(unit) == 1 {
			return nil, fmt.Errorf("unterminated unit '%s'", unit)
		}

		switch unit[i:] {
		case "x":
			if totalResult != nil {
				return nil, ErrInvalidQuantity
			}
			partResult = partValue * multiplier
			totalResult = Float(partResult)
		case "h", "mn", "s", "ms", "us", "ns":

			if totalResult == nil {
				totalResult = Duration(0)
			}

			switch unit[i:] {
			case "h":
				multiplier *= float64(time.Hour)
			case "mn":
				multiplier *= float64(time.Minute)
			case "s":
				multiplier *= float64(time.Second)
			case "ms":
				multiplier *= float64(time.Millisecond)
			case "us":
				multiplier *= float64(time.Microsecond)
			case "ns":
				multiplier *= float64(time.Nanosecond)
			}

			isInt = true
			partResult = partValue * multiplier
			totalResult = totalResult.(Duration) + Duration(partResult)
		case "%":
			if multiplier != 1.0 {
				return nil, fmt.Errorf("invalid multiplier '%s' for %%", string(unit[0]))
			}
			partResult = partValue / 100
			totalResult = Float(partResult)
		case LINE_COUNT_UNIT:
			isInt = true
			partResult = partValue * multiplier
			totalResult = LineCount(partResult)
		case RUNE_COUNT_UNIT:
			isInt = true
			partResult = partValue * multiplier
			totalResult = RuneCount(partResult)
		case BYTE_COUNT_UNIT:
			isInt = true
			partResult = partValue * multiplier
			totalResult = ByteCount(partResult)
		default:
			return nil, fmt.Errorf("unsupported unit '%s'", unit[i:])
		}

		totalResultF += partResult

		if isInt && totalResultF >= math.MaxInt64 {
			return nil, ErrQuantityLooLarge
		}
	}

	return totalResult, nil
}

func evalRate(q Value, unitName string) (Serializable, error) {
	switch qv := q.(type) {
	case ByteCount:
		if unitName != "s" {
			return nil, errors.New("invalid unit " + unitName)
		}
		return ByteRate(qv), nil
	case Float:
		if unitName != "s" {
			return nil, errors.New("invalid unit " + unitName)
		}
		return SimpleRate(qv), nil
	}

	return nil, fmt.Errorf("invalid quantity type: %T", q)
}

func mustEvalQuantityRange(n *parse.QuantityRangeLiteral) QuantityRange {
	lowerBound := utils.Must(evalQuantity(n.LowerBound.Values, n.LowerBound.Units))

	var upperBound Serializable
	if n.UpperBound != nil {
		upperBound = utils.Must(evalQuantity(n.UpperBound.(*parse.QuantityLiteral).Values, n.UpperBound.(*parse.QuantityLiteral).Units))
	} else {
		upperBound = getQuantityTypeMaxValue(lowerBound)
	}

	return QuantityRange{
		unknownStart: false,
		inclusiveEnd: true,
		start:        lowerBound,
		end:          upperBound,
	}
}

func getQuantityTypeStart(v Serializable) Serializable {
	switch v.(type) {
	case ByteCount:
		return ByteCount(0)
	case RuneCount:
		return RuneCount(0)
	case LineCount:
		return LineCount(0)
	case Duration:
		return Duration(0)
	default:
		panic(ErrUnreachable)
	}
}

func getQuantityTypeMaxValue(v Serializable) Serializable {
	switch v.(type) {
	case ByteCount:
		return ByteCount(math.MaxInt64)
	case RuneCount:
		return RuneCount(math.MaxInt64)
	case LineCount:
		return LineCount(math.MaxInt64)
	case Duration:
		return Duration(math.MaxInt64)
	default:
		panic(ErrUnreachable)
	}
}

func quantityLessThan(a, b reflect.Value) bool {
	if a.Type() != b.Type() {
		panic(ErrUnreachable)
	}

	switch a.Kind() {
	case reflect.Float64:
		return a.Float() < b.Float()
	case reflect.Int64:
		return a.Int() < b.Int()
	default:
		panic(ErrUnreachable)
	}
}

func quantityLessOrEqual(a, b reflect.Value) bool {
	if a.Type() != b.Type() {
		panic(ErrUnreachable)
	}

	switch a.Kind() {
	case reflect.Float64:
		return a.Float() <= b.Float()
	case reflect.Int64:
		return a.Int() <= b.Int()
	default:
		panic(ErrUnreachable)
	}
}

func nextInt64Float64(v reflect.Value) reflect.Value {
	ptr := reflect.New(v.Type())
	ptr.Elem().Set(v)

	if v.Kind() == reflect.Float64 {
		ptr.SetFloat(math.Nextafter(v.Float(), math.MaxFloat64))
	} else {
		ptr.SetInt(v.Int() + 1)
	}
	return ptr
}
