package prettyprint

import (
	"bufio"
	"fmt"
	"math"

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
	writer  *bufio.Writer
	written *int //written bytecount

	Depth               int
	ParentIndentCount   int
	RemovePercentPrefix bool

	regions     *[]Region //if nil regions are disabled
	regionDepth *int      //written bytecount
}

func NewWriter(writer *bufio.Writer, enableRegions bool) PrettyPrintWriter {
	w := PrettyPrintWriter{
		writer:  writer,
		written: utils.New(0),
	}

	if enableRegions {
		w.regionDepth = utils.New(0)
		w.regions = &[]Region{}
	}

	return w
}

func (w PrettyPrintWriter) WriteName(str string) {
	if !w.RemovePercentPrefix {
		utils.PanicIfErr(w.writer.WriteByte('%'))
		*w.written++
	}
	utils.Must(w.writer.Write(utils.StringAsBytes(str)))
	*w.written += len(str)
}

func (w PrettyPrintWriter) WriteNameF(fmtStr string, args ...any) {
	if !w.RemovePercentPrefix {
		utils.PanicIfErr(w.writer.WriteByte('%'))
		*w.written++
	}
	n := utils.Must(fmt.Fprintf(w.writer, fmtStr, args...))
	*w.written += n
}

func (w PrettyPrintWriter) WriteString(str string) {
	utils.Must(w.writer.Write(utils.StringAsBytes(str)))
	*w.written += len(str)
}

func (w PrettyPrintWriter) WriteStringF(fmtStr string, args ...any) {
	n := utils.Must(fmt.Fprintf(w.writer, fmtStr, args...))
	*w.written += n
}

func (w PrettyPrintWriter) WriteBytes(b []byte) {
	utils.Must(w.writer.Write(b))
	*w.written += len(b)
}

func (w PrettyPrintWriter) WriteManyBytes(b ...[]byte) {
	utils.MustWriteMany(w.writer, b...)
	for _, slice := range b {
		*w.written += len(slice)
	}
}

func (w PrettyPrintWriter) WriteLFCR() {
	utils.PanicIfErr(w.writer.WriteByte('\n'))
	*w.written++

	utils.PanicIfErr(w.writer.WriteByte('\r'))
	*w.written++
}

func (w PrettyPrintWriter) WriteAnsiReset() {
	n := utils.Must(w.writer.Write(ANSI_RESET_SEQUENCE))
	*w.written += n
}

func (w PrettyPrintWriter) WriteColonSpace() {
	n := utils.Must(w.writer.Write(COLON_SPACE))
	*w.written += n
}

func (w PrettyPrintWriter) WriteCommaSpace() {
	n := utils.Must(w.writer.Write(COMMA_SPACE))
	*w.written += n
}

func (w PrettyPrintWriter) WriteClosingBracketClosingParen() {
	n := utils.Must(w.writer.Write(CLOSING_CURLY_BRACKET_CLOSING_PAREN))
	*w.written += n
}

func (w PrettyPrintWriter) WriteClosingbracketClosingParen() {
	n := utils.Must(w.writer.Write(CLOSING_BRACKET_CLOSING_PAREN))
	*w.written += n
}

func (w PrettyPrintWriter) WriteDotOpeningCurlyBracket() {
	n := utils.Must(w.writer.Write(DOT_OPENING_CURLY_BRACKET))
	*w.written += n
}

func (w PrettyPrintWriter) WriteByte(b byte) {
	utils.PanicIfErr(w.writer.WriteByte(b))
	*w.written++
}

func (w PrettyPrintWriter) AreRegionsDisabled() bool {
	return w.regionDepth == nil
}
func (w PrettyPrintWriter) EnterRegion(kind RegionKind) Region {
	if w.AreRegionsDisabled() {
		return Region{}
	}

	if w.Depth < 0 || w.Depth > math.MaxUint8 {
		panic(fmt.Errorf("invalid depth: %d", w.Depth))
	}

	if *w.written < 0 || *w.written > math.MaxInt16 {
		panic(fmt.Errorf("invalid written bytecount: %d", *w.written))
	}

	*w.regionDepth++

	region := Region{
		Kind:  kind,
		Depth: uint8(w.Depth),
		Start: uint16(*w.written),
	}

	return region
}

func (w PrettyPrintWriter) LeaveRegion(region Region) {
	if w.AreRegionsDisabled() {
		return
	}

	if *w.written < 0 || *w.written > math.MaxInt16 {
		panic(fmt.Errorf("invalid written bytecount: %d", *w.written))
	}

	*w.regionDepth--
	region.End = uint16(*w.written)
	*w.regions = append(*w.regions, region)
}

// GetRegions returns the list of regions, this functions should be called after all writes.
func (w *PrettyPrintWriter) Regions() []Region {
	if w.AreRegionsDisabled() {
		return nil
	}

	regions := *w.regions
	w.regions = nil
	return regions
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
