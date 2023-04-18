package internal

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

// this file contains the implementation of Value.HasRepresentation & Value.WriteRepresentation for core types.

type ValueRepresentation []byte

func (r ValueRepresentation) Equal(r2 ValueRepresentation) bool {
	return bytes.Equal([]byte(r), []byte(r2))
}

func GetRepresentation(v Value, ctx *Context) ValueRepresentation {
	return MustGetRepresentationWithConfig(v, &ReprConfig{}, ctx)
}

func MustGetRepresentationWithConfig(v Value, config *ReprConfig, ctx *Context) ValueRepresentation {
	return utils.Must(GetRepresentationWithConfig(v, config, ctx))
}

func GetRepresentationWithConfig(v Value, config *ReprConfig, ctx *Context) (ValueRepresentation, error) {
	buff := bytes.NewBuffer(nil)
	encountered := map[uintptr]int{}
	err := v.WriteRepresentation(ctx, buff, encountered, config)
	if err != nil {
		return nil, fmt.Errorf("failed to get representation: %w", err)
	}
	return buff.Bytes(), nil
}

func WriteRepresentation(w io.Writer, v Value, config *ReprConfig, ctx *Context) error {
	encountered := map[uintptr]int{}
	err := v.WriteRepresentation(ctx, w, encountered, config)
	if err != nil {
		return fmt.Errorf("failed to write representation: %w", err)
	}
	return nil
}

type ReprConfig struct {
	allVisible bool
}

func (r *ReprConfig) IsValueVisible(v Value) bool {
	if r == nil || r.allVisible {
		return true
	}
	if IsAtomSensitive(v) {
		return false
	}
	return true
}

func (r *ReprConfig) IsPropertyVisible(name string, v Value, info *ValueVisibility, ctx *Context) bool {
	if r == nil || r.allVisible || (info != nil && utils.SliceContains(info.publicKeys, name)) {
		return true
	}

	if IsSensitiveProperty(ctx, name, v) || !r.IsValueVisible(v) {
		return false
	}
	return true
}

var ErrNoRepresentation = errors.New("no representation")

type NoReprMixin struct {
}

func (n NoReprMixin) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (n NoReprMixin) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (n NoReprMixin) HasJSONRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (n NoReprMixin) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (n AstNode) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (n AstNode) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (Nil NilT) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (Nil NilT) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write([]byte{'n', 'i', 'l'})
	return err
}

func (err Error) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (err Error) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (b Bool) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (b Bool) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if b {
		_, err := w.Write([]byte{'t', 'r', 'u', 'e'})
		return err
	} else {
		_, err := w.Write([]byte{'f', 'a', 'l', 's', 'e'})
		return err
	}
}

func (r Rune) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (r Rune) reprBytes() []byte {
	var b []byte

	switch r {
	case '\b':
		b = QUOTED_BELL_RUNE
	case '\f':
		b = QUOTED_FFEED_RUNE
	case '\n':
		b = QUOTED_NL_RUNE
	case '\r':
		b = QUOTED_CR_RUNE
	case '\t':
		b = QUOTED_TAB_RUNE
	case '\v':
		b = QUOTED_VTAB_RUNE
	case '\'':
		b = QUOTED_SQUOTE_RUNE
	case '\\':
		b = QUOTED_ASLASH_RUNE
	default:
		b = utils.StringAsBytes(fmt.Sprintf("'%c'", r))
	}

	return b
}

func (r Rune) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(r.reprBytes())
	return err
}

func (Byte) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (b Byte) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (Int) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (i Int) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	fmt.Fprint(w, i)
	return nil
}

func (Float) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (f Float) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	s := strconv.FormatFloat(float64(f), 'f', -1, 64)
	if _, err := w.Write(utils.StringAsBytes(s)); err != nil {
		return err
	}
	if !strings.Contains(s, ".") {
		if _, err := w.Write([]byte{'.', '0'}); err != nil {
			return err
		}
	}
	return nil
}

func (Str) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (s Str) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	jsonStr, err := utils.MarshalJsonNoHTMLEspace(string(s))
	if err != nil {
		return err
	}
	_, err = w.Write(jsonStr)
	return err
}

func (*RuneSlice) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (slice *RuneSlice) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	jsonStr, err := utils.MarshalJsonNoHTMLEspace(string(slice.elements))
	if err != nil {
		return err
	}

	_, err = w.Write(utils.StringAsBytes("Runes"))
	if err != nil {
		return err
	}

	_, err = w.Write(jsonStr)
	return err
}

