package core

import (
	"bufio"
	"bytes"
	"errors"
	"strings"

	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

const ASSERTION_BUFF_WRITER_SIZE = 100

// An AssertionError is raised when an assertion statement fails (false condition).
type AssertionError struct {
	msg  string
	data *AssertionData

	isTestAssertion bool
	testModule      *Module //set if isTestAssertion
}

func (err *AssertionError) ShallowCopy() *AssertionError {
	return &AssertionError{
		msg:             err.msg,
		data:            err.data,
		isTestAssertion: err.isTestAssertion,
		testModule:      err.testModule,
	}
}

func (err AssertionError) IsTestAssertion() bool {
	return err.isTestAssertion
}

func (err AssertionError) Error() string {
	if err.data == nil || !err.isTestAssertion {
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

// writeExplanation attempts to determine an explanation about why the assertion failed,
// if an explanation is found it is written to w.
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
		case
			parse.Equal, parse.NotEqual, parse.Is, parse.IsNot,
			parse.LessThan, parse.LessOrEqual, parse.GreaterThan, parse.GreaterOrEqual,
			parse.Match, parse.Keyof, parse.Substrof:
		default:
			return
		}

		lhs := err.stringifyNode(node.Left)
		if strings.TrimSpace(lhs) == "" {
			return
		}

		if !parse.NodeIsSimpleValueLiteral(node.Left) {
			lhs = lhs + " (" + StringifyWithConfig(leftVal, config) + ")"
		}

		w.WriteString(": expected ")
		w.WriteString(lhs)

		switch node.Operator {
		case parse.Equal:
			w.WriteString(" to be equal to ")
		case parse.NotEqual:
			w.WriteString(" to not be equal to ")
		case parse.Is:
			w.WriteString(" to be ")
		case parse.IsNot:
			w.WriteString(" to not be ")

		case parse.LessThan:
			w.WriteString(" to be < ")
		case parse.LessOrEqual:
			w.WriteString(" to be <= ")
		case parse.GreaterThan:
			w.WriteString(" to be > ")
		case parse.GreaterOrEqual:
			w.WriteString(" to be >= ")
		case parse.Match:
			w.WriteString(" to match the patern ")
		case parse.Keyof:
			w.WriteString(" to be a key of ")
		case parse.Substrof:
			w.WriteString(" to be a substring of ")
		}

		rightVal.PrettyPrint(w, config, 0, 0)
	}
}

func (err AssertionError) stringifyNode(node parse.Node) string {
	if !err.isTestAssertion {
		panic(errors.New("node stringification is only supported by test assertion errors"))
	}

	switch n := node.(type) {
	case *parse.Variable:
		return "local variable `" + n.Name + "`"
	case *parse.GlobalVariable:
		return "global variable `" + n.Name + "`"
	case *parse.IdentifierLiteral:
		return "variable `" + n.Name + "`"
	case *parse.CallExpression:
		identCallee, ok := n.Callee.(*parse.IdentifierLiteral)
		if !ok {
			return ""
		}
		switch identCallee.Name {
		case globalnames.LEN_FN:
			if len(n.Arguments) != 1 {
				return ""
			}
			actual := err.stringifyNode(n.Arguments[0])
			if actual == "" {
				return ""
			}
			return "the length of " + actual
		}
	}
	return parse.SPrint(node, err.testModule.MainChunk.Node, parse.PrintConfig{})
}

func (err AssertionError) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig) {
	w.Write(utils.StringAsBytes(err.msg))
	if err.isTestAssertion {
		err.writeExplanation(w, config)
	}
}

func (err AssertionError) PrettySPrint(config *PrettyPrintConfig) string {
	buf := bytes.NewBuffer(nil)
	w := bufio.NewWriterSize(buf, ASSERTION_BUFF_WRITER_SIZE)

	err.PrettyPrint(w, config)
	w.Flush()
	return buf.String()
}

// AssertionData is the data recorded about an assertion.
type AssertionData struct {
	assertionStatement *parse.AssertionStatement
	intermediaryValues map[parse.Node]Value
}
