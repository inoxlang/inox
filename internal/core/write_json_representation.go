package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	JSON_UNTYPED_VALUE_SUFFIX   = "__value"
	MAX_JSON_REPR_WRITING_DEPTH = 20
	JS_MIN_SAFE_INTEGER         = -9007199254740991
	JS_MAX_SAFE_INTEGER         = 9007199254740991

	//int range serialization

	SERIALIZED_INT_RANGE_START_KEY     = "start"
	SERIALIZED_INT_RANGE_START_END_KEY = "end"

	//float range serialization

	SERIALIZED_FLOAT_RANGE_START_KEY          = "start"
	SERIALIZED_FLOAT_RANGE_START_EXCL_END_KEY = "exclusiveEnd"
	SERIALIZED_FLOAT_RANGE_START_END_KEY      = "end"

	//int range pattern serialization

	SERIALIZED_INT_RANGE_PATTERN_RANGE_KEY = "range"
	SERIALIZED_INT_RANGE_PATTERN_MULT_OF   = "multipleOf"

	//float range pattern serialization

	SERIALIZED_FLOAT_RANGE_PATTERN_RANGE_KEY = "range"
	SERIALIZED_FLOAT_RANGE_PATTERN_MULT_OF   = "multipleOf"

	//int range string pattern serialization

	SERIALIZED_INT_RANGE_STRING_PATTERN_RANGE_KEY = "range"

	//float range string pattern serialization

	SERIALIZED_FLOAT_RANGE_STRING_PATTERN_RANGE_KEY = "range"

	//secret pattern serialization

	SERIALIZED_SECRET_PATTERN_VAL_PATTERN_KEY     = "valuePattern"
	SERIALIZED_SECRET_PATTERN_VAL_PEM_ENCODED_KEY = "isPemEncoded"

	//event pattern serialization

	SERIALIZED_EVENT_PATTERN_VAL_PATTERN_KEY = "value"

	//object pattern serialization

	SERIALIZED_OBJECT_PATTERN_INEXACT_KEY           = "inexact"
	SERIALIZED_OBJECT_PATTERN_ENTRIES_KEY           = "entries"
	SERIALIZED_OBJECT_PATTERN_ENTRY_PATTERN_KEY     = "pattern"
	SERIALIZED_OBJECT_PATTERN_ENTRY_IS_OPTIONAL_KEY = "isOptional"
	SERIALIZED_OBJECT_PATTERN_ENTRY_REQ_KEYS_KEY    = "requiredKeys"
	SERIALIZED_OBJECT_PATTERN_ENTRY_REQ_PATTERN_KEY = "requiredPattern"

	//record pattern serialization

	SERIALIZED_RECORD_PATTERN_INEXACT_KEY           = "inexact"
	SERIALIZED_RECORD_PATTERN_ENTRIES_KEY           = "entries"
	SERIALIZED_RECORD_PATTERN_ENTRY_PATTERN_KEY     = "pattern"
	SERIALIZED_RECORD_PATTERN_ENTRY_IS_OPTIONAL_KEY = "isOptional"

	//list pattern serialization

	SERIALIZED_LIST_PATTERN_ELEMENTS_KEY  = "elements"
	SERIALIZED_LIST_PATTERN_ELEMENT_KEY   = "element"
	SERIALIZED_LIST_PATTERN_MIN_COUNT_KEY = "minCount"
	SERIALIZED_LIST_PATTERN_MAX_COUNT_KEY = "maxCount"

	//tuple pattern serialization

	SERIALIZED_TUPLE_PATTERN_ELEMENTS_KEY = "elements"
	SERIALIZED_TUPLE_PATTERN_ELEMENT_KEY  = "element"
)

var (
	ErrMaximumJSONReprWritingDepthReached  = errors.New("maximum JSON representation writing depth reached")
	ErrPatternDoesNotMatchValueToSerialize = errors.New("pattern does not match value to serialize")
	ErrPatternRequiredToSerialize          = errors.New("pattern required to serialize")

	ALL_VISIBLE_REPR_CONFIG = &ReprConfig{AllVisible: true}
)

// this file contains the implementation of Value.WriteJSONRepresentation for core types.

//TODO: for all types, add more checks before not using JSON_UNTYPED_VALUE_SUFFIX.

type ReprConfig struct {
	AllVisible bool
}

