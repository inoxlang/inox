package core

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/globalnames"
	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
)

// this file contains the implementation of Value.HasRepresentation & Value.WriteRepresentation for core types.

const (
	MAX_REPR_WRITING_DEPTH = 20
)

var (
	ErrMaximumReprWritingDepthReached = errors.New("maximum representation writing depth reached")

	OPENING_BRACKET = []byte{'['}
	CLOSING_BRACKET = []byte{']'}
)

type ValueRepresentation []byte

func (r ValueRepresentation) Equal(r2 ValueRepresentation) bool {
	return bytes.Equal([]byte(r), []byte(r2))
}

func GetRepresentation(v Serializable, ctx *Context) ValueRepresentation {
	return MustGetRepresentationWithConfig(v, &ReprConfig{}, ctx)
}

func MustGetRepresentationWithConfig(v Serializable, config *ReprConfig, ctx *Context) ValueRepresentation {
	return utils.Must(GetRepresentationWithConfig(v, config, ctx))
}

func GetRepresentationWithConfig(v Serializable, config *ReprConfig, ctx *Context) (ValueRepresentation, error) {
	buff := bytes.NewBuffer(nil)

	err := v.WriteRepresentation(ctx, buff, config, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get representation: %w", err)
	}
	return buff.Bytes(), nil
}

func WriteRepresentation(w io.Writer, v Serializable, config *ReprConfig, ctx *Context) error {

	err := v.WriteRepresentation(ctx, w, config, 0)
	if err != nil {
		return fmt.Errorf("failed to write representation: %w", err)
	}
	return nil
}

type ReprConfig struct {
	AllVisible bool
}

func (r *ReprConfig) IsValueVisible(v Value) bool {
	if r == nil || r.AllVisible {
		return true
	}
	if IsAtomSensitive(v) {
		return false
	}
	return true
}

func (r *ReprConfig) IsPropertyVisible(name string, v Value, info *ValueVisibility, ctx *Context) bool {
	if r == nil || r.AllVisible || (info != nil && utils.SliceContains(info.publicKeys, name)) {
		return true
	}

	if IsSensitiveProperty(ctx, name, v) || !r.IsValueVisible(v) {
		return false
	}
	return true
}

var ErrNoRepresentation = errors.New("no representation")

type CallBasedPatternReprMixin struct {
	Callee Pattern
	Params []Serializable
}

func (m CallBasedPatternReprMixin) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	err := m.Callee.WriteRepresentation(ctx, w, config, depth+1)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'('})
	if err != nil {
		return err
	}

	for i, p := range m.Params {
		if i != 0 {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		err := p.WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{')'})
	return err
}

func (m CallBasedPatternReprMixin) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

type NamespaceMemberPatternReprMixin struct {
	NamespaceName string
	MemberName    string
}

func (m NamespaceMemberPatternReprMixin) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	repr := "%" + m.NamespaceName + "." + m.MemberName
	_, err := w.Write(utils.StringAsBytes(repr))
	return err
}

func (m NamespaceMemberPatternReprMixin) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	return ErrNotImplementedYet
}

// implementations

func (Nil NilT) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := w.Write([]byte{'n', 'i', 'l'})
	return err
}

func (b Bool) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if b {
		_, err := w.Write([]byte{'t', 'r', 'u', 'e'})
		return err
	} else {
		_, err := w.Write([]byte{'f', 'a', 'l', 's', 'e'})
		return err
	}
}

func (r Rune) reprBytes() []byte {
	return []byte(commonfmt.FmtRune(rune(r)))
}

func (r Rune) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := w.Write(r.reprBytes())
	return err
}

func (b Byte) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNoRepresentation
}

func (i Int) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	fmt.Fprint(w, i)
	return nil
}

func fmtFloat(f float64) string {
	return strconv.FormatFloat(float64(f), 'f', -1, 64)
}

func (f Float) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	s := fmtFloat(float64(f))
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

func (s Str) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	jsonStr, err := utils.MarshalJsonNoHTMLEspace(string(s))
	if err != nil {
		return err
	}
	_, err = w.Write(jsonStr)
	return err
}

func (slice *RuneSlice) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

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

