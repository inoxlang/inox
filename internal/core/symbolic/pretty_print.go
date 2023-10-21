package symbolic

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"runtime/debug"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/muesli/termenv"
)

const (
	PRETTY_PRINT_BUFF_WRITER_SIZE = 100
	MAX_VALUE_PRINT_DEPTH         = 10
)

var (
	ANSI_RESET_SEQUENCE = []byte(termenv.CSI + termenv.ResetSeq + "m")

	LF_CR                               = []byte{'\n', '\r'}
	DASH_DASH                           = []byte{'-', '-'}
	SHARP_OPENING_PAREN                 = []byte{'#', '('}
	COLON_SPACE                         = []byte{':', ' '}
	COMMA_SPACE                         = []byte{',', ' '}
	CLOSING_BRACKET_CLOSING_PAREN       = []byte{']', ')'}
	CLOSING_CURLY_BRACKET_CLOSING_PAREN = []byte{'}', ')'}
	THREE_DOTS                          = []byte{'.', '.', '.'}
	DOT_OPENING_CURLY_BRACKET           = []byte{'.', '{'}
)

type PrettyPrintWriter struct {
	Writer              *bufio.Writer
	Depth               int
	ParentIndentCount   int
	RemovePercentPrefix bool
}

func (w PrettyPrintWriter) WriteName(str string) {
	if !w.RemovePercentPrefix {
		utils.PanicIfErr(w.Writer.WriteByte('%'))
	}
	utils.Must(w.Writer.Write(utils.StringAsBytes(str)))
}

func (w PrettyPrintWriter) WriteNameF(fmtStr string, args ...any) {
	if !w.RemovePercentPrefix {
		utils.PanicIfErr(w.Writer.WriteByte('%'))
	}
	utils.Must(fmt.Fprintf(w.Writer, fmtStr, args...))
}

func (w PrettyPrintWriter) WriteString(str string) {
	utils.Must(w.Writer.Write(utils.StringAsBytes(str)))
}

func (w PrettyPrintWriter) WriteStringF(fmtStr string, args ...any) {
	utils.Must(fmt.Fprintf(w.Writer, fmtStr, args...))
}

func (w PrettyPrintWriter) WriteBytes(b []byte) {
	utils.Must(w.Writer.Write(b))
}

func (w PrettyPrintWriter) WriteManyBytes(b ...[]byte) {
	utils.MustWriteMany(w.Writer, b...)
}

func (w PrettyPrintWriter) WriteLFCR() {
	utils.PanicIfErr(w.Writer.WriteByte('\n'))
	utils.PanicIfErr(w.Writer.WriteByte('\r'))
}

func (w PrettyPrintWriter) WriteByte(b byte) {
	utils.PanicIfErr(w.Writer.WriteByte(b))
}

func (w PrettyPrintWriter) ZeroDepthIndent() PrettyPrintWriter {
	new := w
	new.Depth = 0
	new.ParentIndentCount = 0
	return new
}

func (w PrettyPrintWriter) ZeroDepth() PrettyPrintWriter {
	new := w
	new.Depth = 0
	return new
}

func (w PrettyPrintWriter) ZeroIndent() PrettyPrintWriter {
	new := w
	new.ParentIndentCount = 0
	return new
}

func (w PrettyPrintWriter) IncrDepthWithIndent(indentCount int) PrettyPrintWriter {
	new := w
	new.Depth++
	new.ParentIndentCount = indentCount
	return new
}

func (w PrettyPrintWriter) IncrDepth() PrettyPrintWriter {
	new := w
	new.Depth++
	return new
}

func (w PrettyPrintWriter) WithDepth(depth int) PrettyPrintWriter {
	new := w
	new.Depth = depth
	return new
}

func (w PrettyPrintWriter) WithDepthIndent(depth, indent int) PrettyPrintWriter {
	new := w
	new.Depth = depth
	new.ParentIndentCount = indent
	return new
}

// Stringify calls PrettyPrint on the passed value
func Stringify(v Value) string {
	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, PRETTY_PRINT_BUFF_WRITER_SIZE)

	err := PrettyPrint(v, w, &pprint.PrettyPrintConfig{
		MaxDepth: 7,
		Colorize: false,
		Compact:  true,
	}, 0, 0)

	if err != nil {
		panic(err)
	}

	w.Flush()
	return buff.String()
}

func PrettyPrint(v Value, w io.Writer, config *pprint.PrettyPrintConfig, depth, parentIndentCount int) (err error) {
	buffered, ok := w.(*bufio.Writer)
	if !ok {
		buffered = bufio.NewWriterSize(w, PRETTY_PRINT_BUFF_WRITER_SIZE)
	}

	defer func() {
		e := recover()
		switch v := e.(type) {
		case error:
			err = fmt.Errorf("%w %s", v, string(debug.Stack()))
		default:
			err = fmt.Errorf("panic: %#v %s", e, string(debug.Stack()))
		case nil:
		}
	}()

	prettyPrintWriter := PrettyPrintWriter{
		Writer:            buffered,
		Depth:             depth,
		ParentIndentCount: parentIndentCount,
	}
	v.PrettyPrint(prettyPrintWriter, config)
	return buffered.Flush()
}
