package internal

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"github.com/inoxlang/inox/internal/utils"
)

// this file contains the implementation of Value.HasJSONRepresentation & Value.WriteJSONRepresentation for core types.

func GetJSONRepresentation(v Value, ctx *Context) string {
	buff := bytes.NewBuffer(nil)
	encountered := map[uintptr]int{}
	err := v.WriteJSONRepresentation(ctx, buff, encountered, &ReprConfig{})
	if err != nil {
		panic(fmt.Errorf("%s: %w", Stringify(v, ctx), err))
	}
	return buff.String()
}

func (n AstNode) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (n AstNode) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (Nil NilT) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (Nil NilT) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write([]byte{'n', 'u', 'l', 'l'})
	return err
}

func (err Error) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (err Error) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (b Bool) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (b Bool) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if b {
		_, err := w.Write([]byte{'t', 'r', 'u', 'e'})
		return err
	} else {
		_, err := w.Write([]byte{'f', 'a', 'l', 's', 'e'})
		return err
	}
}

func (r Rune) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (r Rune) writeJSON(w io.Writer) (int, error) {
	b := []byte("\"")
	b = append(b, r.reprBytes()...)
	b = append(b, '"')
	return w.Write(b)
}

func (r Rune) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := r.writeJSON(w)
	return err
}

func (Byte) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (b Byte) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (Int) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (i Int) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	fmt.Fprintf(w, `"%d"`, i)
	return nil
}

func (Float) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (f Float) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(f)))
	return err
}

func (Str) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (s Str) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	jsonStr, err := utils.MarshalJsonNoHTMLEspace(string(s))
	if err != nil {
		return err
	}
	_, err = w.Write(jsonStr)
	return err
}

func (obj *Object) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {

	ptr := reflect.ValueOf(obj).Pointer()
	if _, ok := encountered[ptr]; ok {
		return false
	}
	encountered[ptr] = -1

	obj.Lock(nil)
	defer obj.Unlock(nil)
	for _, v := range obj.values {
		if !v.HasJSONRepresentation(encountered, config) {
			return false
		}
	}
	return true
}

func (obj *Object) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	//TODO: prevent modification of the Object while this function is running

	if encountered != nil && !obj.HasJSONRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	_, err := w.Write([]byte{'{'})
	if err != nil {
		return err
	}

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	keys := obj.keys
	visibility, _ := GetVisibility(obj.visibilityId)

	first := true
	for i, k := range keys {
		v := obj.values[i]

		if !config.IsPropertyVisible(k, v, visibility, ctx) {
			continue
		}

		if !first {
			w.Write([]byte{','})
		}
		first = false
		jsonStr, _ := utils.MarshalJsonNoHTMLEspace(k)
		_, err = w.Write(jsonStr)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte{':'})
		if err != nil {
			return err
		}
		err = v.WriteJSONRepresentation(ctx, w, nil, config)
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

func (rec *Record) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (rec *Record) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {

	if encountered != nil && !rec.HasJSONRepresentation(encountered, config) {
		return ErrNoRepresentation
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
			w.Write([]byte{','})
		}

		first = false
		jsonStr, _ := utils.MarshalJsonNoHTMLEspace(k)
		_, err = w.Write(jsonStr)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte{':'})
		if err != nil {
			return err
		}
		err = v.WriteJSONRepresentation(ctx, w, nil, config)
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

func (dict *Dictionary) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (dict *Dictionary) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (list KeyList) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (list KeyList) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation

}

func (list *List) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return list.underylingList.HasJSONRepresentation(encountered, config)
}

func (list *List) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return list.underylingList.WriteJSONRepresentation(ctx, w, encountered, config)
}

func (list *ValueList) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	ptr := reflect.ValueOf(list).Pointer()
	if _, ok := encountered[ptr]; ok {
		return false
	}
	encountered[ptr] = -1

	for _, v := range list.elements {
		if !v.HasJSONRepresentation(encountered, config) {
			return false
		}
	}
	return true
}

func (list *ValueList) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !list.HasJSONRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	_, err := w.Write([]byte{'['})
	if err != nil {
		return err
	}
	first := true
	for _, v := range list.elements {

		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = v.WriteJSONRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (list *IntList) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (list *IntList) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !list.HasJSONRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	_, err := w.Write([]byte{'['})
	if err != nil {
		return err
	}
	first := true
	for _, v := range list.Elements {
		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = v.WriteJSONRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (tuple *BoolList) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (list *BoolList) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !list.HasJSONRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	return list.WriteRepresentation(ctx, w, nil, config)
}

func (list *StringList) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (list *StringList) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !list.HasJSONRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	_, err := w.Write([]byte{'['})
	if err != nil {
		return err
	}
	first := true
	for _, v := range list.elements {
		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = v.WriteJSONRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (tuple *Tuple) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (tuple *Tuple) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !tuple.HasJSONRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	_, err := w.Write([]byte{'['})
	if err != nil {
		return err
	}

	first := true
	for _, v := range tuple.elements {

		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = v.WriteJSONRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (*RuneSlice) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (slice *RuneSlice) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (*ByteSlice) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (slice *ByteSlice) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (*GoFunction) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (v *GoFunction) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (opt Option) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	v, ok := opt.Value.(Bool)
	return ok && v == True
}

func (opt Option) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !opt.HasJSONRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	b := make([]byte, 0, len(opt.Name)+4)

	if len(opt.Name) <= 1 {
		b = append(b, '-')
	} else {
		b = append(b, '-', '-')
	}

	b = append(b, opt.Name...)

	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(string(b))))
	if err != nil {
		return err
	}

	// _, err = w.Write([]byte{'='})
	// if err != nil {
	// 	return err
	// }
	// if err := opt.Value.WriteJSONRepresentation(ctx, w, nil); err != nil {
	// 	return err
	// }
	return nil
}

