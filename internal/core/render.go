package core

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	customRenderers = map[reflect.Type]RenderingFn{}
)

var (
	ErrNotRenderable                  = errors.New("value is not renderable")
	ErrInvalidRenderingConfig         = errors.New("invalid rendering configuration")
	ErrNotRenderableUseCustomRenderer = fmt.Errorf("%w: use a custom renderer", ErrNotRenderable)

	LIST_OPENING_TAG = []byte{'<', 'u', 'l', '>'}
	LIST_CLOSING_TAG = []byte{'<', '/', 'u', 'l', '>'}

	DIV_OPENING_TAG = []byte{'<', 'd', 'i', 'v', '>'}
	DIV_CLOSING_TAG = []byte{'<', '/', 'd', 'i', 'v', '>'}

	TIME_OPENING_TAG = []byte{'<', 't', 'i', 'm', 'e', '>'}
	TIME_CLOSING_TAG = []byte{'<', '/', 't', 'i', 'm', 'e', '>'}

	S_TRUE  = []byte{'t', 'r', 'u', 'e'}
	S_FALSE = []byte{'f', 'a', 'l', 's', 'e'}

	_ = []Renderable{Bool(true), Int(0), String(""), &List{}, &Object{}, &ValueHistory{}}
)

// A renderable is a Value that can be rendered to at least one MIME type.
type Renderable interface {
	Value
	IsRecursivelyRenderable(ctx *Context, config RenderingInput) bool
	Render(ctx *Context, w io.Writer, config RenderingInput) (int, error)

	// possible issues: value is changed by other goroutine after call to IsRecursivelyRenderable
}

type RenderingFn func(ctx *Context, w io.Writer, renderable Renderable, config RenderingInput) (int, error)

// RegisterRenderer register a custom rendering function for a given type,
// this function should ONLY be called during the initialization phase (calls to init()) since it is not protected by a lock
func RegisterRenderer(t reflect.Type, fn RenderingFn) {
	if _, ok := customRenderers[t]; ok {
		panic(fmt.Errorf("custom renderer already provided for type %s", t.Name()))
	}
	customRenderers[t] = fn
}

type RenderingInput struct {
	Mime               Mimetype
	OptionalUserConfig Value
}

type NotRenderableMixin struct {
}

func (m NotRenderableMixin) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	return false
}

func (m NotRenderableMixin) Render(ctx *Context, w io.Writer, config RenderingInput) (int, error) {
	return 0, ErrNotRenderable
}

// Renders renders the renderable with a custom renderer if registered, otherwise it calls renderable.Render.
func Render(ctx *Context, w io.Writer, renderable Renderable, config RenderingInput) (int, error) {
	customRenderFn, ok := customRenderers[reflect.TypeOf(renderable)]
	if ok {
		return customRenderFn(ctx, w, renderable, config)
	}

	return renderable.Render(ctx, w, config)
}

func render(ctx *Context, v Renderable, mime Mimetype) string {
	buf := bytes.NewBuffer(nil)
	v.Render(ctx, buf, RenderingInput{
		Mime: mime,
	})
	return buf.String()
}

// ------- implementation of IsRecursivelyRenderable & Render for some core types --------

func (b Bool) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	return true
}

func (b Bool) Render(ctx *Context, w io.Writer, config RenderingInput) (int, error) {
	switch config.Mime {
	case mimeconsts.HTML_CTYPE:
		if b {
			return w.Write(S_TRUE)
		} else {
			return w.Write(S_FALSE)
		}
	default:
		return 0, formatErrUnsupportedRenderingMime(config.Mime)
	}
}
func (s String) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	return true
}

func (s String) Render(ctx *Context, w io.Writer, config RenderingInput) (int, error) {
	switch config.Mime {
	case mimeconsts.HTML_CTYPE:
		escaped := html.EscapeString(string(s))
		return w.Write([]byte(escaped))
	default:
		return 0, formatErrUnsupportedRenderingMime(config.Mime)
	}
}

func (s *RuneSlice) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	return true
}

