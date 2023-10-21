package symbolic

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"runtime/debug"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
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

	v.PrettyPrint(buffered, config, depth, parentIndentCount)
	return buffered.Flush()
}
