package prettyprint

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/muesli/termenv"
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
	writer *bufio.Writer

	Depth               int
	ParentIndentCount   int
	RemovePercentPrefix bool
}

func NewWriter(writer *bufio.Writer) PrettyPrintWriter {
	return PrettyPrintWriter{
		writer: writer,
	}
}

func (w PrettyPrintWriter) WriteName(str string) {
	if !w.RemovePercentPrefix {
		utils.PanicIfErr(w.writer.WriteByte('%'))
	}
	utils.Must(w.writer.Write(utils.StringAsBytes(str)))
}

func (w PrettyPrintWriter) WriteNameF(fmtStr string, args ...any) {
	if !w.RemovePercentPrefix {
		utils.PanicIfErr(w.writer.WriteByte('%'))
	}
	utils.Must(fmt.Fprintf(w.writer, fmtStr, args...))
}

func (w PrettyPrintWriter) WriteString(str string) {
	utils.Must(w.writer.Write(utils.StringAsBytes(str)))
}

func (w PrettyPrintWriter) WriteStringF(fmtStr string, args ...any) {
	utils.Must(fmt.Fprintf(w.writer, fmtStr, args...))
}

func (w PrettyPrintWriter) WriteBytes(b []byte) {
	utils.Must(w.writer.Write(b))
}

func (w PrettyPrintWriter) WriteManyBytes(b ...[]byte) {
	utils.MustWriteMany(w.writer, b...)
}

func (w PrettyPrintWriter) WriteLFCR() {
	utils.PanicIfErr(w.writer.WriteByte('\n'))
	utils.PanicIfErr(w.writer.WriteByte('\r'))
}

func (w PrettyPrintWriter) WriteAnsiReset() {
	utils.Must(w.writer.Write(ANSI_RESET_SEQUENCE))
}

func (w PrettyPrintWriter) WriteColonSpace() {
	utils.Must(w.writer.Write(COLON_SPACE))
}

func (w PrettyPrintWriter) WriteCommaSpace() {
	utils.Must(w.writer.Write(COMMA_SPACE))
}

func (w PrettyPrintWriter) WriteClosingBracketClosingParen() {
	utils.Must(w.writer.Write(CLOSING_CURLY_BRACKET_CLOSING_PAREN))
}

func (w PrettyPrintWriter) WriteClosingbracketClosingParen() {
	utils.Must(w.writer.Write(CLOSING_BRACKET_CLOSING_PAREN))
}

func (w PrettyPrintWriter) WriteDotOpeningCurlyBracket() {
	utils.Must(w.writer.Write(DOT_OPENING_CURLY_BRACKET))
}

func (w PrettyPrintWriter) WriteByte(b byte) {
	utils.PanicIfErr(w.writer.WriteByte(b))
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

func (w PrettyPrintWriter) EnterPattern() PrettyPrintWriter {
	new := w
	new.RemovePercentPrefix = true
	return new
}
