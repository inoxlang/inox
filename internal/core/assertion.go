package internal

import (
	"bytes"
	"io"

	parse "github.com/inox-project/inox/internal/parse"
)

type AssertionError struct {
	msg  string
	data *AssertionData
}

func (err AssertionError) Error() string {
	if err.data == nil {
		return err.msg
	}

	buf := bytes.NewBufferString(err.msg)
	err.writeExplanation(buf, &PrettyPrintConfig{
		MaxDepth: 10,
		Colorize: false,
		Colors:   &DEFAULT_LIGHTMODE_PRINT_COLORS,
		Compact:  false,
		Indent:   []byte{' ', ' '},
	})
	return buf.String()
}

func (err AssertionError) writeExplanation(w io.Writer, config *PrettyPrintConfig) {
	expr := err.data.assertionStatement.Expr

	switch node := expr.(type) {
	case *parse.BinaryExpression:
		leftVal := err.data.intermediaryValues[node.Left]
		rightVal := err.data.intermediaryValues[node.Right]

		switch node.Operator {
		case parse.Equal:
			w.Write([]byte(": expected "))
			leftVal.PrettyPrint(w, config, 0, 0)
			w.Write([]byte(" to be equal to "))
			rightVal.PrettyPrint(w, config, 0, 0)
		}
	}
}

func (err AssertionError) PrettyPrint(w io.Writer, config *PrettyPrintConfig) {
	w.Write([]byte(err.msg))
	err.writeExplanation(w, config)
}

func (err AssertionError) PrettySPrint(config *PrettyPrintConfig) string {
	buf := bytes.NewBuffer(nil)
	err.PrettyPrint(buf, config)
	return buf.String()
}

type AssertionData struct {
	assertionStatement *parse.AssertionStatement
	intermediaryValues map[parse.Node]Value
}