func (obj *Object) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	_, err := w.Write([]byte{'{'})
	if err != nil {
		return err
	}

	first := true

	//meta properties

	if obj.url != "" {
		first = false

		_, err = w.Write(utils.StringAsBytes(`"_url_":`))
		if err != nil {
			return err
		}

		err = obj.url.WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
	}

	//properties
	keys := obj.keys
	visibility, _ := GetVisibility(obj.visibilityId)

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
		err = v.WriteRepresentation(ctx, w, config, depth+1)
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

func (rec Record) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
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
		err = v.WriteRepresentation(ctx, w, config, depth+1)
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

func (dict *Dictionary) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	_, err := w.Write([]byte{':', '{'})
	if err != nil {
		return err
	}

	var keys []string
	for k := range dict.entries {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	first := true
	for _, k := range keys {
		v := dict.entries[k]
		if !first {
			_, err := w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = dict.keys[k].WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte{':'})
		if err != nil {
			return err
		}
		err = v.WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
	}

	w.Write([]byte{'}'})
	return nil
}

func (list KeyList) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

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

func (list *List) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	return list.underlyingList.WriteRepresentation(ctx, w, config, depth)
}

func (list *ValueList) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
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
		err = v.WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (list *IntList) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
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
		err = v.WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (list *BoolList) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {

	_, err := w.Write(OPENING_BRACKET)
	if err != nil {
		return err
	}

	length := int(list.elements.Len())
	if length == 0 {
		_, err := w.Write(CLOSING_BRACKET)
		return err
	}

	counter := 0
	i, found := list.elements.NextSet(0)

	if !found { //all zeroes
		b := bytes.Repeat(utils.StringAsBytes("false, "), length)
		b = b[:len(b)-2] //remove trailing ', '
		_, err := w.Write(b)
		if err != nil {
			return err
		}

		_, err = w.Write(CLOSING_BRACKET)
		return err
	}

	zeroCount := i

	for found {
		if zeroCount > 0 {
			b := bytes.Repeat(utils.StringAsBytes("false, "), length)
			_, err := w.Write(b)
			if err != nil {
				return err
			}
		}

		counter = counter + 1
		if i == uint(length)-1 { //last
			w.Write(utils.StringAsBytes("true"))
		} else {
			w.Write(utils.StringAsBytes("true, "))
		}

		prevI := i
		i, found = list.elements.NextSet(i + 1)
		zeroCount = i - prevI - 1

		if found {
			w.Write(COMMA)
		}
	}

	_, err = w.Write(CLOSING_BRACKET)
	return err
}

func (list *StringList) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
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
		err = v.WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
}

func (tuple *Tuple) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
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
		err = v.WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{']'})
	return err
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

func (slice *ByteSlice) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}
	_, err := slice.write(w)
	return err
}

func (opt Option) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {

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
	// if err := opt.Value.WriteRepresentation(ctx, w, config); err != nil {
	// 	return err
	// }
	return nil
}

func (pth Path) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
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

func (patt PathPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
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

func (u URL) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := w.Write(utils.StringAsBytes(u))
	return err
}

func (host Host) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := w.Write(utils.StringAsBytes(host))
	return err
}

func (scheme Scheme) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := w.Write(utils.StringAsBytes(scheme + "://"))
	return err
}

func (patt HostPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	var b = []byte{'%'}
	b = append(b, patt...)

	_, err := w.Write(b)
	return err
}

func (addr EmailAddress) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if config.AllVisible || len(addr) < 5 {
		_, err := w.Write(utils.StringAsBytes(addr))
		return err
	}

	addrS := string(addr)
	atDomainIndex := strings.LastIndexByte(addrS, '@')
	if atDomainIndex < 0 {
		return fmt.Errorf("invalid email address")
	}

	name := addrS[:atDomainIndex]
	atDomain := addrS[atDomainIndex:]

	_, err := w.Write(utils.StringAsBytes(name[0:1] + strings.Repeat("*", len(name)-1) + atDomain))
	return err
}

func (patt URLPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	var b = []byte{'%'}
	b = append(b, patt...)

	_, err := w.Write(b)
	return err
}

func (i Identifier) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
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

func (p PropertyName) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
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

func (str CheckedString) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
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