func (obj *Object) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {

	ptr := reflect.ValueOf(obj).Pointer()
	if _, ok := encountered[ptr]; ok {
		return false
	}
	encountered[ptr] = -1

	obj.Lock(nil)
	defer obj.Unlock(nil)
	for _, v := range obj.values {
		if !v.HasRepresentation(encountered, config) {
			return false
		}
	}
	return true
}

func (obj *Object) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !obj.HasRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	_, err := w.Write([]byte{'{'})
	if err != nil {
		return err
	}

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
		err = v.WriteRepresentation(ctx, w, nil, config)
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

func (rec Record) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (rec Record) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	//TODO: prevent modification of the Object while this function is running

	if encountered != nil && !rec.HasRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	_, err := w.Write([]byte{'#', '{'})
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
		err = v.WriteRepresentation(ctx, w, nil, config)
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

func (dict *Dictionary) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	ptr := reflect.ValueOf(dict).Pointer()
	if _, ok := encountered[ptr]; ok {
		return false
	}
	encountered[ptr] = -1

	for _, v := range dict.Entries {
		if !v.HasRepresentation(encountered, config) {
			return false
		}
	}
	return true
}

func (dict *Dictionary) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !dict.HasRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	_, err := w.Write([]byte{':', '{'})
	if err != nil {
		return err
	}

	var keys []string
	for k := range dict.Entries {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	first := true
	for _, k := range keys {
		v := dict.Entries[k]
		if !first {
			_, err := w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = dict.Keys[k].WriteRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte{':'})
		if err != nil {
			return err
		}
		err = v.WriteRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}

	w.Write([]byte{'}'})
	return nil
}

func (list KeyList) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	if len(list) == 0 {
		return true
	}
	ptr := reflect.ValueOf(list).Pointer()
	if data, ok := encountered[ptr]; ok && len(list) == data {
		return false
	}
	encountered[ptr] = len(list)

	return true
}

func (list KeyList) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write([]byte{'.', '{'})
	if err != nil {
		return err
	}

	first := true
	for _, v := range list {
		if !first {
			_, err := w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		_, err := w.Write(utils.StringAsBytes(v))
		if err != nil {
			return err
		}
	}

	w.Write([]byte{'}'})
	return nil
}

func (list *List) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return list.underylingList.HasRepresentation(encountered, config)
}

func (list *List) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return list.underylingList.WriteRepresentation(ctx, w, encountered, config)
}

func (list *ValueList) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	ptr := reflect.ValueOf(list).Pointer()
	if _, ok := encountered[ptr]; ok {
		return false
	}
	encountered[ptr] = -1

	for _, v := range list.elements {
		if !v.HasRepresentation(encountered, config) {
			return false
		}
	}
	return true
}

func (list *ValueList) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !list.HasRepresentation(encountered, config) {
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
		err = v.WriteRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (list *IntList) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (list *IntList) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !list.HasRepresentation(encountered, config) {
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
		err = v.WriteRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (tuple *Tuple) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (tuple *Tuple) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !tuple.HasRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	_, err := w.Write([]byte{'#', '['})
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
		err = v.WriteRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (*ByteSlice) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (slice *ByteSlice) write(w io.Writer) (int, error) {
	totalN, err := w.Write([]byte{'0', 'x', '['})
	if err != nil {
		return totalN, err
	}

	n, err := hex.NewEncoder(w).Write(slice.Bytes)
	totalN += n
	if err != nil {
		return totalN, err
	}

	n, err = w.Write([]byte{']'})
	totalN += n
	return totalN, err
}

func (slice *ByteSlice) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := slice.write(w)
	return err
}

func (*GoFunction) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (v *GoFunction) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (opt Option) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	v, ok := opt.Value.(Bool)
	return ok && v == True
}

func (opt Option) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !opt.HasRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	if len(opt.Name) <= 1 {
		_, err := w.Write([]byte{'-'})
		if err != nil {
			return err
		}
	} else {
		_, err := w.Write([]byte{'-', '-'})
		if err != nil {
			return err
		}
	}

	_, err := w.Write(utils.StringAsBytes(opt.Name))
	if err != nil {
		return err
	}

	// _, err = w.Write([]byte{'='})
	// if err != nil {
	// 	return err
	// }
	// if err := opt.Value.WriteRepresentation(ctx, w, nil, config); err != nil {
	// 	return err
	// }
	return nil
}

