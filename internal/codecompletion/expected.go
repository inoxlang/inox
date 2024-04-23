package codecompletion

import (
	"bytes"
	"strconv"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/jsoniter"
)

type expectedValueStringificationParams struct {
	expectedValue symbolic.Value
	search        completionSearch
	valueAtCursor symbolic.Value //may be nil
}

func stringifyExpectedValue(params expectedValueStringificationParams) (string, bool) {
	indentationUnit := params.search.chunk.EstimatedIndentationUnit()

	buf := make([]byte, 0, 10)

	ok := _stringifiedExpectedValue(&buf, indentationUnit, params)
	if ok {
		return string(buf), true
	}

	return "", false
}

func _stringifiedExpectedValue(buf *[]byte, indentationUnit string, params expectedValueStringificationParams) bool {
	expectedValue := params.expectedValue
	search := params.search

	switch v := expectedValue.(type) {
	case *symbolic.Object:
		appendByte(buf, '{')

		currentObj, _ := params.valueAtCursor.(*symbolic.Object)

		v.ForEachEntry(func(propName string, propValue symbolic.Value) error {
			if v.IsExistingPropertyOptional(propName) || (currentObj != nil && currentObj.HasPropertyOptionalOrNot(propName)) {
				return nil
			}

			appendByte(buf, '\n')

			appendString(buf, indentationUnit)
			appendPropName(buf, propName)
			appendString(buf, ": ")
			_stringifiedExpectedValue(buf, indentationUnit, expectedValueStringificationParams{
				expectedValue: propValue,
				search:        search,
			})
			return nil
		})

		appendString(buf, "\n}")

		return true
	case *symbolic.Record:
		appendString(buf, "#{")

		currentRecord, _ := params.valueAtCursor.(*symbolic.Object)

		v.ForEachEntry(func(propName string, propValue symbolic.Value) error {
			if v.IsExistingPropertyOptional(propName) || (currentRecord != nil && currentRecord.HasPropertyOptionalOrNot(propName)) {
				return nil
			}

			appendByte(buf, '\n')

			appendString(buf, indentationUnit)
			appendPropName(buf, propName)
			appendString(buf, ": ")

			_stringifiedExpectedValue(buf, indentationUnit, expectedValueStringificationParams{
				expectedValue: propValue,
				search:        search,
			})
			return nil
		})

		appendString(buf, "\n}")

		return true
	case *symbolic.Dictionary:

		if !v.AllKeysConcretizable() {
			return false
		}

		appendString(buf, ":{")

		v.ForEachEntry(func(_ symbolic.Serializable, keyString string, value symbolic.Value) error {
			appendByte(buf, '\n')

			appendString(buf, indentationUnit)
			appendString(buf, keyString)

			appendString(buf, ": ")
			_stringifiedExpectedValue(buf, indentationUnit, expectedValueStringificationParams{
				expectedValue: value,
				search:        search,
			})
			return nil
		})

		appendString(buf, "\n}")
	case symbolic.StringLike:
		symbString := v.GetOrBuildString()
		if symbString.HasValue() {
			jsoniter.AppendString(buf, symbString.Value())
			return true
		}
	case *symbolic.Bool:
		if v.IsConcretizable() {
			if v.MustGetValue() {
				appendString(buf, "true")
			} else {
				appendString(buf, "false")
			}
			return true
		}
	case *symbolic.Int:
		if v.HasValue() {
			*buf = strconv.AppendInt(*buf, v.Value(), 10)
			return true
		}
	case *symbolic.Float:
		if v.IsConcretizable() {
			prevLen := len(*buf)
			*buf = strconv.AppendFloat(*buf, v.MustGetValue(), 'f', -1, 64)
			if !bytes.ContainsAny((*buf)[prevLen:], ".e") {
				appendString(buf, ".0")
			}
			return true
		}
	case *symbolic.Path:
		if v.IsConcretizable() {
			appendString(buf, symbolic.Stringify(v))
		}
	case *symbolic.PathPattern:
		if v.IsConcretizable() {
			appendByte(buf, '%')
			appendString(buf, symbolic.Stringify(v))
		}
	}
	return false
}
