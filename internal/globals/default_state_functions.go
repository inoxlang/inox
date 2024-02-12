package internal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mod"

	"github.com/inoxlang/inox/internal/globals/inoxsh_ns"

	"github.com/inoxlang/inox/internal/core/permkind"

	"github.com/inoxlang/inox/internal/utils"
)

const BUFF_WRITER_SIZE = 100

func _get_current_tx(ctx *core.Context) *core.Transaction {
	return ctx.GetTx()
}

func __fprint(ctx *core.Context, out io.Writer, args ...core.Value) {
	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, BUFF_WRITER_SIZE)

	for i, e := range args {
		if i != 0 {
			buff.WriteRune(' ')
		}

		err := core.PrettyPrint(e, w, config.DEFAULT_PRETTY_PRINT_CONFIG.WithContext(ctx), 0, 0)
		if err != nil {
			panic(err)
		}
	}

	buff.WriteRune('\n')

	//TODO: strip ansi sequences without removing valid colors
	fmt.Fprint(out, buff.String())
}

func _print(ctx *core.Context, args ...core.Value) {
	out := ctx.GetClosestState().Out
	__fprint(ctx, out, args...)
}

func _fprint(ctx *core.Context, out core.Writable, args ...core.Value) {
	__fprint(ctx, out.Writer(), args...)
}

func _Error(ctx *core.Context, text core.String, args ...core.Serializable) core.Error {
	goErr := errors.New(string(text))
	if len(args) == 0 {
		return core.NewError(goErr, core.Nil)
	}
	if len(args) > 1 {
		panic(errors.New("at most two arguments were expected"))
	}

	return core.NewError(goErr, args[0])
}

func _typeof(ctx *core.Context, arg core.Value) core.Type {
	t := reflect.TypeOf(arg)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return core.Type{Type: t}
}

func _tostr(ctx *core.Context, arg core.Value) core.StringLike {
	switch a := arg.(type) {
	case core.Bool:
		if a {
			return core.String("true")
		}
		return core.String("false")
	case core.Integral:
		return core.String(core.Stringify(a, ctx))
	case core.StringLike:
		return a
	case *core.ByteSlice:
		return core.String(a.UnderlyingBytes()) //TODO: panic if invalid characters ?
	case *core.RuneSlice:
		return core.String(a.ElementsDoNotModify())
	case core.ResourceName:
		return core.String(a.ResourceName())
	default:
		panic(fmt.Errorf("cannot convert value of type %T to a string-like value", a))
	}
}

func _tostring(ctx *core.Context, arg core.Value) core.String {
	switch a := arg.(type) {
	case core.Bool:
		if a {
			return core.String("true")
		}
		return core.String("false")
	case core.Integral:
		return core.String(core.Stringify(a, ctx))
	case core.StringLike:
		return core.String(a.GetOrBuildString())
	case *core.ByteSlice:
		return core.String(a.UnderlyingBytes()) //TODO: panic if invalid characters ?
	case *core.RuneSlice:
		return core.String(a.ElementsDoNotModify())
	case core.ResourceName:
		return core.String(a.ResourceName())
	default:
		panic(fmt.Errorf("cannot convert value of type %T to a string value", a))
	}
}

func _torune(ctx *core.Context, i core.Integral) core.Rune {
	n := i.Int64()
	if n < 0 {
		panic(fmt.Errorf("cannot convert to a rune a negative integer obtained from an integral value"))
	}
	// TODO: panic if if larger than maximum unicode point ?
	return core.Rune(n)
}

func _tobyte(ctx *core.Context, i core.Int) core.Byte {
	return core.Byte(i)
}

func _tofloat(ctx *core.Context, v core.Integral) core.Float {
	// TODO: panic if loss ?
	return core.Float(v.Int64())
}

func _toint(ctx *core.Context, v core.Value) core.Int {
	switch val := v.(type) {
	case core.Integral:
		return core.Int(val.Int64())
	case core.Float:
		f := val
		n := core.Int(f)
		if core.Float(n) != f {
			panic(core.ErrPrecisionLoss)
		}
		return n
	default:
		panic(core.ErrUnreachable)
	}
}

func _tobytecount(ctx *core.Context, v core.Int) core.ByteCount {
	if v < 0 {
		panic(fmt.Errorf("negative value %d", v))
	}
	return core.ByteCount(v)
}