func (Path) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (pth Path) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(pth)))
	return err
}

func (PathPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (patt PathPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace("%" + patt)))
	return err
}

func (URL) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (u URL) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(u)))
	return err
}

func (Host) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (host Host) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(host)))
	return err
}

func (Scheme) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (scheme Scheme) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(scheme + "://")))
	return err
}

func (HostPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (patt HostPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace("%" + patt)))
	return err
}

func (EmailAddress) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (addr EmailAddress) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(addr)))
	return err
}

func (URLPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (patt URLPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace("%" + patt)))
	return err
}

func (Identifier) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (i Identifier) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace("#" + i)))
	return err
}

func (PropertyName) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (p PropertyName) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace("." + p)))
	return err
}

func (CheckedString) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (str CheckedString) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	var buff bytes.Buffer
	str.WriteRepresentation(ctx, &buff, encountered, config)

	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
	return err
}

func (ByteCount) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (count ByteCount) writeJSON(w io.Writer) (int, error) {
	var buff bytes.Buffer
	count.Write(&buff, -1)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (count ByteCount) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := count.writeJSON(w)
	return err
}

func (LineCount) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (count LineCount) writeJSON(w io.Writer) (int, error) {
	var buff bytes.Buffer
	count.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (count LineCount) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if count < 0 {
		return ErrNoRepresentation
	}

	_, err := count.writeJSON(w)
	return err
}

func (RuneCount) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (count RuneCount) writeJSON(w io.Writer) (int, error) {
	var buff bytes.Buffer
	count.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (count RuneCount) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if count < 0 {
		return ErrNoRepresentation
	}

	_, err := count.writeJSON(w)
	return err
}

func (ByteRate) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (rate ByteRate) writeJSON(w io.Writer) (int, error) {
	var buff bytes.Buffer
	rate.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (rate ByteRate) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if rate < 0 {
		return ErrNoRepresentation
	}

	_, err := rate.writeJSON(w)
	return err
}

func (SimpleRate) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (rate SimpleRate) writeJSON(w io.Writer) (int, error) {
	var buff bytes.Buffer
	rate.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (rate SimpleRate) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if rate < 0 {
		return ErrNoRepresentation
	}

	_, err := rate.writeJSON(w)
	return err
}

func (Duration) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (d Duration) writeJSON(w io.Writer) (int, error) {
	var buff bytes.Buffer
	d.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (d Duration) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := d.writeJSON(w)
	return err
}

func (Date) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (d Date) writeJSON(w io.Writer) (int, error) {
	var buff bytes.Buffer
	d.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (d Date) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := d.writeJSON(w)
	return err
}

func (FileMode) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (m FileMode) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RuneRange) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (r RuneRange) writeJSON(w io.Writer) (int, error) {
	var buff bytes.Buffer
	r.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (r RuneRange) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := r.writeJSON(w)
	return err
}

func (r QuantityRange) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return r.HasRepresentation(encountered, config)
}

func (r QuantityRange) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	var buff bytes.Buffer
	r.WriteRepresentation(ctx, &buff, encountered, config)

	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
	return err
}

func (IntRange) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (r IntRange) writeJSON(w io.Writer) (int, error) {
	var buff bytes.Buffer
	r.write(&buff)

	return w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
}

func (r IntRange) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := r.writeJSON(w)
	return err
}

//patterns

func (ExactValuePattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (pattern ExactValuePattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (TypePattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (pattern TypePattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (*DifferencePattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (pattern *DifferencePattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (*OptionalPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (pattern *OptionalPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RegexPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt RegexPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (UnionPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt UnionPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (SequenceStringPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt SequenceStringPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (UnionStringPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt UnionStringPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RuneRangeStringPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt RuneRangeStringPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (*IntRangePattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt *IntRangePattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (DynamicStringPatternElement) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt DynamicStringPatternElement) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RepeatedPatternElement) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt *RepeatedPatternElement) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (NamedSegmentPathPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (patt *NamedSegmentPathPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	var buff bytes.Buffer
	patt.WriteRepresentation(ctx, &buff, encountered, config)

	_, err := w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(buff.String())))
	return err
}

func (ObjectPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt ObjectPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RecordPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt RecordPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (ListPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt ListPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (TuplePattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt TuplePattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (OptionPattern) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt OptionPattern) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (Reader) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (reader Reader) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (Mimetype) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (mt Mimetype) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (FileInfo) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (i FileInfo) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (b *Bytecode) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (b *Bytecode) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (t Type) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (t Type) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (Port) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (port Port) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(port.repr(true))
	return err
}

func (*StringConcatenation) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (c *StringConcatenation) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return Str(c.GetOrBuildString()).WriteJSONRepresentation(ctx, w, encountered, config)
}

func (Color) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (c Color) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	panic(ErrNotImplementedYet)
}