func (r *ReprConfig) IsValueVisible(v Value) bool {
	if _, ok := v.(*Secret); ok {
		return false
	}

	if r == nil || r.AllVisible {
		return true
	}
	if IsAtomSensitive(v) {
		return false
	}
	return true
}

func (r *ReprConfig) IsPropertyVisible(name string, v Value, info *ValueVisibility, ctx *Context) bool {
	if _, ok := v.(*Secret); ok {
		return false
	}

	if r == nil || r.AllVisible || (info != nil && utils.SliceContains(info.publicKeys, name)) {
		return true
	}

	if IsSensitiveProperty(ctx, name, v) || !r.IsValueVisible(v) {
		return false
	}
	return true
}

var ErrNoRepresentation = errors.New("no representation")

type JSONSerializationConfig struct {
	*ReprConfig
	Pattern  Pattern //nillable
	Location string  //location of the current value being serialized
}

func GetJSONRepresentation(v Serializable, ctx *Context, pattern Pattern) string {
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

	err := v.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{Pattern: pattern}, 0)
	if err != nil {
		panic(fmt.Errorf("%s: %w", Stringify(v, ctx), err))
	}
	return string(stream.Buffer())
}

func MustGetJSONRepresentationWithConfig(v Serializable, ctx *Context, config JSONSerializationConfig) string {
	repr, err := GetJSONRepresentationWithConfig(v, ctx, config)
	if err != nil {
		panic(fmt.Errorf("%s: %w", Stringify(v, ctx), err))
	}
	return repr
}

func GetJSONRepresentationWithConfig(v Serializable, ctx *Context, config JSONSerializationConfig) (string, error) {
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

	err := v.WriteJSONRepresentation(ctx, stream, config, 0)
	if err != nil {
		return "", err
	}
	return string(stream.Buffer()), nil
}

func WriteUntypedValueJSON(typeName string, fn func(w *jsoniter.Stream) error, w *jsoniter.Stream) error {
	w.WriteObjectStart()
	w.WriteObjectField(typeName + JSON_UNTYPED_VALUE_SUFFIX)
	if err := fn(w); err != nil {
		return err
	}
	w.WriteObjectEnd()
	return nil
}

func fmtErrPatternDoesNotMatchValueToSerialize(ctx *Context, config JSONSerializationConfig) error {
	return fmt.Errorf("%w at /%s, pattern: %s", ErrPatternDoesNotMatchValueToSerialize, config.Location, Stringify(config.Pattern, ctx))
}

func (Nil NilT) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	w.WriteNil()
	return nil
}

func (b Bool) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	w.WriteBool(bool(b))
	return nil
}

func (r Rune) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(RUNE_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(r))
			return nil
		}, w)
	}

	w.WriteString(string(r))
	return nil
}

func (b Byte) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNoRepresentation
}

func writeIntJsonRepr(n Int, w *jsoniter.Stream) {
	if n < JS_MIN_SAFE_INTEGER || n > JS_MAX_SAFE_INTEGER {
		fmt.Fprintf(w, `"%d"`, n)
	} else {
		fmt.Fprintf(w, `%d`, n)
	}
}

func (i Int) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(INT_PATTERN.Name, func(w *jsoniter.Stream) error {
			writeIntJsonRepr(i, w)
			return nil
		}, w)
		return nil
	}

	writeIntJsonRepr(i, w)
	return nil
}

func (f Float) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	w.WriteFloat64(float64(f))
	return w.Error
}

func (s String) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	w.WriteString(string(s))
	return nil
}