func (s *RuneSlice) Render(ctx *Context, w io.Writer, config RenderingInput) (int, error) {
	switch config.Mime.WithoutParams() {
	case mimeconsts.HTML_CTYPE:
		escaped := html.EscapeString(string(s.elements))
		return w.Write([]byte("<span>" + escaped + "</span>"))
	default:
		return 0, formatErrUnsupportedRenderingMime(config.Mime)
	}
}

func (n Int) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	return true
}

func (n Int) Render(ctx *Context, w io.Writer, config RenderingInput) (int, error) {
	switch config.Mime {
	case mimeconsts.HTML_CTYPE:
		return w.Write([]byte(strconv.FormatInt(int64(n), 10)))
	default:
		return 0, formatErrUnsupportedRenderingMime(config.Mime)
	}
}

func (d DateTime) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	return true
}

func (d DateTime) Render(ctx *Context, w io.Writer, config RenderingInput) (int, error) {
	var format *DateFormat

	if config.OptionalUserConfig != nil {
		userConfig, ok := config.OptionalUserConfig.(*DateFormat)
		if !ok {
			return 0, ErrInvalidRenderingConfig
		}
		format = userConfig
	}

	switch config.Mime {
	case mimeconsts.HTML_CTYPE:
		totalN, err := w.Write(TIME_OPENING_TAG)
		if err != nil {
			return totalN, err
		}
		var n int

		if format == nil {
			s := time.Time(d).Format(time.UnixDate)
			n, err = w.Write(utils.StringAsBytes(s))
		} else {
			n, err = format.Format(ctx, d, w)
		}
		totalN += n
		if err != nil {
			return totalN, err
		}

		n, err = w.Write(TIME_CLOSING_TAG)
		totalN += n
		return totalN, err
	default:
		return 0, formatErrUnsupportedRenderingMime(config.Mime)
	}
}

func (list *List) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	length := list.Len()
	for i := 0; i < length; i++ {
		_, ok := list.At(ctx, i).(Renderable)
		if !ok {
			return false
		}
	}
	return true
}

func (list *List) Render(ctx *Context, w io.Writer, config RenderingInput) (int, error) {
	if !list.IsRecursivelyRenderable(ctx, config) {
		return 0, ErrNotRenderable
	}

	length := list.Len()

	switch config.Mime {
	case mimeconsts.HTML_CTYPE:
		totalN, err := w.Write(LIST_OPENING_TAG)
		if err != nil {
			return totalN, err
		}

		for i := 0; i < length; i++ {
			elem := list.At(ctx, i).(Renderable)
			n, err := elem.Render(ctx, w, config)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		n, err := w.Write(LIST_CLOSING_TAG)
		totalN += n
		return totalN, err
	default:
		return 0, formatErrUnsupportedRenderingMime(config.Mime)
	}
}

func (obj *Object) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	closestState := ctx.MustGetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)
	for _, v := range obj.values {
		_, ok := v.(Renderable)
		if !ok {
			return false
		}
	}
	return true
}

func (obj *Object) Render(ctx *Context, w io.Writer, config RenderingInput) (int, error) {
	if !obj.IsRecursivelyRenderable(ctx, config) {
		return 0, ErrNotRenderable
	}

	closestState := ctx.MustGetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)
	switch config.Mime {
	case mimeconsts.HTML_CTYPE:
		totalN, err := w.Write(DIV_OPENING_TAG)
		if err != nil {
			return totalN, err
		}

		keys := obj.Keys(nil)
		sort.Strings(keys)

		for i, k := range keys {
			isIndexKey := IsIndexKey(k)
			propVal := obj.values[i]

			_ = isIndexKey

			n, err := w.Write(DIV_OPENING_TAG)
			totalN += n
			if err != nil {
				return totalN, err
			}

			n, err = propVal.(Renderable).Render(ctx, w, config)
			totalN += n
			if err != nil {
				return totalN, err
			}

			n, err = w.Write(DIV_CLOSING_TAG)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		n, err := w.Write(DIV_CLOSING_TAG)
		totalN += n
		return totalN, err
	default:
		return 0, formatErrUnsupportedRenderingMime(config.Mime)
	}
}

func (node AstNode) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	return true
}

