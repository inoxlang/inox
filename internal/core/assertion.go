package core

import (
	"bufio"
	"bytes"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

const ASSERTION_BUFF_WRITER_SIZE = 100

type AssertionError struct {
	msg  string
	data *AssertionData
}

func (err *AssertionError) ShallowCopy() *AssertionError {
	return &AssertionError{
		msg:  err.msg,
		data: err.data,
	}
}

func (err AssertionError) Error() string {
	if err.data == nil {
		return err.msg
	}

	buf := bytes.NewBufferString(err.msg)
	w := bufio.NewWriterSize(buf, ASSERTION_BUFF_WRITER_SIZE)

	err.writeExplanation(w, &PrettyPrintConfig{
		PrettyPrintConfig: pprint.PrettyPrintConfig{
			MaxDepth: 10,
			Colorize: false,
			Compact:  false,
			Indent:   []byte{' ', ' '},
		},
	})

	w.Flush()
	return buf.String()
}

func (err AssertionError) writeExplanation(w *bufio.Writer, config *PrettyPrintConfig) {
	expr := err.data.assertionStatement.Expr

	switch node := expr.(type) {
	case *parse.BinaryExpression:
		leftVal := err.data.intermediaryValues[node.Left]
		rightVal := err.data.intermediaryValues[node.Right]

		if leftVal == nil || rightVal == nil {
			return
		}

		switch node.Operator {
		case parse.Equal:
			w.Write([]byte(": expected "))
			leftVal.PrettyPrint(w, config, 0, 0)
			w.Write([]byte(" to be equal to "))
			rightVal.PrettyPrint(w, config, 0, 0)
		}
	}
}

func (err AssertionError) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig) {
	w.Write(utils.StringAsBytes(err.msg))
	err.writeExplanation(w, config)
}

func (err AssertionError) PrettySPrint(config *PrettyPrintConfig) string {
	buf := bytes.NewBuffer(nil)
	w := bufio.NewWriterSize(buf, ASSERTION_BUFF_WRITER_SIZE)

	err.PrettyPrint(w, config)
	w.Flush()
	return buf.String()
}

type AssertionData struct {
	assertionStatement *parse.AssertionStatement
	intermediaryValues map[parse.Node]Value
}
