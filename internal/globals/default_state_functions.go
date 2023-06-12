package internal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/config"
	core "github.com/inoxlang/inox/internal/core"

	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/globals/inoxsh_ns"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"

	"github.com/inoxlang/inox/internal/utils"
)

const BUFF_WRITER_SIZE = 100

func _get_current_tx(ctx *core.Context) *core.Transaction {
	return ctx.GetTx()
}

func _clone_val(ctx *core.Context, arg core.Value) core.Value {
	return utils.Must(arg.Clone(map[uintptr]map[int]core.Value{}))
}

func _logvals(ctx *core.Context, args ...core.Value) {
	buff := bytes.NewBuffer(nil)
	for _, arg := range args {
		buff.WriteString(fmt.Sprintf("%#v", arg))
	}

	s := utils.StripANSISequences(buff.String())
	ctx.GetClosestState().Logger.Print(s)
}

func _log(ctx *core.Context, args ...core.Value) {
	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, BUFF_WRITER_SIZE)

	for i, e := range args {
		if i != 0 {
			buff.WriteRune(' ')
		}

		err := core.PrettyPrint(e, w, config.DEFAULT_LOG_PRINT_CONFIG.WithContext(ctx), 0, 0)
		if err != nil {
			panic(err)
		}
	}

	buff.WriteRune('\n')
	s := utils.StripANSISequences(buff.String())

	ctx.GetClosestState().Logger.Print(s)
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

func _printvals(ctx *core.Context, args ...core.Value) {
	buff := bytes.NewBuffer(nil)
	for _, arg := range args {
		buff.WriteString(fmt.Sprintf("%#v", arg))
	}

	out := ctx.GetClosestState().Out
	fmt.Fprintln(out, utils.StripANSISequences(buff.String()))
}

func _stringify_ast(ctx *core.Context, arg core.AstNode) core.Str {
	buf := bytes.Buffer{}
	_, err := parse.Print(arg.Node, &buf, parse.PrintConfig{TrimStart: true})
	if err != nil {
		panic(err)
	}
	return core.Str(buf.String())
}

func _Error(ctx *core.Context, text core.Str, args ...core.Value) core.Error {
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
			return core.Str("true")
		}
		return core.Str("false")
	case core.Integral:
		return core.Str(core.Stringify(a, ctx))
	case core.StringLike:
		return a
	case *core.ByteSlice:
		return core.Str(a.Bytes) //TODO: panic if invalid characters ?
	case *core.RuneSlice:
		return core.Str(a.ElementsDoNotModify())
	case core.ResourceName:
		return core.Str(a.ResourceName())
	default:
		panic(fmt.Errorf("cannot convert value of type %T to string", a))
	}
}

func _torune(ctx *core.Context, i core.Integral) core.Rune {
	n, _ := i.Int64()
	// TODO: panic if if larger than maximum unicode point ?
	return core.Rune(n)
}

func _tobyte(ctx *core.Context, i core.Int) core.Byte {
	return core.Byte(i)
}

func _tofloat(ctx *core.Context, v core.Int) core.Float {
	// TODO: panic if loss ?
	return core.Float(v)
}