func (Path) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (pth Path) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	quote := parse.ContainsSpace(string(pth))
	if !quote {
		for _, r := range pth {
			if parse.IsDelim(r) {
				quote = true
			}
		}
	}

	var b []byte
	if quote {
		i := strings.Index(string(pth), "/")
		b = append(b, pth[:i+1]...)
		b = append(b, '`')
		b = append(b, pth[i+1:]...)
		b = append(b, '`')
	} else {
		b = utils.StringAsBytes(pth)
	}
	_, err := w.Write(b)
	return err
}

func (PathPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (patt PathPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	quote := parse.ContainsSpace(string(patt))
	if !quote {
		for _, r := range patt {
			if parse.IsDelim(r) {
				quote = true
			}
		}
	}

	var b = []byte{'%'}
	if quote {
		i := strings.Index(string(patt), "/")
		b = append(b, patt[:i+1]...)
		b = append(b, '`')
		b = append(b, patt[i+1:]...)
		b = append(b, '`')
	} else {
		b = append(b, patt...)
	}
	_, err := w.Write(b)
	return err
}

func (URL) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (u URL) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.StringAsBytes(u))
	return err
}

func (Host) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (host Host) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.StringAsBytes(host))
	return err
}

func (Scheme) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (scheme Scheme) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.StringAsBytes(scheme + "://"))
	return err
}

func (HostPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (patt HostPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	var b = []byte{'%'}
	b = append(b, patt...)

	_, err := w.Write(b)
	return err
}

func (EmailAddress) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (addr EmailAddress) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.StringAsBytes(addr))
	return err
}

func (URLPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (patt URLPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	var b = []byte{'%'}
	b = append(b, patt...)

	_, err := w.Write(b)
	return err
}

func (Identifier) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (i Identifier) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write([]byte{'#'})
	if err != nil {
		return err
	}
	_, err = w.Write(utils.StringAsBytes(i))
	if err != nil {
		return err
	}
	return nil
}

func (PropertyName) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (p PropertyName) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write([]byte{'.'})
	if err != nil {
		return err
	}
	_, err = w.Write(utils.StringAsBytes(p))
	if err != nil {
		return err
	}
	return nil
}

func (CheckedString) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (str CheckedString) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write([]byte{'%'})
	if err != nil {
		return err
	}
	_, err = w.Write(utils.StringAsBytes(str.matchingPatternName))
	if err != nil {
		return err
	}
	jsonStr, _ := utils.MarshalJsonNoHTMLEspace(str.str)
	_, err = w.Write([]byte{'`'})
	if err != nil {
		return err
	}
	_, err = w.Write(jsonStr[1 : len(jsonStr)-1])
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'`'})
	return err
}

func (ByteCount) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (count ByteCount) write(w io.Writer) (int, error) {
	format := "%dB"
	var v = int64(count)

	switch {
	case count >= 1_000_000_000 && count%1_000_000_000 == 0:
		format = "%dGB"
		v /= 1_000_000_000
	case count >= 1_000_000 && count%1_000_000 == 0:
		format = "%dMB"
		v /= 1_000_000
	case count >= 1000 && count%1_000 == 0:
		format = "%dkB"
		v /= 1_000
	case count >= 0:
		break
	case count < 0:
		return 0, ErrNoRepresentation
	}
	return fmt.Fprintf(w, format, v)
}

func (count ByteCount) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := count.write(w)
	return err
}

func (LineCount) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (count LineCount) write(w io.Writer) (int, error) {
	return fmt.Fprintf(w, "%dln", count)
}

func (count LineCount) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if count < 0 {
		return ErrNoRepresentation
	}

	_, err := count.write(w)
	return err
}

func (RuneCount) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (count RuneCount) write(w io.Writer) (int, error) {
	return fmt.Fprintf(w, "%drn", count)
}

func (count RuneCount) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if count < 0 {
		return ErrNoRepresentation
	}

	_, err := count.write(w)
	return err
}

func (ByteRate) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (rate ByteRate) write(w io.Writer) (int, error) {
	totalN := 0
	if n, err := ByteCount(rate).write(w); err != nil {
		return n, err
	} else {
		totalN = n
	}
	n, err := w.Write([]byte{'/', 's'})
	totalN += n
	return totalN, err
}

func (rate ByteRate) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if rate < 0 {
		return ErrNoRepresentation
	}

	_, err := rate.write(w)
	return err
}