func (count ByteCount) Write(w io.Writer, _3digitGroupCount int) (int, error) {
	s, err := commonfmt.FmtByteCount(int64(count), _3digitGroupCount)
	if err != nil {
		return 0, err
	}
	return w.Write(utils.StringAsBytes(s))
}

func (count ByteCount) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := count.Write(w, -1)
	return err
}

func (count LineCount) write(w io.Writer) (int, error) {
	return fmt.Fprintf(w, "%dln", count)
}

func (count LineCount) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if count < 0 {
		return ErrNoRepresentation
	}

	_, err := count.write(w)
	return err
}

func (count RuneCount) write(w io.Writer) (int, error) {
	return fmt.Fprintf(w, "%drn", count)
}

func (count RuneCount) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if count < 0 {
		return ErrNoRepresentation
	}

	_, err := count.write(w)
	return err
}

func (rate ByteRate) write(w io.Writer) (int, error) {
	totalN := 0
	if n, err := ByteCount(rate).Write(w, -1); err != nil {
		return n, err
	} else {
		totalN = n
	}
	n, err := w.Write([]byte{'/', 's'})
	totalN += n
	return totalN, err
}

func (rate ByteRate) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if rate < 0 {
		return ErrNoRepresentation
	}

	_, err := rate.write(w)
	return err
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

func (rate SimpleRate) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if rate < 0 {
		return ErrNoRepresentation
	}

	_, err := rate.write(w)
	return err
}

func (d Duration) write(w io.Writer) (int, error) {
	return w.Write(utils.StringAsBytes(commonfmt.FmtInoxDuration(time.Duration(d))))
}

func (d Duration) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := d.write(w)
	return err
}

func (d Date) write(w io.Writer) (int, error) {
	return w.Write(utils.StringAsBytes(commonfmt.FmtInoxDate(time.Time(d))))
}

func (d Date) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := d.write(w)
	return err
}

func (m FileMode) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if _, err := w.Write(utils.StringAsBytes(globalnames.FILEMODE_FN)); err != nil {
		return err
	}

	if _, err := w.Write([]byte{'('}); err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, uint32(m)); err != nil {
		return err
	}

	_, err := w.Write([]byte{')'})
	return err
}

func (r RuneRange) write(w io.Writer) (int, error) {
	b := []byte{'\''}
	b = append(b, string(r.Start)...)
	b = append(b, '\'', '.', '.', '\'')
	b = append(b, string(r.End)...)
	b = append(b, '\'')

	return w.Write(b)
}

func (r RuneRange) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := r.write(w)
	return err
}

func (r QuantityRange) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	if r.start != nil {
		err := r.start.WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
	}

	_, err := w.Write([]byte{'.', '.'})
	if err != nil {
		return err
	}

	if r.end != nil {
		err := r.InclusiveEnd().WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}
	}
	return nil
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

func (r IntRange) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := r.write(w)
	return err
}

func (r FloatRange) write(w io.Writer) (int, error) {
	b := make([]byte, 0, 10)
	if !r.unknownStart {
		repr := fmtFloat(r.Start)
		b = append(b, repr...)

		hasPoint := false
		for _, r := range repr {
			if r == '.' {
				hasPoint = true
			}
		}

		if !hasPoint {
			b = append(b, '.', '0')
		}
	}
	b = append(b, '.', '.')

	repr := fmtFloat(r.End)
	b = append(b, repr...)

	hasPoint := false
	for _, r := range repr {
		if r == '.' {
			hasPoint = true
		}
	}

	if !hasPoint {
		b = append(b, '.', '0')
	}

	return w.Write(b)
}

func (r FloatRange) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := r.write(w)
	return err
}

//patterns

func (p *ExactValuePattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	_, err := w.Write([]byte{'%', '('})
	if err != nil {
		return err
	}

	err = p.value.WriteRepresentation(ctx, w, config, depth+1)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte{')'})
	if err != nil {
		return err
	}
	return nil
}

func (pattern TypePattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := w.Write(utils.StringAsBytes("%" + pattern.Name))
	return err
}

