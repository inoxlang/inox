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
	PRETTY_PRINT_BUFF_WRITER_SIZE = 100
)

var (
	STRINGIFY_PRETTY_PRINT_CONFIG = &pprint.PrettyPrintConfig{
		MaxDepth: 7,
		Colorize: false,
		Compact:  true,
	}
)

// Stringify calls PrettyPrint on the passed value
func Stringify(v Value) string {
	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, PRETTY_PRINT_BUFF_WRITER_SIZE)

	_, err := PrettyPrint(PrettyPrintArgs{
		Value:             v,
		Writer:            w,
		Config:            STRINGIFY_PRETTY_PRINT_CONFIG,
		Depth:             0,
		ParentIndentCount: 0,
	})

	if err != nil {
		panic(err)
	}

	w.Flush()
	return buff.String()
}

func StringifyGetRegions(v Value) (string, pprint.Regions) {
	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, PRETTY_PRINT_BUFF_WRITER_SIZE)

	regions, err := PrettyPrint(PrettyPrintArgs{
		Value:             v,
		Writer:            w,
		Config:            STRINGIFY_PRETTY_PRINT_CONFIG,
		Depth:             0,
		ParentIndentCount: 0,
		EnableRegions:     true,
	})

	if err != nil {
		panic(err)
	}

	w.Flush()
	return buff.String(), regions
}

type PrettyPrintArgs struct {
	Value                    Value
	Writer                   io.Writer
	Config                   *pprint.PrettyPrintConfig
	Depth, ParentIndentCount int
	EnableRegions            bool //if false the returned region list is empty
}

func PrettyPrint(args PrettyPrintArgs) (regions pprint.Regions, err error) {
	v := args.Value
	w := args.Writer

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

	prettyPrintWriter := pprint.NewWriter(buffered, args.EnableRegions)
	prettyPrintWriter.Depth = args.Depth
	prettyPrintWriter.ParentIndentCount = args.ParentIndentCount

	v.PrettyPrint(prettyPrintWriter, args.Config)
	err = buffered.Flush()
	if err != nil {
		return nil, err
	}

	if args.EnableRegions {
		return prettyPrintWriter.Regions(), nil
	}
	return nil, nil
}