func _toint(ctx *core.Context, v core.Value) core.Int {
	switch val := v.(type) {
	case core.Byte:
		return core.Int(val)
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

func _torstream(ctx *core.Context, v core.Value) core.ReadableStream {
	return core.ToReadableStream(ctx, v, core.ANYVAL_PATTERN)
}

func _repr(ctx *core.Context, v core.Value) core.Str {
	return core.Str(core.GetRepresentation(v, ctx))
}

func _parse_repr(ctx *core.Context, r core.Readable) (core.Value, error) {
	bytes, err := r.Reader().ReadAll()
	if err != nil {
		return nil, err
	}
	return core.ParseRepr(ctx, bytes.Bytes)
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

	return strPatt.Parse(ctx, utils.BytesAsString(bytes.Bytes))
}

func _split(ctx *core.Context, r core.Readable, sep core.Str, p core.Pattern) (core.Value, error) {
	bytes, err := r.Reader().ReadAll()
	if err != nil {
		return nil, err
	}

	strPatt, ok := p.(core.StringPattern)

	if !ok {
		strPatt, ok = p.StringPattern()
	}

	if !ok {
		return nil, errors.New("failed to parse: passed pattern has no associated string pattern")
	}

	substrings := strings.Split(utils.BytesAsString(bytes.Bytes), string(sep))
	values := make([]core.Value, len(substrings))
	for i, substring := range substrings {
		v, err := strPatt.Parse(ctx, substring)
		if err != nil {
			return nil, fmt.Errorf("failed to parse one of the substring: %w", err)
		}
		values[i] = v
	}

	return core.NewWrappedValueList(values...), nil
}

func _idt(ctx *core.Context, v core.Value) core.Value {
	return v
}

func _len(ctx *core.Context, v core.Value) core.Int {
	return core.Int(v.(core.Indexable).Len())
}

func _len_range(ctx *core.Context, p core.StringPattern) core.IntRange {
	return p.LengthRange()
}

func _mkbytes(ctx *core.Context, size core.Int) *core.ByteSlice {
	return &core.ByteSlice{Bytes: make([]byte, size), IsDataMutable: true}
}

func _Runes(ctx *core.Context, v core.Readable) *core.RuneSlice {
	r := v.Reader()
	var b []byte

	if !r.AlreadyHasAllData() {
		bytes, err := v.Reader().ReadAll()
		if err != nil {
			panic(err)
		}
		b = bytes.Bytes
	} else {
		b = r.GetBytesDataToNotModify()
	}

	//TODO: check that all runes are valid ?

	return core.NewRuneSlice([]rune(utils.BytesAsString(b)))
}

func _Bytes(ctx *core.Context, v core.Readable) *core.ByteSlice {
	r := v.Reader()
	var b []byte

	if !r.AlreadyHasAllData() {
		bytes, err := v.Reader().ReadAll()
		if err != nil {
			panic(err)
		}
		b = bytes.Bytes
	} else {
		b = utils.CopySlice(r.GetBytesDataToNotModify())
	}

	return core.NewByteSlice(b, true, "")
}

func _Reader(_ *core.Context, v core.Readable) *core.Reader {
	return v.Reader()
}

func _dynimport(ctx *core.Context, src core.Value, argObj *core.Object, manifestObj *core.Object, options ...core.Value) (*core.Routine, error) {
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
		Src:                src,
		ArgObj:             argObj,
		GrantedPermListing: manifestObj,
		ParentState:        state,
		Insecure:           insecure,
		Timeout:            timeout,
	})
}

func _run(ctx *core.Context, src core.Path, args ...core.Value) error {
	_, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
		Fpath:                     string(src),
		ParsingCompilationContext: ctx,
		ParentContext:             ctx,
		UseContextAsParent:        true,

		Out: ctx.GetClosestState().Out,
	})
	return err
}

func _is_rune_space(r core.Rune) core.Bool {
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
	ctx.Cancel()
}

func _List(ctx *core.Context, args ...core.Value) *core.List {
	var elements []core.Value

	for _, arg := range args {
		switch a := arg.(type) {
		case core.Indexable:
			if elements != nil {
				panic(commonfmt.FmtErrArgumentProvidedAtLeastTwice("elements"))
			}
			length := a.Len()
			elements = make([]core.Value, length)
			for i := 0; i < length; i++ {
				elements[i] = a.At(ctx, i)
			}
		case core.Iterable:
			if elements != nil {
				panic(commonfmt.FmtErrArgumentProvidedAtLeastTwice("elements"))
			}
			it := a.Iterator(ctx, core.IteratorConfiguration{})
			for it.Next(ctx) {
				elem := it.Value(ctx)
				elements = append(elements, elem)
			}
		default:
			panic(core.FmtErrInvalidArgument(a))
		}
	}
	return core.NewWrappedValueListFrom(elements)
}

func _Event(ctx *core.Context, value core.Value) *core.Event {
	return core.NewEvent(value, core.Date(time.Now()))
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

func _FileMode(ctx *core.Context, firstArg core.Value) core.FileMode {
	integer, ok := firstArg.(core.Int)
	if !ok {
		panic(commonfmt.FmtErrInvalidArgumentAtPos(0, "should be an integer"))
	}

	return core.FileMode(integer)
}

func _add_ctx_data(ctx *core.Context, name core.Identifier, value core.Value) {
	ctx.AddUserData(name, value)
}

func _ctx_data(ctx *core.Context, name core.Identifier) core.Value {
	return ctx.ResolveUserData(name)
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
	values := utils.MapSlice(props, func(s string) core.Value { return core.Str(s) })
	return core.NewWrappedValueListFrom(values)
}