func (obj *Object) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		objectPattern, writeWithObjectPattern := config.Pattern.(*ObjectPattern)

		w.WriteObjectStart()
		var err error

		closestState := ctx.MustGetClosestState()
		obj._lock(closestState)
		defer obj._unlock(closestState)
		keys := obj.keys
		var visibility *ValueVisibility
		if obj.hasAdditionalFields() {
			visibility, _ = GetVisibility(obj.visibilityId)
		}

		first := true

		//meta properties

		if obj.hasAdditionalFields() && obj.url != "" {
			first = false

			_, err = w.Write(utils.StringAsBytes(`"_url_":`))
			if err != nil {
				return err
			}

			w.WriteString(obj.url.UnderlyingString())
		}

		for i, k := range keys {
			v := obj.values[i]

			if !config.IsPropertyVisible(k, v, visibility, ctx) {
				continue
			}

			if !first {
				w.WriteMore()
			}

			first = false
			w.WriteObjectField(k)

			config := JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}

			if writeWithObjectPattern {
				pattern, _, ok := objectPattern.Entry(k)
				if ok {
					config.Pattern = pattern
				}
			}

			err = v.WriteJSONRepresentation(ctx, w, config, depth+1)
			if err != nil {
				return err
			}
		}

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(OBJECT_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (rec *Record) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		recordPattern, writeWithRecordPattern := config.Pattern.(*RecordPattern)

		w.WriteObjectStart()

		keys := rec.keys
		first := true

		for i, k := range keys {
			v := rec.values[i]

			if !config.IsPropertyVisible(k, v, nil, ctx) {
				continue
			}

			if !first {
				w.WriteMore()
			}

			first = false
			w.WriteObjectField(k)

			config := JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}

			if writeWithRecordPattern {
				entry, ok := recordPattern.CompleteEntry(k)
				if ok {
					config.Pattern = entry.Pattern
				}
			}

			err := v.WriteJSONRepresentation(ctx, w, config, depth+1)
			if err != nil {
				return err
			}
		}

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(RECORD_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}

	return write(w)
}

func (list KeyList) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNoRepresentation

}

func (list *List) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return list.underlyingList.WriteJSONRepresentation(ctx, w, config, depth)
}

func (list *ValueList) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	//TODO: bypass check if done at pattern level
	for _, v := range list.elements {
		if !config.IsValueVisible(v) {
			return ErrNoRepresentation
		}
	}

	listPattern, _ := config.Pattern.(*ListPattern)

	write := func(w *jsoniter.Stream) error {
		w.WriteArrayStart()

		first := true
		for i, v := range list.elements {

			if !first {
				w.WriteMore()
			}
			first = false

			elementConfig := JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}

			if listPattern != nil {
				if listPattern.generalElementPattern != nil {
					elementConfig.Pattern = listPattern.generalElementPattern
				} else if listPattern != nil {
					elementConfig.Pattern = listPattern.elementPatterns[i]
				}
			}

			err := v.WriteJSONRepresentation(ctx, w, elementConfig, depth+1)
			if err != nil {
				return err
			}
		}

		w.WriteArrayEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(LIST_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (list *NumberList[T]) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	listPattern, _ := config.Pattern.(*ListPattern)

	write := func(w *jsoniter.Stream) error {
		w.WriteArrayStart()
		first := true
		for i, v := range list.elements {
			if !first {
				w.WriteMore()
			}
			first = false

			elementConfig := JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}

			if listPattern != nil {
				if listPattern.generalElementPattern != nil {
					elementConfig.Pattern = listPattern.generalElementPattern
				} else if listPattern != nil {
					elementConfig.Pattern = listPattern.elementPatterns[i]
				}
			}

			err := v.WriteJSONRepresentation(ctx, w, elementConfig, depth+1)
			if err != nil {
				return err
			}
		}

		w.WriteArrayEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(LIST_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (list *BoolList) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func(w *jsoniter.Stream) error {
		w.WriteArrayStart()
		first := true
		length := list.elements.Len()

		for i := uint(0); i < length; i++ {
			if !first {
				w.WriteMore()
			}
			first = false

			if list.elements.Test(i) {
				w.WriteTrue()
			} else {
				w.WriteFalse()
			}
		}
		w.WriteArrayEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(LIST_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (list *StringList) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func(w *jsoniter.Stream) error {
		w.WriteArrayStart()
		first := true
		for _, v := range list.elements {
			if !first {
				w.WriteMore()
			}
			first = false
			err := v.WriteJSONRepresentation(ctx, w, JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}, depth+1)
			if err != nil {
				return err
			}
		}

		w.WriteArrayEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(LIST_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (tuple *Tuple) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	//TODO: bypass check if done at pattern level
	for _, v := range tuple.elements {
		if !config.IsValueVisible(v) {
			return ErrNoRepresentation
		}
	}

	write := func(w *jsoniter.Stream) error {
		tuplePattern, _ := config.Pattern.(*TuplePattern)

		w.WriteArrayStart()

		first := true
		for i, v := range tuple.elements {

			if !first {
				w.WriteMore()
			}
			first = false

			elementConfig := JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}

			if tuplePattern != nil {
				if tuplePattern.generalElementPattern != nil {
					elementConfig.Pattern = tuplePattern.generalElementPattern
				} else {
					elementConfig.Pattern = tuplePattern.elementPatterns[i]
				}
			}

			err := v.WriteJSONRepresentation(ctx, w, elementConfig, depth+1)
			if err != nil {
				return err
			}
		}

		w.WriteArrayEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(TUPLE_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (p *OrderedPair) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplemented
}

func (d *Dictionary) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (u *Treedata) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (u *TreedataHiearchyEntry) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (slice *RuneSlice) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (slice *ByteSlice) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

func (opt Option) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		b := make([]byte, 0, len(opt.Name)+4)

		if len(opt.Name) <= 1 {
			b = append(b, '-')
		} else {
			b = append(b, '-', '-')
		}

		b = append(b, opt.Name...)

		w.WriteString(string(b))

		// _, err = w.Write([]byte{'='})
		// if err != nil {
		// 	return err
		// }
		// if err := opt.Value.WriteJSONRepresentation(ctx, w,); err != nil {
		// 	return err
		// }
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(OPTION_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}

	return nil
}

func (pth Path) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		WriteUntypedValueJSON(PATH_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(pth))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(pth))
	return nil
}

func (patt PathPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		WriteUntypedValueJSON(PATHPATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(patt))
			return nil
		}, w)
		return nil
	}

	w.WriteString(string(patt))
	return nil
}