func (pattern DifferencePattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (pattern *OptionalPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt RegexPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt UnionPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt IntersectionPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt LengthCheckingStringPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt SequenceStringPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt UnionStringPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt RuneRangeStringPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt DynamicStringPatternElement) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt *RepeatedPatternElement) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt *NamedSegmentPathPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := w.Write(utils.StringAsBytes(patt.node.Raw))
	return err
}

func (p *ObjectPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	_, err := w.Write([]byte{'%', '{'})
	if err != nil {
		return err
	}

	keys := maps.Keys(p.entryPatterns)
	sort.Strings(keys)

	first := true
	for _, k := range keys {
		entryPattern := p.entryPatterns[k]

		if !first {
			w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false

		//key
		{
			jsonStr, _ := utils.MarshalJsonNoHTMLEspace(k)
			_, err = w.Write(jsonStr)
			if err != nil {
				return err
			}

			if _, ok := p.optionalEntries[k]; ok {
				w.Write([]byte{'?'})
				if err != nil {
					return err
				}
			}
		}

		_, err = w.Write([]byte{':'})
		if err != nil {
			return err
		}

		//value
		err = entryPattern.WriteRepresentation(ctx, w, config, depth+1)
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

func (patt RecordPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (p *ListPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	if p.elementPatterns != nil {
		_, err := w.Write([]byte{'%', '['})
		if err != nil {
			return err
		}

		for i, e := range p.elementPatterns {
			err = e.WriteRepresentation(ctx, w, config, depth+1)
			if err != nil {
				return err
			}

			//comma & indent
			isLastEntry := i == len(p.elementPatterns)-1

			if !isLastEntry {
				utils.Must(w.Write([]byte{','}))
			}
		}
		_, err = w.Write([]byte{']'})
		if err != nil {
			return err
		}

		return nil
	}

	generalElementPattern := p.generalElementPattern
	switch generalElementPattern.(type) {
	case *ExactValuePattern, *ExactStringPattern:
		_, err := w.Write([]byte{'%', '[', ']'})
		if err != nil {
			return err
		}

		return generalElementPattern.WriteRepresentation(ctx, w, config, depth+1)
	default:
		//surround the general element pattern with %( )

		_, err := w.Write([]byte{'%', '[', ']', '%', '('})
		if err != nil {
			return err
		}

		err = generalElementPattern.WriteRepresentation(ctx, w, config, depth+1)
		if err != nil {
			return err
		}

		_, err = w.Write([]byte{')'})
		return err
	}

}

func (patt TuplePattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt OptionPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt PathStringPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (patt *FunctionPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

//

func (mt Mimetype) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNoRepresentation
}

func (i FileInfo) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNoRepresentation
}

func (b *Bytecode) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNoRepresentation
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

func (port Port) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	_, err := w.Write(port.repr(false))
	return err
}

func (udata *UData) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	//TODO: prevent modification of the Object while this function is running
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	_, err := w.Write([]byte{'u', 'd', 'a', 't', 'a', ' '})
	if err != nil {
		return err
	}

	if udata.Root != nil {
		err = udata.Root.WriteRepresentation(ctx, w, config, depth+1)
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

		if err := entry.WriteRepresentation(ctx, w, config, depth+1); err != nil {
			return err
		}
	}

	_, err = w.Write([]byte{'}'})
	if err != nil {
		return err
	}
	return nil
}

func (entry UDataHiearchyEntry) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	if depth > MAX_REPR_WRITING_DEPTH {
		return ErrMaximumReprWritingDepthReached
	}

	if err := entry.Value.WriteRepresentation(ctx, w, config, depth+1); err != nil {
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

			err := child.WriteRepresentation(ctx, w, config, depth+1)
			if err != nil {
				return err
			}
		}
		w.Write([]byte{'}'})
	}
	return nil
}

func (c *StringConcatenation) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return Str(c.GetOrBuildString()).WriteRepresentation(ctx, w, config, depth)
}

func (c *BytesConcatenation) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (c Color) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (g *SystemGraph) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (e SystemGraphEvent) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (e SystemGraphEdge) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (v *DynamicValue) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (Error) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (*LifetimeJob) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (*SynchronousMessageHandler) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (f *InoxFunction) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (m *Mapping) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (n AstNode) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}

func (p *StructPattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	return ErrNotImplementedYet
}