func (node AstNode) Render(ctx *Context, w io.Writer, config RenderingInput) (n int, finalErr error) {
	if !node.IsRecursivelyRenderable(ctx, config) {
		return 0, ErrNotRenderable
	}

	if config.Mime != mimeconsts.HTML_CTYPE {
		return 0, formatErrUnsupportedRenderingMime(config.Mime)
	}

	defer func() {
		recoveredErr, ok := recover().(error)
		if ok {
			finalErr = fmt.Errorf("%w: %s", recoveredErr, string(debug.Stack()))
		}
	}()

	// check user config

	var codeErrors []Error

	if config.OptionalUserConfig != nil {
		object, ok := config.OptionalUserConfig.(*Object)
		if !ok {
			return 0, fmt.Errorf("%w: an object was expected but a value of type %T was provided", ErrInvalidRenderingConfig, config.OptionalUserConfig)
		}

		if object.HasProp(ctx, "errors") {
			val := Unwrap(ctx, object.Prop(ctx, "errors"))
			tuple, ok := val.(*Tuple)

			if !ok {
				return 0, fmt.Errorf("%w: .errors should be a tuple not a(n) %T", ErrInvalidRenderingConfig, val)
			}

			if ok {
				for _, e := range tuple.elements {
					if err, isErr := e.(Error); isErr {
						codeErrors = append(codeErrors, err)
					} else {
						return 0, fmt.Errorf(
							"%w: .errors should be a tuple that only contains errors: unexpected element %T", ErrInvalidRenderingConfig, e,
						)
					}
				}
			}

		}
	}

	//

	tokens := parse.GetTokens(node.Node, node.Chunk_.Node, true)

	//we iterate over the tokens a first time to known the line spans
	bw := WrapWriter(w, true, nil)
	lineSpans := []parse.NodeSpan{{Start: 0, End: 0}}

	trailingSpaceStart := 0
	trailingSpace := 0

	if len(tokens) > 0 {
		lineIndex := 0
		for _, token := range tokens {
			if token.Type == parse.NEWLINE {
				lineIndex++
				lineSpans[lineIndex-1].End = token.Span.Start
				lineSpans = append(lineSpans, parse.NodeSpan{Start: token.Span.Start, End: 0})
			}
		}
		lineSpans[lineIndex].End = node.Node.Base().Span.End

		lastToken := tokens[len(tokens)-1]
		trailingSpaceStart = int(lastToken.Span.End)
		trailingSpace = int(node.Node.Base().Span.End - lastToken.Span.End)
	}

	// print opening tags for container
	if _, err := bw.WriteString("<div class='code-chunk'>"); err != nil {
		finalErr = err
		return
	}

	// print all code errors

	positionDataPattern := NewInexactRecordPattern([]RecordPatternEntry{
		{Name: "line", Pattern: INT_PATTERN},
		{Name: "column", Pattern: INT_PATTERN},
	})

	positionStackPattern := NewInexactRecordPattern([]RecordPatternEntry{
		{Name: "position-stack", Pattern: NewTuplePatternOf(positionDataPattern)},
	})

	for _, codeError := range codeErrors {
		text := html.EscapeString(codeError.Text())

		data := codeError.Data()
		tagEnd := ">"

		if positionDataPattern.Test(ctx, data) {
			iprops := data.(IProps)
			tagEnd = fmt.Sprintf(" data-line='%d' data-column='%d' data-span='%d,%d'>",
				iprops.Prop(ctx, "line").(Int), iprops.Prop(ctx, "column").(Int), iprops.Prop(ctx, "start").(Int), iprops.Prop(ctx, "end").(Int))
		} else if positionStackPattern.Test(ctx, data) {
			positions := data.(IProps).Prop(ctx, "position-stack").(*Tuple).elements
			lastPosition := positions[len(positions)-1]
			iprops := lastPosition.(IProps)

			tagEnd = fmt.Sprintf(" data-line='%d' data-column='%d' data-span='%d,%d'>",
				iprops.Prop(ctx, "line").(Int), iprops.Prop(ctx, "column").(Int), iprops.Prop(ctx, "start").(Int), iprops.Prop(ctx, "end").(Int))
		}

		if _, err := bw.WriteStrings("<span class='code-chunk__error'", tagEnd, text, "</span>"); err != nil {
			finalErr = err
			return
		}

	}

	lineCountDigits := int(math.Ceil(math.Log10(float64(len(lineSpans)))))

	// print opening tag for line list
	if _, err := bw.WriteStrings(
		"<ul contenteditable spellcheck='false' class='code-chunk__lines' style='--line-count-digits:", strconv.Itoa(lineCountDigits), "'>",
	); err != nil {
		finalErr = err
		return
	}

	// print opening tags for line
	if _, err := bw.WriteString(fmt.Sprintf("<li data-span='0,%d' data-n='1'>", lineSpans[0].End)); err != nil {
		finalErr = err
		return
	}

	defer func() {
		n = int(bw.TotalWritten())
		bw.Flush(ctx)
	}()

	if len(tokens) == 0 {
		if _, err := bw.WriteString("<div data-type='newline' class='token' data-span='0,1'> </div>"); err != nil {
			finalErr = err
			return
		}
	}

	writeSpace := func(spanData string, space int) error {
		_, err := bw.WriteStrings("<div class='space' data-span='", spanData, "'>", strings.Repeat(" ", space), "</div>")
		return err
	}

	prevEnd := 0
	tokensInLine := 0
	lineIndex := 0

	//we iterate over the tokens to print the HTML for tokens & lines

	for _, t := range tokens {
		spaceBetweenTokens := int(t.Span.Start) - prevEnd
		if spaceBetweenTokens > 0 {
			if err := writeSpace(fmt.Sprintf("%d,%d", prevEnd, t.Span.Start), spaceBetweenTokens); err != nil {
				finalErr = err
				return
			}
		}

		tokenSpanData := fmt.Sprintf("%d,%d", t.Span.Start, t.Span.End)
		tokenType := strings.ReplaceAll(strings.ToLower(t.Type.String()), "_", "-")

		if t.Type == parse.NEWLINE {
			lineIndex++

			// new line

			lineStart := lineSpans[lineIndex].Start
			lineEnd := lineSpans[lineIndex].End

			lineSpanData := fmt.Sprintf("%d,%d", lineStart, lineEnd)
			spaceIfEmpty := ""
			if lineEnd == lineStart+1 {
				spaceIfEmpty = " "
			}

			if _, err := bw.WriteStrings(
				" </li><li data-span='", lineSpanData, "' data-n='", strconv.Itoa(lineIndex+1), "'>",
				"<div class='token' data-type='", tokenType, "' data-span='", tokenSpanData, "'>", spaceIfEmpty, "</div>"); err != nil {
				finalErr = err
				return
			}
			prevEnd = int(t.Span.End)
			tokensInLine = 0

			continue
		} else {
			tokensInLine++
		}

		metaS := ""
		meta, metaCount := t.Meta.Strings()
		if metaCount > 0 {
			metaS = "data-tmeta='" + strings.Join(meta[:metaCount], ",") + "' "
		}

		if _, err := bw.WriteStrings("<div class='token' data-type='", tokenType, "' ", metaS, "data-span='", tokenSpanData, `'>`, t.Str(), "</div>"); err != nil {
			finalErr = err
			return
		}
		prevEnd = int(t.Span.End)
	}

	if trailingSpace > 0 {
		if err := writeSpace(fmt.Sprintf("%d,%d", trailingSpaceStart, trailingSpaceStart+trailingSpace), trailingSpace); err != nil {
			finalErr = err
			return
		}
	}

	if _, err := bw.WriteString("</li></ul></div>"); err != nil {
		finalErr = err
		return
	}

	return 0, nil
}

func (h *ValueHistory) IsRecursivelyRenderable(ctx *Context, input RenderingInput) bool {
	return true
}

func (h *ValueHistory) Render(ctx *Context, w io.Writer, config RenderingInput) (int, error) {
	return 0, ErrNotRenderableUseCustomRenderer
}

func formatErrUnsupportedRenderingMime(mime Mimetype) error {
	return fmt.Errorf("cannot render: mime %s is not supported", mime)
}