func (u URL) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(URL_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(u))
			return nil
		}, w)
	}
	w.WriteString(string(u))
	return nil
}

func (host Host) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		WriteUntypedValueJSON(HOST_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(host))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(host))
	return nil
}

func (scheme Scheme) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(SCHEME_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(scheme))
			return nil
		}, w)
	}
	w.WriteString(string(scheme))
	return nil
}

func (patt HostPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		WriteUntypedValueJSON(HOSTPATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(patt))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(patt))
	return nil
}

func (addr EmailAddress) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(EMAIL_ADDR_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(addr))
			return nil
		}, w)
	}
	w.WriteString(string(addr))
	return nil
}

func (patt URLPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(URLPATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(patt))
			return nil
		}, w)
	}
	w.WriteString(string(patt))
	return nil
}
func (i Identifier) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(IDENT_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(i))
			return nil
		}, w)
	}
	w.WriteString(string(i))
	return nil
}

func (p PropertyName) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(PROPNAME_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(p))
			return nil
		}, w)
	}
	w.WriteString(string(p))
	return nil
}

func (p *LongValuePath) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func() error {
		w.WriteArrayStart()
		for i, segment := range *p {
			if i != 0 {
				w.WriteMore()
			}
			err := segment.WriteJSONRepresentation(ctx, w, config, depth+1)
			if err != nil {
				return err
			}
		}
		w.WriteArrayEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(LONG_VALUEPATH_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write()
		}, w)
	}
	return write()
}

func (str CheckedString) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNoRepresentation
}

func (count ByteCount) Write(w io.Writer, _3digitGroupCount int) (int, error) {
	s, err := commonfmt.FmtByteCount(int64(count), _3digitGroupCount)
	if err != nil {
		return 0, err
	}
	return w.Write(utils.StringAsBytes(s))
}

func (count ByteCount) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer
	if _, err := count.Write(&buff, -1); err != nil {
		return err
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(BYTECOUNT_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
	}
	w.WriteString(buff.String())
	return nil
}

func (count LineCount) write(w io.Writer) (int, error) {
	return fmt.Fprintf(w, "%dln", count)
}

func (count LineCount) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer
	if count < 0 {
		return ErrNoRepresentation
	}
	if _, err := count.write(&buff); err != nil {
		return err
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(LINECOUNT_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
	}
	w.WriteString(buff.String())
	return nil
}

func (count RuneCount) write(w io.Writer) (int, error) {
	return fmt.Fprintf(w, "%drn", count)
}

func (count RuneCount) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer
	if count < 0 {
		return ErrNoRepresentation
	}
	if _, err := count.write(&buff); err != nil {
		return err
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(RUNECOUNT_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
	}
	w.WriteString(buff.String())
	return nil
}

