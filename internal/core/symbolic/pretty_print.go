package symbolic

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"runtime/debug"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

const (
	prettyprint_BUFF_WRITER_SIZE = 100
)

// Stringify calls PrettyPrint on the passed value
func Stringify(v Value) string {
	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, prettyprint_BUFF_WRITER_SIZE)

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
		buffered = bufio.NewWriterSize(w, prettyprint_BUFF_WRITER_SIZE)
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

	prettyPrintWriter := pprint.NewWriter(buffered)
	prettyPrintWriter.Depth = depth
	prettyPrintWriter.ParentIndentCount = parentIndentCount

	v.PrettyPrint(prettyPrintWriter, config)
	return buffered.Flush()
}