func _torstream(ctx *core.Context, v core.Value) core.ReadableStream {
	return core.ToReadableStream(ctx, v, core.ANYVAL_PATTERN)
}

func _parse(ctx *core.Context, r core.Readable, p core.Pattern) (core.Value, error) {
	bytes, err := r.Reader().ReadAll()
	if err != nil {
		return nil, err
	}
	strPatt, ok := p.StringPattern()
	if !ok {
		return nil, errors.New("failed to parse: passed pattern has no associated string pattern")
	}

	return strPatt.Parse(ctx, utils.BytesAsString(bytes.UnderlyingBytes()))
}

func _split(ctx *core.Context, r core.Readable, sep core.String, p *core.OptionalParam[core.Pattern]) (core.Value, error) {
	bytes, err := r.Reader().ReadAll()
	if err != nil {
		return nil, err
	}

	var strPatt core.StringPattern
	if p != nil {
		var ok bool
		strPatt, ok = p.Value.(core.StringPattern)

		if !ok {
			strPatt, ok = p.Value.StringPattern()
		}

		if !ok {
			return nil, errors.New("passed pattern has no associated string pattern")
		}
	}

	substrings := strings.Split(string(bytes.UnderlyingBytes()), string(sep))
	values := make([]core.Serializable, len(substrings))

	if strPatt != nil {
		for i, substring := range substrings {
			v, err := strPatt.Parse(ctx, substring)
			if err != nil {
				return nil, fmt.Errorf("failed to parse one of the substring: %w", err)
			}
			values[i] = v
		}
	} else {
		for i, substring := range substrings {
			values[i] = core.String(substring)
		}
	}

	return core.NewWrappedValueList(values...), nil
}

func _idt(ctx *core.Context, v core.Value) core.Value {
	return v
}

func _len(ctx *core.Context, v core.Indexable) core.Int {
	return core.Int(v.Len())
}

func _len_range(ctx *core.Context, p core.StringPattern) core.IntRange {
	return p.LengthRange()
}

func _is_mutable(ctx *core.Context, v core.Value) core.Bool {
	return core.Bool(v.IsMutable())
}

func _mkbytes(ctx *core.Context, size core.ByteCount) *core.ByteSlice {
	return core.NewMutableByteSlice(make([]byte, size), "")
}

func _Runes(ctx *core.Context, v core.Readable) *core.RuneSlice {
	r := v.Reader()
	var b []byte

	if !r.AlreadyHasAllData() {
		bytes, err := v.Reader().ReadAll()
		if err != nil {
			panic(err)
		}
		b = bytes.UnderlyingBytes()
	} else {
		b = r.GetBytesDataToNotModify()
	}

	//TODO: check that all runes are valid ?

	return core.NewRuneSlice([]rune(utils.BytesAsString(b)))
}

func _EmailAddress(ctx *core.Context, s core.StringLike) core.EmailAddress {
	return utils.Must(core.NormalizeEmailAddress(s.GetOrBuildString()))
}

func _ULID(ctx *core.Context, s core.StringLike) core.ULID {
	return utils.Must(core.ParseULID(s.GetOrBuildString()))
}

func _UUIDV4(ctx *core.Context, s core.StringLike) core.UUIDv4 {
	return utils.Must(core.ParseUUIDv4(s.GetOrBuildString()))
}

func _UUIDv4(ctx *core.Context, s core.StringLike) core.UUIDv4 {
	return utils.Must(core.ParseUUIDv4(s.GetOrBuildString()))
}

func _Bytes(ctx *core.Context, v core.Readable) *core.ByteSlice {
	r := v.Reader()
	var b []byte

	if !r.AlreadyHasAllData() {
		bytes, err := v.Reader().ReadAll()
		if err != nil {
			panic(err)
		}
		b = bytes.UnderlyingBytes()
	} else {
		b = slices.Clone(r.GetBytesDataToNotModify())
	}

	return core.NewByteSlice(b, true, "")
}

func _Reader(_ *core.Context, v core.Readable) *core.Reader {
	return v.Reader()
}