func (rate ByteRate) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if rate < 0 {
		return ErrNoRepresentation
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(BYTERATE_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteInt64(int64(rate))
			return nil
		}, w)
	}
	w.WriteInt64(int64(rate))
	return nil
}

func (f Frequency) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if f < 0 {
		return ErrNoRepresentation
	}
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(FREQUENCY_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteFloat64(float64(f))
			return nil
		}, w)
	}
	w.WriteFloat64(float64(f))
	return nil
}

func (d Duration) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if d < 0 {
		return ErrNoRepresentation
	}

	float := float64(d) / float64(time.Second)

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(DURATION_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteFloat64(float)
			return nil
		}, w)
	}
	w.WriteFloat64(float)
	return nil
}

func (y Year) write(w io.Writer) (int, error) {
	return w.Write(utils.StringAsBytes(commonfmt.FmtInoxYear(time.Time(y))))
}

func (y Year) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer

	if _, err := y.write(&buff); err != nil {
		return err
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(YEAR_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
	}
	w.WriteString(buff.String())
	return nil
}

func (d Date) write(w io.Writer) (int, error) {
	return w.Write(utils.StringAsBytes(commonfmt.FmtInoxDate(time.Time(d))))
}

func (d Date) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer

	if _, err := d.write(&buff); err != nil {
		return err
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(DATE_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
	}
	w.WriteString(buff.String())
	return nil
}

func (d DateTime) write(w io.Writer) (int, error) {
	return w.Write(utils.StringAsBytes(commonfmt.FmtInoxDateTime(time.Time(d))))
}

func (d DateTime) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer

	if _, err := d.write(&buff); err != nil {
		return err
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(DATETIME_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
	}
	w.WriteString(buff.String())
	return nil
}

func (m FileMode) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNoRepresentation
}

func (r RuneRange) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()

		w.WriteObjectField("start")
		w.WriteString(string(r.Start))
		w.WriteMore()

		w.WriteObjectField("end")
		w.WriteString(string(r.End))

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(RUNE_RANGE_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}

	return write(w)
}

func (r QuantityRange) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

func (r IntRange) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()

		if !r.unknownStart {
			w.WriteObjectField(SERIALIZED_INT_RANGE_START_KEY)
			writeIntJsonRepr(Int(r.start), w)
			w.WriteMore()
		}

		w.WriteObjectField(SERIALIZED_INT_RANGE_START_END_KEY)
		writeIntJsonRepr(Int(r.end), w)

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(INT_RANGE_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}

	return write(w)
}

func (r FloatRange) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()

		if !r.unknownStart {
			w.WriteObjectField(SERIALIZED_FLOAT_RANGE_START_KEY)
			w.WriteFloat64(r.start)
			w.WriteMore()
		}

		if !r.inclusiveEnd {
			w.WriteObjectField(SERIALIZED_FLOAT_RANGE_START_EXCL_END_KEY)
		} else {
			w.WriteObjectField(SERIALIZED_FLOAT_RANGE_START_END_KEY)
		}
		w.WriteFloat64(r.end)

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(FLOAT_RANGE_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}

	return write(w)
}

//patterns

func (p IntRangePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()

		rangeConfig := JSONSerializationConfig{ReprConfig: config.ReprConfig, Pattern: INT_RANGE_PATTERN}

		w.WriteObjectField(SERIALIZED_INT_RANGE_PATTERN_RANGE_KEY)
		err := p.intRange.WriteJSONRepresentation(ctx, w, rangeConfig, depth+1)
		if err != nil {
			return err
		}

		if p.multipleOfFloat != nil {
			w.WriteMore()
			w.WriteObjectField(SERIALIZED_INT_RANGE_PATTERN_MULT_OF)
			w.WriteFloat64(float64(*p.multipleOfFloat))
		} else if p.multipleOf > 0 {
			w.WriteMore()
			w.WriteObjectField(SERIALIZED_INT_RANGE_PATTERN_MULT_OF)
			w.WriteInt64(int64(p.multipleOf))
		}

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(INT_RANGE_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}

	return write(w)
}