func (SimpleRate) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (rate SimpleRate) write(w io.Writer) (int, error) {
	var format = "%dx/s"
	var v = int64(rate)

	switch {
	case rate >= 1_000_000_000 && rate%1_000_000_000 == 0:
		format = "%dGx/s"
		v /= 1_000_000_000
	case rate >= 1_000_000 && rate%1_000_000 == 0:
		format = "%dMx/s"
		v /= 1_000_000
	case rate >= 1_000 && rate%1_000 == 0:
		format = "%dkx/s"
		v /= 1_000
	case rate >= 0:
		break
	default:
		return 0, ErrNoRepresentation
	}

	return fmt.Fprintf(w, format, v)
}

func (rate SimpleRate) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if rate < 0 {
		return ErrNoRepresentation
	}

	_, err := rate.write(w)
	return err
}

func (Duration) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (d Duration) write(w io.Writer) (int, error) {
	if d == 0 {
		return w.Write(utils.StringAsBytes("0s"))
	}

	v := time.Duration(d)
	b := make([]byte, 0, 32)

	for v != 0 {
		switch {
		case v >= time.Hour:
			b = strconv.AppendUint(b, uint64(v/time.Hour), 10)
			b = append(b, 'h')
			v %= time.Hour
		case v >= time.Minute:
			b = strconv.AppendUint(b, uint64(v/time.Minute), 10)
			b = append(b, "mn"...)
			v %= time.Minute
		case v >= time.Second:
			b = strconv.AppendUint(b, uint64(v/time.Second), 10)
			b = append(b, 's')
			v %= time.Second
		case v >= time.Millisecond:
			b = strconv.AppendUint(b, uint64(v/time.Millisecond), 10)
			b = append(b, "ms"...)
			v %= time.Millisecond
		case v >= time.Microsecond:
			b = strconv.AppendUint(b, uint64(v/time.Microsecond), 10)
			b = append(b, "us"...)
			v %= time.Microsecond
		default:
			b = strconv.AppendUint(b, uint64(v), 10)
			b = append(b, "ns"...)
			v = 0
		}
	}

	return w.Write(b)
}

func (d Duration) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := d.write(w)
	return err
}

func (Date) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (d Date) write(w io.Writer) (int, error) {
	// TODO: change
	t := time.Time(d)
	ns := t.Nanosecond()
	ms := ns / 1_000_000
	us := (ns % 1_000_000) / 1000

	return fmt.Fprintf(w, "%dy-%dmt-%dd-%dh-%dm-%ds-%dms-%dus-%s",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), ms, us, t.Location().String())
}

func (d Date) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := d.write(w)
	return err
}

func (FileMode) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (m FileMode) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RuneRange) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (r RuneRange) write(w io.Writer) (int, error) {
	b := []byte{'\''}
	b = append(b, string(r.Start)...)
	b = append(b, '\'', '.', '.', '\'')
	b = append(b, string(r.End)...)
	b = append(b, '\'')

	return w.Write(b)
}

func (r RuneRange) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := r.write(w)
	return err
}

func (r QuantityRange) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	if r.Start != nil && !r.Start.HasRepresentation(encountered, config) {
		return false
	}
	return r.End == nil || r.End.HasRepresentation(encountered, config)
}

func (r QuantityRange) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !r.HasRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	if r.Start != nil {
		err := r.Start.WriteRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}

	_, err := w.Write([]byte{'.', '.'})
	if err != nil {
		return err
	}

	if r.End != nil {
		err := r.End.WriteRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}
	return nil
}

func (IntRange) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (r IntRange) write(w io.Writer) (int, error) {
	b := make([]byte, 0, 10)
	if !r.unknownStart {
		b = append(b, strconv.FormatInt(r.Start, 10)...)
	}
	b = append(b, '.', '.')
	b = append(b, strconv.FormatInt(r.End, 10)...)

	return w.Write(b)
}

func (r IntRange) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := r.write(w)
	return err
}

//patterns

