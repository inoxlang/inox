package core

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"

	"github.com/inoxlang/inox/internal/utils"
	jsoniter "github.com/json-iterator/go"
)

const (
	JSON_UNTYPED_VALUE_SUFFIX   = "__value"
	MAX_JSON_REPR_WRITING_DEPTH = 20
)

var (
	ErrMaximumJSONReprWritingDepthReached  = errors.New("maximum JSON representation writing depth reached")
	ErrPatternDoesNotMatchValueToSerialize = errors.New("pattern does not match value to serialize")
	ErrPatternRequiredToSerialize          = errors.New("pattern required to serialize")
)

// this file contains the implementation of Value.WriteJSONRepresentation for core types.

//TODO: for all types, add more checks before not using JSON_UNTYPED_VALUE_SUFFIX.

type JSONSerializationConfig struct {
	*ReprConfig
	Pattern  Pattern //nillable
	Location string  //location of the current value being serialized
}

func GetJSONRepresentation(v Serializable, ctx *Context, pattern Pattern) string {
	stream := jsoniter.NewStream(jsoniter.ConfigCompatibleWithStandardLibrary, nil, 0)

	err := v.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{Pattern: pattern}, 0)
	if err != nil {
		panic(fmt.Errorf("%s: %w", Stringify(v, ctx), err))
	}
	return string(stream.Buffer())
}

func writeUntypedValueJSON(typeName string, fn func(w *jsoniter.Stream) error, w *jsoniter.Stream) error {
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
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(RUNE_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(r))
			return nil
		}, w)
		return nil
	}

	w.WriteString(string(r))
	return nil
}

func (b Byte) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNoRepresentation
}

func (i Int) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(INT_PATTERN.Name, func(w *jsoniter.Stream) error {
			fmt.Fprintf(w, `"%d"`, i)
			return nil
		}, w)
		return nil
	}

	fmt.Fprintf(w, `"%d"`, i)
	return nil
}

func (f Float) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	w.WriteFloat64(float64(f))
	return w.Error
}

func (s Str) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	w.WriteString(string(s))
	return nil
}

func (obj *Object) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		var entryPatterns map[string]Pattern

		objPatt, ok := config.Pattern.(*ObjectPattern)
		if ok {
			entryPatterns = objPatt.entryPatterns
		}

		w.WriteObjectStart()
		var err error

		closestState := ctx.GetClosestState()
		obj.Lock(closestState)
		defer obj.Unlock(closestState)
		keys := obj.keys
		visibility, _ := GetVisibility(obj.visibilityId)

		first := true

		//meta properties

		if obj.url != "" {
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

			err = v.WriteJSONRepresentation(ctx, w, JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
				Pattern:    entryPatterns[k],
			}, depth+1)
			if err != nil {
				return err
			}
		}

		w.WriteObjectEnd()
		return nil
	}

	if noPatternOrAny(config.Pattern) {
		return writeUntypedValueJSON(OBJECT_PATTERN.Name, func(w *jsoniter.Stream) error {
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
		var entryPatterns map[string]Pattern

		recPatt, ok := config.Pattern.(*RecordPattern)
		if ok {
			entryPatterns = recPatt.entryPatterns
		}

		_, err := w.Write([]byte{'{'})
		if err != nil {
			return err
		}

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

			err = v.WriteJSONRepresentation(ctx, w, JSONSerializationConfig{
				ReprConfig: config.ReprConfig,
				Pattern:    entryPatterns[k],
			}, depth+1)
			if err != nil {
				return err
			}
		}

		_, err = w.Write([]byte{'}'})
		if err != nil {
			return err
		}
		return nil
	}

	if noPatternOrAny(config.Pattern) {
		return writeUntypedValueJSON(RECORD_PATTERN.Name, func(w *jsoniter.Stream) error {
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

	if noPatternOrAny(config.Pattern) {
		return writeUntypedValueJSON(LIST_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (list *IntList) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
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

	if noPatternOrAny(config.Pattern) {
		return writeUntypedValueJSON(LIST_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (list *BoolList) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return list.WriteRepresentation(ctx, w, &ReprConfig{
		AllVisible: true,
	}, 0)
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

	if noPatternOrAny(config.Pattern) {
		return writeUntypedValueJSON(LIST_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (tuple *Tuple) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		tuplePattern := config.Pattern.(*TuplePattern)

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

			if tuplePattern.generalElementPattern != nil {
				elementConfig.Pattern = tuplePattern.generalElementPattern
			} else {
				elementConfig.Pattern = tuplePattern.elementPatterns[i]
			}

			err := v.WriteJSONRepresentation(ctx, w, elementConfig, depth+1)
			if err != nil {
				return err
			}
		}

		w.WriteArrayEnd()
		return nil
	}

	if noPatternOrAny(config.Pattern) {
		return writeUntypedValueJSON(TUPLE_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func (d *Dictionary) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (u *UData) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func (u *UDataHiearchyEntry) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
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

	if noPatternOrAny(config.Pattern) {
		return writeUntypedValueJSON(OPTION_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}

	return nil
}

func (pth Path) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(PATH_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(pth))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(pth))
	return nil
}

func (patt PathPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(PATHPATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(patt))
			return nil
		}, w)
		return nil
	}

	w.WriteString(string(patt))
	return nil
}

func (u URL) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(URL_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(u))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(u))
	return nil
}

func (host Host) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(HOST_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(host))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(host))
	return nil
}

func (scheme Scheme) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(SCHEME_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(scheme))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(scheme))
	return nil
}