func (p FloatRangePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()

		rangeConfig := JSONSerializationConfig{ReprConfig: config.ReprConfig, Pattern: FLOAT_RANGE_PATTERN}

		w.WriteObjectField(SERIALIZED_FLOAT_RANGE_PATTERN_RANGE_KEY)
		err := p.floatRange.WriteJSONRepresentation(ctx, w, rangeConfig, depth+1)
		if err != nil {
			return err
		}

		if p.multipleOf > 0 {
			w.WriteMore()
			w.WriteObjectField(SERIALIZED_FLOAT_RANGE_PATTERN_MULT_OF)
			w.WriteFloat64(float64(p.multipleOf))
		}

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(FLOAT_RANGE_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}

	return write(w)
}

func (pattern ExactValuePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		valueConfig := JSONSerializationConfig{ReprConfig: config.ReprConfig}
		return pattern.value.WriteJSONRepresentation(ctx, w, valueConfig, depth+1)
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(EXACT_VALUE_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (pattern ExactStringPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		valueConfig := JSONSerializationConfig{ReprConfig: config.ReprConfig}
		return pattern.value.WriteJSONRepresentation(ctx, w, valueConfig, depth+1)
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(EXACT_STRING_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (pattern TypePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	if pattern.Name == "" {
		return fmt.Errorf("type pattern has no name")
	}
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(TYPE_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(pattern.Name)
			return nil
		}, w)
	}
	w.WriteString(pattern.Name)
	return nil
}

func (pattern *DifferencePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}
func (pattern *OptionalPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt RegexPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(REGEX_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(patt.Regex())
			return nil
		}, w)
	}
	w.WriteString(patt.Regex())
	return nil
}

func (patt UnionPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt IntersectionPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt LengthCheckingStringPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt ParserBasedPseudoPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt DateFormat) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt SequenceStringPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt UnionStringPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt RuneRangeStringPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt DynamicStringPatternElement) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt *RepeatedPatternElement) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt *NamedSegmentPathPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(NAMED_SEGMENT_PATH_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(patt.node.Raw)
			return nil
		}, w)
	}
	w.WriteString(patt.node.Raw)
	return nil
}

func (patt ObjectPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	if len(patt.complexPropertyPatterns) > 0 {
		return fmt.Errorf("serialization of object pattern with complex constraints is not supported yet")
	}

	write := func() error {
		w.WriteObjectStart()

		w.WriteObjectField(SERIALIZED_OBJECT_PATTERN_INEXACT_KEY)
		w.WriteBool(patt.inexact)

		//entries
		w.WriteMore()
		w.WriteObjectField(SERIALIZED_OBJECT_PATTERN_ENTRIES_KEY)
		w.WriteObjectStart()
		for i, entry := range patt.entries {
			if i != 0 {
				w.WriteMore()
			}
			// write <entry name>: {
			w.WriteObjectField(entry.Name)
			w.WriteObjectStart()

			//write "pattern": <pattern>
			w.WriteObjectField(SERIALIZED_OBJECT_PATTERN_ENTRY_PATTERN_KEY)

			config := JSONSerializationConfig{ReprConfig: config.ReprConfig}

			err := entry.Pattern.WriteJSONRepresentation(ctx, w, config, depth+1)
			if err != nil {
				return fmt.Errorf("failed to serialize pattern of entry %q: %w", entry.Name, err)
			}

			//if optional write "isOptional": true
			if entry.IsOptional {
				w.WriteMore()
				w.WriteObjectField(SERIALIZED_OBJECT_PATTERN_ENTRY_IS_OPTIONAL_KEY)
				w.WriteTrue()
			}

			//write required keys in an array.
			if len(entry.Dependencies.RequiredKeys) > 0 {
				w.WriteMore()
				w.WriteObjectField(SERIALIZED_OBJECT_PATTERN_ENTRY_REQ_KEYS_KEY)
				w.WriteArrayStart()

				for keyIndex, key := range entry.Dependencies.RequiredKeys {
					if keyIndex != 0 {
						w.WriteMore()
					}
					w.WriteString(key)
				}
				w.WriteArrayEnd()
			}

			//write required pattern.
			if entry.Dependencies.Pattern != nil {
				w.WriteMore()
				w.WriteObjectField(SERIALIZED_OBJECT_PATTERN_ENTRY_REQ_PATTERN_KEY)

				config := JSONSerializationConfig{ReprConfig: config.ReprConfig}

				err := entry.Dependencies.Pattern.WriteJSONRepresentation(ctx, w, config, depth+1)
				if err != nil {
					return fmt.Errorf("failed to serialize required pattern: %w", err)
				}
			}

			w.WriteObjectEnd() //'}' after <entry name>: { ...
		}
		w.WriteObjectEnd() //'}' after "entries": { ...
		w.WriteObjectEnd() //final '}'
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(OBJECT_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write()
		}, w)
	}
	return write()
}