func _dynimport(ctx *core.Context, src core.Value, argObj *core.Object, manifestObj *core.Object, options ...core.Value) (*core.LThread, error) {
	insecure := false
	var timeout time.Duration

	state := ctx.GetClosestState()

	for _, arg := range options {
		if opt, ok := arg.(core.Option); ok {
			switch opt {
			case core.Option{Name: "insecure", Value: core.True}:
				insecure = true
				continue
			default:
				switch opt.Name {
				case "timeout":
					timeout = time.Duration(opt.Value.(core.Duration))
					continue
				}
			}
		}
		return nil, errors.New("invalid options")
	}
	return core.ImportModule(core.ImportConfig{
		Src:                src.(core.ResourceName),
		ArgObj:             argObj,
		GrantedPermListing: manifestObj,
		ParentState:        state,
		Insecure:           insecure,
		Timeout:            timeout,
	})
}

func _run(ctx *core.Context, src core.Path, args ...core.Value) error {
	closestState := ctx.GetClosestState()

	_, _, _, _, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
		Fpath:                     string(src),
		ParsingCompilationContext: ctx,
		ParentContext:             ctx,
		ParentContextRequired:     true,

		Out: closestState.Out,
	})
	return err
}

func _is_space(r core.Rune) core.Bool {
	return core.Bool(unicode.IsSpace(rune(r)))
}

func _is_even(i core.Int) core.Bool {
	return core.Bool(i%2 == 0)
}

func _is_odd(i core.Int) core.Bool {
	return core.Bool(i%2 == 1)
}

func _url_of(ctx *core.Context, v core.Value) core.URL {
	return utils.Must(core.UrlOf(ctx, v))
}

func _cancel_exec(ctx *core.Context) {
	ctx.CancelGracefully()
}

func _List(ctx *core.Context, args ...core.Value) *core.List {
	var elements []core.Serializable

	for _, arg := range args {
		switch a := arg.(type) {
		case core.Indexable:
			if elements != nil {
				panic(commonfmt.FmtErrArgumentProvidedAtLeastTwice("elements"))
			}
			length := a.Len()
			elements = make([]core.Serializable, length)
			for i := 0; i < length; i++ {
				elements[i] = a.At(ctx, i).(core.Serializable)
			}
		case core.Iterable:
			if elements != nil {
				panic(commonfmt.FmtErrArgumentProvidedAtLeastTwice("elements"))
			}
			it := a.Iterator(ctx, core.IteratorConfiguration{})
			for it.Next(ctx) {
				elem := it.Value(ctx)
				elements = append(elements, elem.(core.Serializable))
			}
		default:
			panic(core.FmtErrInvalidArgument(a))
		}
	}
	return core.NewWrappedValueListFrom(elements)
}

func _Event(ctx *core.Context, value core.Value) *core.Event {
	return core.NewEvent(nil, value, core.DateTime(time.Now()))
}

func _Color(ctx *core.Context, firstArg core.Value, other ...core.Value) core.Color {
	switch len(other) {
	case 0:
		if ident, ok := firstArg.(core.Identifier); ok && strings.HasPrefix(string(ident), "ansi-") {
			name := ident[len("ansi-"):]
			color, ok := inoxsh_ns.COLOR_NAME_TO_COLOR[name]
			if ok {
				return core.ColorFromTermenvColor(color)
			}
		}
		panic(core.FmtErrInvalidArgumentAtPos(firstArg, 0))
	default:
		panic(errors.New("invalid number of arguments"))
	}
}

func _add_ctx_data(ctx *core.Context, path core.Path, value core.Value) {
	ctx.PutUserData(path, value)
}

func _ctx_data(ctx *core.Context, path core.Path, pattern *core.OptionalParam[core.Pattern]) core.Value {
	data := ctx.ResolveUserData(path)
	if data == nil {
		data = core.Nil
	}
	if pattern != nil && !pattern.Value.Test(ctx, data) {
		panic(fmt.Errorf("the value of the user data entry %q does not match the provided pattern", path.UnderlyingString()))
	}
	return data
}

func _get_system_graph(ctx *core.Context) (*core.SystemGraph, core.Bool) {
	perm := core.SystemGraphAccessPermission{
		Kind_: permkind.Read,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		panic(err)
	}

	g := ctx.GetClosestState().SystemGraph
	return g, g != nil
}

func _propnames(ctx *core.Context, val core.Value) *core.List {
	props := val.(core.IProps).PropertyNames(ctx)
	values := utils.MapSlice(props, func(s string) core.Serializable { return core.String(s) })
	return core.NewWrappedValueListFrom(values)
}