func (patt HostPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(HOSTPATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(patt))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(patt))
	return nil
}

func (addr EmailAddress) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(EMAIL_ADDR_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(addr))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(addr))
	return nil
}

func (patt URLPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(URLPATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(patt))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(patt))
	return nil
}
func (i Identifier) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(IDENT_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(i))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(i))
	return nil
}

func (p PropertyName) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(PROPNAME_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(string(p))
			return nil
		}, w)
		return nil
	}
	w.WriteString(string(p))
	return nil
}

func (str CheckedString) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNoRepresentation
}

func (count ByteCount) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer
	if _, err := count.Write(&buff, -1); err != nil {
		return err
	}

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(BYTECOUNT_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
		return nil
	}
	w.WriteString(buff.String())
	return nil
}

func (count LineCount) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer
	if count < 0 {
		return ErrNoRepresentation
	}
	if _, err := count.write(&buff); err != nil {
		return err
	}

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(LINECOUNT_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
		return nil
	}
	w.WriteString(buff.String())
	return nil
}

func (count RuneCount) writeJSON(w *jsoniter.Stream) (int, error) {
	var buff bytes.Buffer
	count.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (count RuneCount) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer
	if count < 0 {
		return ErrNoRepresentation
	}
	if _, err := count.write(&buff); err != nil {
		return err
	}

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(RUNECOUNT_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
		return nil
	}
	w.WriteString(buff.String())
	return nil
}

func (rate ByteRate) writeJSON(w *jsoniter.Stream) (int, error) {
	var buff bytes.Buffer
	rate.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (rate ByteRate) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer
	if rate < 0 {
		return ErrNoRepresentation
	}
	if _, err := rate.write(&buff); err != nil {
		return err
	}

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(BYTERATE_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
		return nil
	}
	w.WriteString(buff.String())
	return nil
}

func (rate SimpleRate) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer
	if rate < 0 {
		return ErrNoRepresentation
	}
	if _, err := rate.write(&buff); err != nil {
		return err
	}

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(SIMPLERATE_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
		return nil
	}
	w.WriteString(buff.String())
	return nil
}

func (d Duration) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer
	if d < 0 {
		return ErrNoRepresentation
	}
	if _, err := d.write(&buff); err != nil {
		return err
	}

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(DURATION_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
		return nil
	}
	w.WriteString(buff.String())
	return nil
}

func (d Date) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	var buff bytes.Buffer

	if _, err := d.write(&buff); err != nil {
		return err
	}

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(DATE_PATTERN.Name, func(w *jsoniter.Stream) error {
			w.WriteString(buff.String())
			return nil
		}, w)
		return nil
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

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(RUNE_RANGE_PATTERN.Name, func(w *jsoniter.Stream) error {
			write(w)
			return nil
		}, w)
		return nil
	}

	write(w)
	return nil
}

func (r QuantityRange) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

func (r IntRange) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()

		if !r.unknownStart {
			w.WriteObjectField("start")
			w.WriteString(strconv.FormatInt(int64(r.Start), 10))
			w.WriteMore()
		}

		w.WriteObjectField("end")
		w.WriteString(strconv.FormatInt(int64(r.End), 10))

		w.WriteObjectEnd()
		return nil
	}

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(INT_RANGE_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
		return nil
	}

	write(w)
	return nil
}

func (r FloatRange) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()

		if !r.unknownStart {
			w.WriteObjectField("start")
			w.WriteString(fmtFloat(float64(r.Start)))
			w.WriteMore()
		}

		w.WriteObjectField("end")
		w.WriteString(fmtFloat(float64(r.End)))

		w.WriteObjectEnd()
		return nil
	}

	if noPatternOrAny(config.Pattern) {
		writeUntypedValueJSON(INT_RANGE_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
		return nil
	}

	write(w)
	return nil
}

//patterns

func (pattern ExactValuePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (pattern TypePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
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
	return ErrNotImplementedYet
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
	return ErrNotImplementedYet
}

func (patt ObjectPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt RecordPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt ListPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
}

func (patt TuplePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}
	return ErrNotImplementedYet
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

//

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
	return Str(c.GetOrBuildString()).WriteJSONRepresentation(ctx, w, config, depth+1)
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

func (p *StructPattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	if depth > MAX_JSON_REPR_WRITING_DEPTH {
		return ErrMaximumJSONReprWritingDepthReached
	}

	return ErrNotImplementedYet
}

func noPatternOrAny(p Pattern) bool {
	return p == nil || p == ANYVAL_PATTERN || p == SERIALIZABLE_PATTERN
}