func (patt RecordPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func() error {
		w.WriteObjectStart()

		w.WriteObjectField(SERIALIZED_RECORD_PATTERN_INEXACT_KEY)
		w.WriteBool(patt.inexact)

		//entries
		w.WriteMore()
		w.WriteObjectField(SERIALIZED_RECORD_PATTERN_ENTRIES_KEY)
		w.WriteObjectStart()
		for i, entry := range patt.entries {
			if i != 0 {
				w.WriteMore()
			}
			// write <entry name>: {
			w.WriteObjectField(entry.Name)
			w.WriteObjectStart()

			//write "pattern": <pattern>
			w.WriteObjectField(SERIALIZED_RECORD_PATTERN_ENTRY_PATTERN_KEY)

			config := JSONSerializationConfig{ReprConfig: config.ReprConfig}

			err := entry.Pattern.WriteJSONRepresentation(ctx, w, config, depth+1)
			if err != nil {
				return fmt.Errorf("failed to serialize pattern of entry %q: %w", entry.Name, err)
			}

			//if optional write "isOptional": true
			if entry.IsOptional {
				w.WriteMore()
				w.WriteObjectField(SERIALIZED_RECORD_PATTERN_ENTRY_IS_OPTIONAL_KEY)
				w.WriteTrue()
			}

			w.WriteObjectEnd() //'}' after <entry name>: { ...
		}
		w.WriteObjectEnd() //'}' after "entries": { ...
		w.WriteObjectEnd() //final '}'
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(RECORD_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write()
		}, w)
	}
	return write()
}

func (patt ListPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func() error {
		w.WriteObjectStart()

		if patt.generalElementPattern == nil { //known length
			w.WriteObjectField(SERIALIZED_LIST_PATTERN_ELEMENTS_KEY)
			w.WriteArrayStart()

			elementConfig := JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}

			for elemIndex, elem := range patt.elementPatterns {
				if elemIndex != 0 {
					w.WriteMore()
				}
				err := elem.WriteJSONRepresentation(ctx, w, elementConfig, depth+1)
				if err != nil {
					return fmt.Errorf("failed to serialize pattern of the list pattern's element at index %d: %w", elemIndex, err)
				}
			}
			w.WriteArrayEnd()
		} else { //general element
			w.WriteObjectField(SERIALIZED_LIST_PATTERN_ELEMENT_KEY)
			elementConfig := JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}
			err := patt.generalElementPattern.WriteJSONRepresentation(ctx, w, elementConfig, depth+1)
			if err != nil {
				return fmt.Errorf("failed to serialize pattern of the list pattern's general element: %w", err)
			}

			if patt.containedElement != nil {
				w.WriteMore()
				elementConfig := JSONSerializationConfig{
					ReprConfig: config.ReprConfig,
				}
				err := patt.generalElementPattern.WriteJSONRepresentation(ctx, w, elementConfig, depth+1)
				if err != nil {
					return fmt.Errorf("failed to serialize pattern of the list pattern's contained element: %w", err)
				}
			}

			//write minimum element count.
			w.WriteMore()
			w.WriteObjectField(SERIALIZED_LIST_PATTERN_MIN_COUNT_KEY)
			w.WriteInt(patt.MinElementCount())

			max := patt.MaxElementCount()
			if max != DEFAULT_LIST_PATTERN_MAX_ELEM_COUNT {
				//write maximum element count.
				w.WriteMore()
				w.WriteObjectField(SERIALIZED_LIST_PATTERN_MAX_COUNT_KEY)
				w.WriteInt(max)
			}
		}

		w.WriteObjectEnd() //final '}'
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(LIST_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write()
		}, w)
	}
	return write()
}