func (ExactValuePattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (pattern ExactValuePattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (p *TypePattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	_, ok := DEFAULT_NAMED_PATTERNS[p.Name]
	return ok
}

func (pattern TypePattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if !pattern.HasRepresentation(encountered, config) {
		return ErrNoRepresentation
	}
	_, err := w.Write(utils.StringAsBytes("%" + pattern.Name))
	return err
}

func (DifferencePattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (pattern DifferencePattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (*OptionalPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (pattern *OptionalPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RegexPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt RegexPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (UnionPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt UnionPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (SequenceStringPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt SequenceStringPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (UnionStringPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt UnionStringPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RuneRangeStringPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt RuneRangeStringPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (*IntRangePattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt *IntRangePattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (DynamicStringPatternElement) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt DynamicStringPatternElement) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RepeatedPatternElement) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt *RepeatedPatternElement) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (NamedSegmentPathPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (patt *NamedSegmentPathPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(utils.StringAsBytes(patt.node.Raw))
	return err
}

func (ObjectPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt ObjectPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (RecordPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt RecordPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (ListPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt ListPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (TuplePattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt TuplePattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (OptionPattern) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (patt OptionPattern) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (Reader) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (reader Reader) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (Mimetype) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (mt Mimetype) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (FileInfo) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (i FileInfo) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (*Routine) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (r *Routine) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (*RoutineGroup) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (g *RoutineGroup) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (f *InoxFunction) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (f *InoxFunction) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (b *Bytecode) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (b *Bytecode) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (it IntRangeIterator) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (it IntRangeIterator) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (it RuneRangeIterator) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (it RuneRangeIterator) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (it *PatternIterator) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (it *PatternIterator) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (it indexedEntryIterator) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (it indexedEntryIterator) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (it *EventSourceIterator) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (it *EventSourceIterator) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (it *DirWalker) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (it *DirWalker) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (it *ValueListIterator) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (it *ValueListIterator) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (it *TupleIterator) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (it *TupleIterator) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (t Type) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return false
}

func (t Type) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return ErrNoRepresentation
}

func (port Port) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (port Port) repr(quote bool) []byte {
	b := make([]byte, 0, 10)
	if quote {
		b = append(b, '"')
	}
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(port.Number), 10)
	if port.Scheme != NO_SCHEME_SCHEME && port.Scheme != "" {
		b = append(b, '/')
		b = append(b, port.Scheme...)
	}
	if quote {
		b = append(b, '"')
	}

	return b
}

func (port Port) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write(port.repr(false))
	return err
}

func (udata *UData) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	ptr := reflect.ValueOf(udata).Pointer()
	if _, ok := encountered[ptr]; ok {
		return false
	}
	encountered[ptr] = -1

	for _, entry := range udata.HiearchyEntries {
		if !entry.HasRepresentation(encountered, config) {
			return false
		}
	}
	return true
}

func (udata *UData) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	//TODO: prevent modification of the Object while this function is running

	if encountered != nil && !udata.HasRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	_, err := w.Write([]byte{'u', 'd', 'a', 't', 'a', ' '})
	if err != nil {
		return err
	}

	if udata.Root != nil {
		err = udata.Root.WriteRepresentation(ctx, w, nil, config)
		if err != nil {
			return err
		}
	}
	w.Write([]byte{'{'})

	first := true

	for _, entry := range udata.HiearchyEntries {
		if !first {
			w.Write([]byte{','})
		}
		first = false

		if err := entry.WriteRepresentation(ctx, w, encountered, config); err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{'}'})
	if err != nil {
		return err
	}
	return nil
}

func (entry UDataHiearchyEntry) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {

	if !entry.Value.HasRepresentation(encountered, config) {
		return false
	}

	for _, child := range entry.Children {
		if !child.HasRepresentation(encountered, config) {
			return false
		}
	}
	return true
}

func (entry UDataHiearchyEntry) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	if encountered != nil && !entry.HasRepresentation(encountered, config) {
		return ErrNoRepresentation
	}

	if err := entry.Value.WriteRepresentation(ctx, w, encountered, config); err != nil {
		return err
	}

	if len(entry.Children) > 0 {
		first := true

		w.Write([]byte{'{'})
		for _, child := range entry.Children {
			if !first {
				w.Write([]byte{','})
			}
			first = false

			err := child.WriteRepresentation(ctx, w, nil, config)
			if err != nil {
				ctx.Logf("%#v has no repr", child)
				return err
			}
		}
		w.Write([]byte{'}'})
	}
	return nil
}

func (c *StringConcatenation) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (c *StringConcatenation) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	return Str(c.GetOrBuildString()).WriteRepresentation(ctx, w, encountered, config)
}

func (c Color) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (c Color) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	panic(ErrNotImplementedYet)
}
