package utils

import (
	"fmt"
	"io"
	"regexp"

	"github.com/muesli/termenv"
)

const (
	SMALL_LINE_SEP = "------------------------------"
)

var ANSI_ESCAPE_SEQUENCE_REGEX = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

func StripANSISequences(str string) string {
	return ANSI_ESCAPE_SEQUENCE_REGEX.ReplaceAllString(str, "")
}

func MoveCursorNextLine(writer io.Writer, n int) {
	if n == 0 {
		return
	}
	fmt.Fprintf(writer, termenv.CSI+termenv.CursorNextLineSeq, n)
}

func PrintSmallLineSeparator(w io.Writer) {
	fmt.Fprintln(w, SMALL_LINE_SEP)
}