func (patt TuplePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func() error {
		w.WriteObjectStart()

		if patt.generalElementPattern == nil { //known length
			w.WriteObjectField(SERIALIZED_TUPLE_PATTERN_ELEMENTS_KEY)
			w.WriteArrayStart()

			elementConfig := JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}

			for elemIndex, elem := range patt.elementPatterns {
				if elemIndex != 0 {
					w.WriteMore()
				}
				err := elem.WriteJSONRepresentation(ctx, w, elementConfig, depth+1)
				if err != nil {
					return fmt.Errorf("failed to serialize pattern of the tuple pattern's element at index %d: %w", elemIndex, err)
				}
			}
			w.WriteArrayEnd()
		} else { //general element
			w.WriteObjectField(SERIALIZED_TUPLE_PATTERN_ELEMENT_KEY)
			elementConfig := JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
			}
			err := patt.generalElementPattern.WriteJSONRepresentation(ctx, w, elementConfig, depth+1)
			if err != nil {
				return fmt.Errorf("failed to serialize pattern of the tuple pattern's general element: %w", err)
			}
		}

		w.WriteObjectEnd() //final '}'
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(TUPLE_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write()
		}, w)
	}
	return write()
}

func (patt OptionPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt *PathStringPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt *FunctionPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt *IntRangeStringPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()
		w.WriteObjectField(SERIALIZED_INT_RANGE_STRING_PATTERN_RANGE_KEY)

		rangeConfig := JSONSerializationConfig{ReprConfig: config.ReprConfig, Pattern: INT_RANGE_PATTERN}
		err := patt.intRange.WriteJSONRepresentation(ctx, w, rangeConfig, depth+1)
		if err != nil {
			return err
		}

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(INT_RANGE_STRING_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (patt *FloatRangeStringPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()
		w.WriteObjectField(SERIALIZED_FLOAT_RANGE_STRING_PATTERN_RANGE_KEY)

		rangeConfig := JSONSerializationConfig{ReprConfig: config.ReprConfig, Pattern: FLOAT_RANGE_PATTERN}
		err := patt.floatRange.WriteJSONRepresentation(ctx, w, rangeConfig, depth+1)
		if err != nil {
			return err
		}

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(FLOAT_RANGE_STRING_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (patt *SecretPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()
		w.WriteObjectField(SERIALIZED_SECRET_PATTERN_VAL_PATTERN_KEY)

		valPatternConfig := JSONSerializationConfig{ReprConfig: config.ReprConfig}
		err := patt.stringPattern.WriteJSONRepresentation(ctx, w, valPatternConfig, depth+1)
		if err != nil {
			return err
		}

		w.WriteMore()
		w.WriteObjectField(SERIALIZED_SECRET_PATTERN_VAL_PEM_ENCODED_KEY)
		w.WriteBool(patt.pemEncoded)

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(SECRET_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (patt *EventPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()
		w.WriteObjectField(SERIALIZED_EVENT_PATTERN_VAL_PATTERN_KEY)

		valPatternConfig := JSONSerializationConfig{ReprConfig: config.ReprConfig}
		err := patt.valuePattern.WriteJSONRepresentation(ctx, w, valPatternConfig, depth+1)
		if err != nil {
			return err
		}

		w.WriteObjectEnd()
		return nil
	}

	if NoPatternOrAny(config.Pattern) {
		return WriteUntypedValueJSON(EVENT_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (patt *MutationPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

// end of pattern serialization methods.

func (mt Mimetype) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNoRepresentation
}

func (i FileInfo) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNoRepresentation
}

func (b *Bytecode) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNoRepresentation
}

func (port Port) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

func (c *StringConcatenation) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return String(c.GetOrBuildString()).WriteJSONRepresentation(ctx, w, config, depth+1)
}

func (c *BytesConcatenation) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

func (c Color) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

func (g *SystemGraph) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (e SystemGraphEvent) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (e SystemGraphEdge) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (v *DynamicValue) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (v Error) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (j *LifetimeJob) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (h *SynchronousMessageHandler) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (f *InoxFunction) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (m *Mapping) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (n AstNode) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (p *ModuleParamsPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (s *FilesystemSnapshotIL) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	//TODO: only serialize if size is at most a dozen kilobytes.

	return ErrNotImplementedYet
}

func (id ULID) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	w.WriteString(id.libValue().String())
	return nil
}

func (id UUIDv4) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	w.WriteString(id.libValue().String())
	return nil
}

func NoPatternOrAny(p Pattern) bool {
	return p == nil || p == ANYVAL_PATTERN || p == SERIALIZABLE_PATTERN
}
